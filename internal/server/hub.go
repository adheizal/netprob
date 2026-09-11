package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"netprob/internal/auth"
	"netprob/internal/models"
	"netprob/internal/protocol"
	"netprob/internal/storage"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// Hub manages agent WebSocket connections.
type Hub struct {
	agents map[string]*AgentConn
	mu     sync.RWMutex
	store  *storage.Store
}

// AgentConn wraps a WebSocket connection for an authenticated agent.
type AgentConn struct {
	AgentID string
	Conn    *websocket.Conn
	Send    chan []byte
	Stop    chan struct{}
}

var errAgentSendQueueFull = errors.New("agent send queue full")

func NewHub(store *storage.Store) *Hub {
	return &Hub{
		agents: make(map[string]*AgentConn),
		store:  store,
	}
}

// RegisterAgent validates a reusable enrollment token and resolves one agent instance.
func (h *Hub) RegisterAgent(token, instanceID string) (*models.Agent, error) {
	tokenHash := auth.HashToken(token)
	agent, err := h.store.DB.ResolveAgentByTokenAndInstance(tokenHash, instanceID)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// SetAgentConnection registers an agent's connection.
func (h *Hub) SetAgentConnection(agent *models.Agent, conn *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close any existing connection for this agent
	if existing, ok := h.agents[agent.ID]; ok {
		select {
		case existing.Stop <- struct{}{}:
		default:
		}
		delete(h.agents, agent.ID)
	}
	h.agents[agent.ID] = conn

	now := time.Now()
	if err := h.store.DB.UpdateAgentStatus(agent.ID, true, now); err != nil {
		log.Warn().Err(err).Str("agent_id", agent.ID).Msg("failed to update agent status")
	}
}

// RemoveAgentConnection removes an agent's connection.
func (h *Hub) RemoveAgentConnection(agentID string, conn *AgentConn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	current, ok := h.agents[agentID]
	if !ok || current != conn {
		return
	}
	delete(h.agents, agentID)

	// Mark as offline
	now := time.Now()
	if err := h.store.DB.UpdateAgentStatus(agentID, false, now); err != nil {
		log.Warn().Err(err).Str("agent_id", agentID).Msg("failed to update agent status")
	}
}

// DispatchJob sends a job to a connected agent.
func (h *Hub) DispatchJob(agentID string, job *models.Job) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	conn, ok := h.agents[agentID]
	if !ok {
		return errors.New("agent not connected")
	}

	msg := protocol.Envelope{
		Type:    protocol.TypeJob,
		Payload: job,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		return errAgentSendQueueFull
	}
}

// IsAgentOnline checks if an agent is currently connected.
func (h *Hub) IsAgentOnline(agentID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[agentID]
	return ok
}

// AgentIDs returns all known agent IDs.
func (h *Hub) AgentIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.agents))
	for id := range h.agents {
		ids = append(ids, id)
	}
	return ids
}

// HandleAgentConnection processes a new WebSocket connection from an agent.
func (h *Hub) HandleAgentConnection(ctx context.Context, conn *websocket.Conn, agentHello *models.AgentHello) error {
	instanceID := strings.TrimSpace(agentHello.AgentID)
	if instanceID == "" {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "agent_id is required"}})
		return errors.New("agent_id is required")
	}
	if len(instanceID) > 255 {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "agent_id is too long"}})
		return errors.New("agent_id is too long")
	}
	agent, err := h.RegisterAgent(agentHello.Token, instanceID)
	if err != nil {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "invalid token"}})
		return err
	}

	hostname, version, primaryAddress, addresses, capabilities, location := mergeAgentMetadata(agent, agentHello)
	if err := h.store.DB.UpdateAgentMetadata(agent.ID, hostname, version, primaryAddress, addresses, capabilities, location); err != nil {
		log.Warn().Err(err).Str("agent_id", agent.ID).Msg("failed to update agent metadata")
	} else {
		agent.Hostname = hostname
		agent.Version = version
		agent.PrimaryAddress = primaryAddress
		agent.Addresses = addresses
		agent.Capabilities = capabilities
		agent.Location = location
	}

	ac := &AgentConn{
		AgentID: agent.ID,
		Conn:    conn,
		Send:    make(chan []byte, 100),
		Stop:    make(chan struct{}, 1),
	}

	h.SetAgentConnection(agent, ac)

	// Respond with hello_ok
	helloOK := protocol.Envelope{
		Type: protocol.TypeHelloOK,
		Payload: map[string]any{
			"agent_id": agent.ID,
			"online":   "true",
		},
	}
	conn.WriteJSON(helloOK)

	// Handle incoming messages (results, pings)
	go h.writePump(ac)
	defer h.RemoveAgentConnection(agent.ID, ac)

	for {
		select {
		case <-ac.Stop:
			return nil
		case <-ctx.Done():
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Debug().Err(err).Str("agent_id", agent.ID).Msg("agent connection closed")
				return nil
			}
			h.handleMessage(agent.ID, message)
		}
	}
}

func mergeAgentMetadata(agent *models.Agent, hello *models.AgentHello) (string, string, string, []string, []string, models.AgentLocation) {
	hostname := strings.TrimSpace(hello.Hostname)
	if hostname == "" {
		hostname = agent.Hostname
	}
	version := strings.TrimSpace(hello.Version)
	if version == "" {
		version = agent.Version
	}

	addresses := make([]string, 0, len(hello.Addresses))
	seenAddresses := make(map[string]struct{}, len(hello.Addresses))
	for _, value := range hello.Addresses {
		ip := net.ParseIP(strings.TrimSpace(value))
		if ip == nil || !ip.IsGlobalUnicast() {
			continue
		}
		address := ip.String()
		if _, exists := seenAddresses[address]; exists {
			continue
		}
		seenAddresses[address] = struct{}{}
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		addresses = agent.Addresses
	}
	primaryAddress := agent.PrimaryAddress
	if ip := net.ParseIP(strings.TrimSpace(hello.PrimaryAddress)); ip != nil && ip.IsGlobalUnicast() {
		primaryAddress = ip.String()
	}
	if primaryAddress == "" && len(addresses) > 0 {
		primaryAddress = addresses[0]
	}

	capabilities := make([]string, 0, len(hello.Capabilities))
	seenCapabilities := make(map[string]struct{}, len(hello.Capabilities))
	for _, value := range hello.Capabilities {
		capability := strings.TrimSpace(value)
		if capability == "" {
			continue
		}
		if _, exists := seenCapabilities[capability]; exists {
			continue
		}
		seenCapabilities[capability] = struct{}{}
		capabilities = append(capabilities, capability)
	}
	if len(capabilities) == 0 {
		capabilities = agent.Capabilities
	}

	location := agent.Location
	if publicIP := net.ParseIP(strings.TrimSpace(hello.Location.PublicIP)); publicIP != nil && publicIP.IsGlobalUnicast() {
		location.PublicIP = publicIP.String()
	}
	if value := strings.TrimSpace(hello.Location.CountryCode); value != "" {
		location.CountryCode = value
	}
	if value := strings.TrimSpace(hello.Location.Region); value != "" {
		location.Region = value
	}
	if value := strings.TrimSpace(hello.Location.City); value != "" {
		location.City = value
	}
	if value := strings.TrimSpace(hello.Location.Timezone); value != "" {
		location.Timezone = value
	}
	if value := strings.TrimSpace(hello.Location.ASName); value != "" {
		location.ASName = value
	}
	if value := strings.TrimSpace(hello.Location.ISP); value != "" {
		location.ISP = value
	}

	return hostname, version, primaryAddress, addresses, capabilities, location
}

func (h *Hub) handleMessage(agentID string, data []byte) {
	var env protocol.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		log.Warn().Err(err).Msg("failed to parse agent message")
		return
	}

	switch env.Type {
	case protocol.TypeResult:
		h.handleResult(agentID, env.Payload)
	case protocol.TypePing:
		h.sendEnvelope(agentID, protocol.Envelope{Type: protocol.TypePong, Payload: map[string]any{}})
	case protocol.TypePong:
		// just a keepalive
	default:
		log.Debug().Str("type", env.Type).Msg("unknown message type from agent")
	}
}

func (h *Hub) handleResult(agentID string, payload any) {
	// Payload is a map with job_id, status, error, ping_result, mtr_run keys
	payloadMap, ok := payload.(map[string]any)
	if !ok {
		log.Warn().Msg("unexpected result payload type")
		return
	}

	result := models.JobResult{
		JobID:  getString(payloadMap, "job_id"),
		Status: getString(payloadMap, "status"),
	}
	if errVal, ok := payloadMap["error"]; ok && errVal != nil {
		if s, ok := errVal.(string); ok {
			result.Error = s
		}
	}

	// Parse ping_result
	if pr, ok := payloadMap["ping_result"]; ok && pr != nil {
		prBytes, err := json.Marshal(pr)
		if err != nil {
			log.Warn().Err(err).Msg("failed to marshal ping_result")
		}
		var pingResult models.PingResult
		if err := json.Unmarshal(prBytes, &pingResult); err == nil {
			result.PingResult = &pingResult
		}
	}

	// Parse mtr_run
	if mr, ok := payloadMap["mtr_run"]; ok && mr != nil {
		mrBytes, err := json.Marshal(mr)
		if err != nil {
			log.Warn().Err(err).Msg("failed to marshal mtr_run")
		}
		var mtrRun models.MTRRun
		if err := json.Unmarshal(mrBytes, &mtrRun); err == nil {
			result.MTRRun = &mtrRun
		}
	}

	if result.PingResult != nil {
		if err := h.store.SavePingResult(result.PingResult); err != nil {
			log.Warn().Err(err).Msg("failed to save ping result")
		}
	} else if getString(payloadMap, "probe_type") == models.ProbePing {
		completedAt := time.Now().UTC()
		if value, ok := payloadMap["completed_at"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				completedAt = parsed
			}
		}
		if err := h.store.WriteProbeStatus(
			models.ProbePing,
			result.Status == "success",
			completedAt,
			getString(payloadMap, "direction_id"),
			getString(payloadMap, "source_agent_id"),
			getString(payloadMap, "destination_agent_id"),
		); err != nil {
			log.Warn().Err(err).Msg("failed to save ping probe status")
		}
	}

	if result.MTRRun != nil {
		if err := h.store.SaveMTRRun(result.MTRRun); err != nil {
			log.Warn().Err(err).Msg("failed to save mtr result")
		}
	}

	log.Info().
		Str("agent_id", agentID).
		Str("job_id", result.JobID).
		Str("status", result.Status).
		Msg("job result received")
}

func (h *Hub) sendEnvelope(agentID string, envelope protocol.Envelope) {
	data, err := json.Marshal(envelope)
	if err != nil {
		log.Warn().Err(err).Str("agent_id", agentID).Msg("failed to marshal agent message")
		return
	}

	h.mu.RLock()
	conn, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok {
		return
	}

	select {
	case conn.Send <- data:
	default:
		log.Warn().Str("agent_id", agentID).Msg("agent send queue full")
	}
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (h *Hub) writePump(conn *AgentConn) {
	defer func() {
		conn.Conn.Close()
	}()

	for {
		select {
		case msg := <-conn.Send:
			conn.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-conn.Stop:
			return
		}
	}
}
