package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"netprob/internal/auth"
	"netprob/internal/models"

	"github.com/go-chi/chi/v5"
)

// --- Health ---

func (s *APIServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// --- Agent Registration ---

// HandleRegisterAgent creates a reusable enrollment token and its pending agent row.
// The token is plaintext (sent over HTTPS in production). It's stored as a SHA-256 hash.
func (s *APIServer) HandleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Hostname == "" {
		req.Hostname = "pending"
	}
	if len(req.Addresses) == 0 {
		req.Addresses = []string{}
	}
	if len(req.Capabilities) == 0 {
		req.Capabilities = []string{}
	}
	if req.Version == "" {
		req.Version = "pending"
	}

	token, err := auth.GenerateToken()
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	agent := models.NewAgent(req.Hostname, req.Version, req.Addresses, req.Capabilities)
	agent.TokenHash = auth.HashToken(token)

	if err := s.store.DB.CreateAgent(agent); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registerAgentResponse{ID: agent.ID, Token: token})
}

// --- List Agents ---

func (s *APIServer) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := s.store.DB.ListAgents()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(agents)
}

// --- Get Agent ---

func (s *APIServer) HandleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := s.store.DB.GetAgentByID(id)
	if err != nil {
		http.Error(w, "agent not found", http.StatusNotFound)
		return
	}
	agent.TokenHash = ""
	json.NewEncoder(w).Encode(agent)
}

// --- Delete Agent ---

func (s *APIServer) HandleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DB.DeleteAgent(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Links ---

func (s *APIServer) HandleCreateLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	link := models.NewLink(req.Name, req.Description)
	if err := s.store.DB.CreateLink(link); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(link)
}

func (s *APIServer) HandleListLinks(w http.ResponseWriter, r *http.Request) {
	links, err := s.store.DB.ListLinks()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	result := make([]linkWithAgents, len(links))
	for i, l := range links {
		result[i] = linkWithAgents{Link: l, Directions: make([]directionSummary, 0)}
		dirs, err := s.store.DB.ListDirectionsByLink(l.ID)
		if err != nil {
			continue
		}
		for _, d := range dirs {
			srcAgent, _ := s.store.DB.GetAgentByID(d.SourceAgentID)
			destAgent, _ := s.store.DB.GetAgentByID(d.DestinationAgentID)
			latestPing, _ := s.store.DB.GetLatestPingResult(d.ID)

			srcSummary := &agentSummary{ID: d.SourceAgentID, Online: false}
			if srcAgent != nil {
				srcSummary.Hostname = srcAgent.Hostname
				srcSummary.Version = srcAgent.Version
				srcSummary.Addresses = srcAgent.Addresses
				srcSummary.PrimaryAddress = srcAgent.PrimaryAddress
				srcSummary.Online = srcAgent.Online
				srcSummary.LastSeen = srcAgent.LastSeen
			}

			destSummary := &agentSummary{ID: d.DestinationAgentID, Online: false}
			if destAgent != nil {
				destSummary.Hostname = destAgent.Hostname
				destSummary.Version = destAgent.Version
				destSummary.Addresses = destAgent.Addresses
				destSummary.PrimaryAddress = destAgent.PrimaryAddress
				destSummary.Online = destAgent.Online
				destSummary.LastSeen = destAgent.LastSeen
			}

			result[i].Directions = append(result[i].Directions, directionSummary{
				ID:                 d.ID,
				SourceAgentID:      d.SourceAgentID,
				DestinationAgentID: d.DestinationAgentID,
				SourceAgent:        srcSummary,
				DestAgent:          destSummary,
				TargetAddress:      d.TargetAddress,
				PingInterval:       d.PingInterval,
				MTRInterval:        d.MTRInterval,
				PingEnabled:        d.PingEnabled,
				MTREnabled:         d.MTREnabled,
				Online:             s.hub.IsAgentOnline(d.SourceAgentID),
				LatestPing:         latestPing,
			})
		}
	}
	json.NewEncoder(w).Encode(result)
}

func (s *APIServer) HandleGetLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	link, err := s.store.DB.GetLinkByID(id)
	if err != nil {
		http.Error(w, "link not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(link)
}

func (s *APIServer) HandleUpdateLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := s.store.DB.UpdateLink(id, req.Name, req.Description); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) HandleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DB.DeleteLink(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Directions ---

func (s *APIServer) HandleCreateDirection(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "link_id")
	var req createDirectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.SourceAgentID == "" || req.DestinationAgentID == "" || req.TargetAddress == "" {
		http.Error(w, "source_agent_id, destination_agent_id, and target_address are required", http.StatusBadRequest)
		return
	}

	if _, err := s.store.DB.GetLinkByID(linkID); err != nil {
		http.Error(w, "link not found", http.StatusNotFound)
		return
	}

	if _, err := s.store.DB.GetAgentByID(req.SourceAgentID); err != nil {
		http.Error(w, "source agent not found", http.StatusNotFound)
		return
	}
	if _, err := s.store.DB.GetAgentByID(req.DestinationAgentID); err != nil {
		http.Error(w, "destination agent not found", http.StatusNotFound)
		return
	}

	dir := models.NewDirection(linkID, req.SourceAgentID, req.DestinationAgentID, req.TargetAddress)
	if req.PingInterval != nil {
		if *req.PingInterval <= 0 {
			http.Error(w, "ping_interval_seconds must be greater than zero", http.StatusBadRequest)
			return
		}
		dir.PingInterval = *req.PingInterval
	}
	if req.MTRInterval != nil {
		if *req.MTRInterval <= 0 {
			http.Error(w, "mtr_interval_seconds must be greater than zero", http.StatusBadRequest)
			return
		}
		dir.MTRInterval = *req.MTRInterval
	}
	if req.PingEnabled != nil {
		dir.PingEnabled = *req.PingEnabled
	}
	if req.MTREnabled != nil {
		dir.MTREnabled = *req.MTREnabled
	}
	dir.MTRThresholdLoss = req.MTRThresholdLoss
	dir.MTRThresholdRtt = req.MTRThresholdRtt

	if err := s.store.DB.CreateDirection(dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(dir)
}

func (s *APIServer) HandleUpdateDirection(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "link_id")
	directionID := chi.URLParam(r, "direction_id")

	var req updateDirectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dir, err := s.store.DB.GetDirectionByID(directionID)
	if err != nil || dir.LinkID != linkID {
		http.Error(w, "direction not found", http.StatusNotFound)
		return
	}
	if req.TargetAddress != nil {
		if *req.TargetAddress == "" {
			http.Error(w, "target_address cannot be empty", http.StatusBadRequest)
			return
		}
		dir.TargetAddress = *req.TargetAddress
	}
	if req.PingInterval != nil {
		if *req.PingInterval <= 0 {
			http.Error(w, "ping_interval_seconds must be greater than zero", http.StatusBadRequest)
			return
		}
		dir.PingInterval = *req.PingInterval
	}
	if req.MTRInterval != nil {
		if *req.MTRInterval <= 0 {
			http.Error(w, "mtr_interval_seconds must be greater than zero", http.StatusBadRequest)
			return
		}
		dir.MTRInterval = *req.MTRInterval
	}
	if req.PingEnabled != nil {
		dir.PingEnabled = *req.PingEnabled
	}
	if req.MTREnabled != nil {
		dir.MTREnabled = *req.MTREnabled
	}

	if err := s.store.DB.UpdateDirection(dir); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) HandleListDirections(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "link_id")
	dirs, err := s.store.DB.ListDirectionsByLink(linkID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(dirs)
}

// --- Ping Results ---

func (s *APIServer) HandleGetPingResults(w http.ResponseWriter, r *http.Request) {
	directionID := chi.URLParam(r, "direction_id")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	results, err := s.store.DB.QueryPingResults(directionID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(results)
}

// --- MTR Runs ---

func (s *APIServer) HandleGetMTRRuns(w http.ResponseWriter, r *http.Request) {
	directionID := chi.URLParam(r, "direction_id")
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	runs, err := s.store.DB.ListMTRRuns(directionID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(runs)
}

func (s *APIServer) HandleGetMTRRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	run, err := s.store.DB.GetMTRRunByID(id)
	if err != nil {
		http.Error(w, "mtr run not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(run)
}

// --- Retention Settings ---

func (s *APIServer) HandleGetRetentionSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.DB.GetRetentionSettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(settings)
}

func (s *APIServer) HandleUpdateRetentionSettings(w http.ResponseWriter, r *http.Request) {
	var req updateRetentionSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.PingRetentionDays == nil || req.MTRRetentionDays == nil {
		http.Error(w, "ping_retention_days and mtr_retention_days are required", http.StatusBadRequest)
		return
	}
	if *req.PingRetentionDays < 0 || *req.MTRRetentionDays < 0 {
		http.Error(w, "retention days must be zero or greater", http.StatusBadRequest)
		return
	}

	settings := &models.RetentionSettings{
		PingRetentionDays: *req.PingRetentionDays,
		MTRRetentionDays:  *req.MTRRetentionDays,
	}
	if err := s.store.DB.UpdateRetentionSettings(settings); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cleanup, err := s.store.DB.CleanupExpiredProbeHistory(settings, time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(struct {
		*models.RetentionSettings
		Cleanup *models.RetentionCleanupResult `json:"cleanup"`
	}{settings, cleanup})
}
