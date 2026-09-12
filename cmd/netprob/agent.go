package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"netprob/internal/buildinfo"
	"netprob/internal/config"
	"netprob/internal/models"
	"netprob/internal/probe/mtr"
	"netprob/internal/probe/ping"
	"netprob/internal/protocol"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type websocketWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (w *websocketWriter) WriteJSON(value any, timeout time.Duration) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer w.conn.SetWriteDeadline(time.Time{})
	return w.conn.WriteJSON(value)
}

func runAgent(cfg *config.Config) {
	token := cfg.Agent.Token
	if token == "" {
		log.Fatal().Msg("agent token required (set via --config or NETPROB_AGENT_TOKEN env)")
	}

	controllerURL := cfg.Agent.Controller
	if controllerURL == "" {
		log.Fatal().Msg("controller URL required (set in config or NETPROB_CONTROLLER env)")
	}

	agentName := cfg.Agent.Name
	if agentName == "" {
		hostname, _ := os.Hostname()
		agentName = hostname
	}
	agentID := cfg.Agent.ID
	if agentID == "" {
		agentID = agentName
	}
	version := cfg.Agent.Version
	if version == "" {
		version = buildinfo.Version
	}
	caps := cfg.Agent.Capabilities
	if len(caps) == 0 {
		caps = map[string]string{"ping": "", "mtr": ""}
	}

	log.Info().
		Str("controller", controllerURL).
		Str("name", agentName).
		Msg("starting NetProb agent")

	location, err := resolveAgentLocation(context.Background(), cfg.Agent.GeoIPURL, cfg.Agent.Region)
	if err != nil {
		log.Warn().Err(err).Msg("GeoIP lookup failed; continuing without automatic location")
	} else if location.Region != "" || location.PublicIP != "" {
		log.Info().
			Str("public_ip", location.PublicIP).
			Str("region", location.Region).
			Msg("agent location resolved")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Connect with reconnection
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := connectToController(ctx, controllerURL, agentID, agentName, version, caps, location, cfg.Agent.PrimaryAddress, token, cfg.Agent.MaxConcurrentJobs, time.Duration(cfg.Agent.ProbeTimeout)*time.Second); err != nil {
			log.Error().Err(err).Msg("connection failed, retrying in 10s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
			continue
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func connectToController(ctx context.Context, controllerURL, agentID, agentName, version string, caps map[string]string, location models.AgentLocation, primaryAddressOverride, token string, maxConcurrentJobs int, probeTimeout time.Duration) error {
	// Build WebSocket URL
	u, err := url.Parse(controllerURL)
	if err != nil {
		return fmt.Errorf("parse controller URL: %w", err)
	}
	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	u.Scheme = wsScheme
	u.Path = "/ws"

	header := http.Header{}
	header.Set("Origin", "netprob-agent://"+agentName)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return fmt.Errorf("dial WebSocket: %w", err)
	}
	defer conn.Close()
	connCtx, cancelConnection := context.WithCancel(ctx)
	writer := &websocketWriter{conn: conn}
	jobQueue := make(chan any, 100)
	var workerWG sync.WaitGroup
	for i := 0; i < maxConcurrentJobs; i++ {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			for {
				select {
				case <-connCtx.Done():
					return
				default:
				}
				select {
				case <-connCtx.Done():
					return
				case payload := <-jobQueue:
					handleJob(connCtx, writer, payload, probeTimeout)
				}
			}
		}()
	}
	defer func() {
		cancelConnection()
		_ = conn.Close()
		workerWG.Wait()
	}()
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		<-connCtx.Done()
		_ = conn.Close()
	}()

	log.Info().Str("url", u.String()).Msg("WebSocket connected")

	// Send hello
	capList := []string{"ping", "mtr"}
	if len(caps) > 0 {
		capList = make([]string, 0, len(caps))
		for k := range caps {
			capList = append(capList, k)
		}
		sort.Strings(capList)
	}
	addresses, err := detectAgentAddresses()
	if err != nil {
		log.Warn().Err(err).Msg("failed to detect agent addresses")
	}
	primaryAddress := choosePrimaryAddress(primaryAddressOverride, conn.LocalAddr(), addresses, location.PublicIP)

	hello := models.AgentHello{
		AgentID:        agentID,
		Hostname:       agentName,
		Version:        version,
		Addresses:      addresses,
		PrimaryAddress: primaryAddress,
		Capabilities:   capList,
		Location:       location,
		Token:          token,
	}

	helloMsg := protocol.Envelope{
		Type:    protocol.TypeHello,
		Payload: hello,
	}

	if err := writer.WriteJSON(helloMsg, 5*time.Second); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}

	// Start keepalive goroutine
	workerWG.Add(1)
	go func() {
		defer workerWG.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-connCtx.Done():
				return
			case <-ticker.C:
				if err := writer.WriteJSON(protocol.Envelope{Type: protocol.TypePing, Payload: map[string]any{}}, 5*time.Second); err != nil {
					log.Debug().Err(err).Msg("failed to send keepalive")
					return
				}
			}
		}
	}()

	// Read messages
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read message: %w", err)
		}

		var env protocol.Envelope
		if err := json.Unmarshal(msgData, &env); err != nil {
			log.Warn().Err(err).Msg("failed to parse message from controller")
			continue
		}

		switch env.Type {
		case protocol.TypeJob:
			select {
			case jobQueue <- env.Payload:
			default:
				rejectJob(writer, env.Payload, "agent job queue is full")
			}
		case protocol.TypePing:
			if err := writer.WriteJSON(protocol.Envelope{Type: protocol.TypePong, Payload: map[string]any{}}, 5*time.Second); err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
		case protocol.TypeError:
			log.Error().Msg("controller sent error")
		}
	}
}

func decodeJob(payload any) (*models.Job, error) {
	jobBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var job models.Job
	if err := json.Unmarshal(jobBytes, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func rejectJob(writer *websocketWriter, payload any, reason string) {
	job, err := decodeJob(payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse rejected job")
		return
	}
	writeJobResult(writer, job, &models.JobResult{JobID: job.ID, Status: "error", Error: reason})
}

func handleJob(ctx context.Context, writer *websocketWriter, payload any, probeTimeout time.Duration) {
	job, err := decodeJob(payload)
	if err != nil {
		log.Warn().Err(err).Msg("failed to parse job")
		return
	}

	log.Info().
		Str("job_id", job.ID).
		Str("type", job.Type).
		Str("dest", job.Direction.DestinationAgentID).
		Str("target", job.Direction.TargetAddress).
		Msg("executing job")

	result := &models.JobResult{
		JobID:  job.ID,
		Status: "success",
	}

	switch job.Type {
	case models.ProbePing:
		count := 5
		if c, ok := job.Config["count"].(float64); ok {
			count = int(c)
		}
		interval := 1
		if i, ok := job.Config["interval"].(float64); ok {
			interval = int(i)
		}

		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		pingResult, err := ping.ExecuteContext(probeCtx, job.Direction.TargetAddress, count, interval)
		cancel()
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		} else {
			pingResult.SourceAgentID = job.Direction.SourceAgentID
			pingResult.DestinationAgentID = job.Direction.DestinationAgentID
			pingResult.DirectionID = job.Direction.DirectionID
			result.PingResult = pingResult
		}

	case models.ProbeMTR:
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		mtrResult, err := mtr.ExecuteContext(
			probeCtx,
			job.Direction.TargetAddress,
			job.Direction.SourceAgentID,
			job.Direction.DestinationAgentID,
			job.Direction.DirectionID,
		)
		cancel()
		result.MTRRun = mtrResult
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
		}

	default:
		result.Status = "error"
		result.Error = "unknown probe type: " + job.Type
	}

	writeJobResult(writer, job, result)
}

func writeJobResult(writer *websocketWriter, job *models.Job, result *models.JobResult) {
	// Send result back
	resultMsg := protocol.Envelope{
		Type: protocol.TypeResult,
		Payload: map[string]any{
			"job_id":               result.JobID,
			"status":               result.Status,
			"error":                result.Error,
			"probe_type":           job.Type,
			"direction_id":         job.Direction.DirectionID,
			"source_agent_id":      job.Direction.SourceAgentID,
			"destination_agent_id": job.Direction.DestinationAgentID,
			"completed_at":         time.Now().UTC(),
			"ping_result":          result.PingResult,
			"mtr_run":              result.MTRRun,
		},
	}

	if err := writer.WriteJSON(resultMsg, 10*time.Second); err != nil {
		log.Warn().Err(err).Str("job_id", job.ID).Msg("failed to send result")
	}
}
