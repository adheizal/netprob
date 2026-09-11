package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"netprob/internal/models"
	"netprob/internal/protocol"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// HandleWebSocket handles agent WebSocket connections.
// Agents connect outbound to this endpoint. The connection is authenticated
// via a token in the initial hello message.
func (s *APIServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Warn().Err(err).Msg("websocket upgrade failed")
		return
	}

	// Set read deadline for the hello message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read the hello message
	_, msgData, err := conn.ReadMessage()
	if err != nil {
		log.Warn().Err(err).Msg("failed to read hello from agent")
		conn.Close()
		return
	}

	var env protocol.Envelope
	if err := json.Unmarshal(msgData, &env); err != nil {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "invalid JSON"}})
		conn.Close()
		return
	}

	if env.Type != protocol.TypeHello {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "expected hello"}})
		conn.Close()
		return
	}

	helloBytes, _ := json.Marshal(env.Payload)
	var hello models.AgentHello
	if err := json.Unmarshal(helloBytes, &hello); err != nil {
		conn.WriteJSON(protocol.Envelope{Type: protocol.TypeError, Payload: map[string]any{"error": "invalid hello"}})
		conn.Close()
		return
	}

	// Reset read deadline after hello
	conn.SetReadDeadline(time.Time{})

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	log.Info().
		Str("hostname", hello.Hostname).
		Msg("agent connecting")

	if err := s.hub.HandleAgentConnection(ctx, conn, &hello); err != nil {
		log.Warn().Err(err).Str("agent_id", hello.AgentID).Msg("agent auth failed")
		conn.Close()
	}
}
