package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"netprob/internal/auth"
	"netprob/internal/models"
	"netprob/internal/storage"

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
		writeInternalError(w, err, "failed to register agent")
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(registerAgentResponse{ID: agent.ID, Token: token})
}

// --- List Agents ---

func (s *APIServer) HandleListAgents(w http.ResponseWriter, r *http.Request) {
	pageValue := r.URL.Query().Get("page")
	pageSizeValue := r.URL.Query().Get("page_size")
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	if status != "" && status != "online" && status != "offline" {
		http.Error(w, "status must be online or offline", http.StatusBadRequest)
		return
	}
	if pageValue != "" || pageSizeValue != "" || search != "" || status != "" {
		page, pageSize := 1, 20
		var err error
		if pageValue != "" {
			page, err = strconv.Atoi(pageValue)
			if err != nil || page < 1 {
				http.Error(w, "page must be a positive integer", http.StatusBadRequest)
				return
			}
		}
		if pageSizeValue != "" {
			pageSize, err = strconv.Atoi(pageSizeValue)
			if err != nil || pageSize < 1 || pageSize > 100 {
				http.Error(w, "page_size must be between 1 and 100", http.StatusBadRequest)
				return
			}
		}

		total, err := s.store.DB.CountAgentsFiltered(search, status)
		if err != nil {
			writeInternalError(w, err, "failed to count agents")
			return
		}
		totalPages := (total + pageSize - 1) / pageSize
		agents := make([]*models.Agent, 0)
		if page <= totalPages {
			agents, err = s.store.DB.ListAgentsPageFiltered(pageSize, (page-1)*pageSize, search, status)
			if err != nil {
				writeInternalError(w, err, "failed to list agents")
				return
			}
		}
		json.NewEncoder(w).Encode(agentPageResponse{
			Agents:     agents,
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		})
		return
	}

	agents, err := s.store.DB.ListAgents()
	if err != nil {
		writeInternalError(w, err, "failed to list agents")
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
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "agent not found", http.StatusNotFound)
			return
		}
		writeInternalError(w, err, "failed to delete agent")
		return
	}
	s.hub.DisconnectAgent(id)
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
		writeInternalError(w, err, "failed to create link")
		return
	}
	json.NewEncoder(w).Encode(link)
}

func (s *APIServer) HandleListLinks(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("page") != "" || query.Get("page_size") != "" || query.Get("search") != "" ||
		query.Get("status") != "" || query.Get("agent_id") != "" || query.Get("source_agent_id") != "" ||
		query.Get("destination_agent_id") != "" || query.Get("sort") != "" || query.Get("order") != "" {
		s.handleListLinksPage(w, r)
		return
	}

	links, err := s.store.DB.ListLinks()
	if err != nil {
		writeInternalError(w, err, "failed to list links")
		return
	}
	items := make([]storage.LinkListItem, len(links))
	for i, link := range links {
		items[i] = storage.LinkListItem{Link: link}
	}
	result, err := s.hydrateLinkItems(items)
	if err != nil {
		writeInternalError(w, err, "failed to load link summaries")
		return
	}
	json.NewEncoder(w).Encode(result)
}

func (s *APIServer) handleListLinksPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	page, pageSize := 1, 20
	var err error
	if value := query.Get("page"); value != "" {
		page, err = strconv.Atoi(value)
		if err != nil || page < 1 {
			http.Error(w, "page must be a positive integer", http.StatusBadRequest)
			return
		}
	}
	if value := query.Get("page_size"); value != "" {
		pageSize, err = strconv.Atoi(value)
		if err != nil || pageSize < 1 || pageSize > 100 {
			http.Error(w, "page_size must be between 1 and 100", http.StatusBadRequest)
			return
		}
	}
	status := query.Get("status")
	if status != "" && status != "healthy" && status != "degraded" && status != "down" && status != "inactive" {
		http.Error(w, "status must be healthy, degraded, down, or inactive", http.StatusBadRequest)
		return
	}
	sortBy := query.Get("sort")
	if sortBy != "" && sortBy != "status" && sortBy != "name" && sortBy != "latency" && sortBy != "loss" && sortBy != "updated" {
		http.Error(w, "sort must be status, name, latency, loss, or updated", http.StatusBadRequest)
		return
	}
	order := query.Get("order")
	if order != "" && order != "asc" && order != "desc" {
		http.Error(w, "order must be asc or desc", http.StatusBadRequest)
		return
	}

	items, total, err := s.store.DB.ListLinksPage(storage.LinkListOptions{
		Limit: pageSize, Offset: (page - 1) * pageSize, Search: query.Get("search"), Status: status,
		AgentID: query.Get("agent_id"), SourceAgentID: query.Get("source_agent_id"),
		DestinationAgentID: query.Get("destination_agent_id"), Sort: sortBy, Order: order,
	})
	if err != nil {
		writeInternalError(w, err, "failed to list links")
		return
	}
	links, err := s.hydrateLinkItems(items)
	if err != nil {
		writeInternalError(w, err, "failed to load link summaries")
		return
	}
	totalPages := (total + pageSize - 1) / pageSize
	json.NewEncoder(w).Encode(linkPageResponse{Links: links, Page: page, PageSize: pageSize, Total: total, TotalPages: totalPages})
}

func (s *APIServer) hydrateLinkItems(items []storage.LinkListItem) ([]linkWithAgents, error) {
	linkIDs := make([]string, len(items))
	for i, item := range items {
		linkIDs[i] = item.Link.ID
	}
	var directions []*models.Direction
	var err error
	if len(linkIDs) > 500 {
		// The legacy unpaginated endpoint can exceed SQLite's bind-variable
		// limit. Keep its old full-inventory behavior while paginated callers
		// remain scoped to the current page.
		directions, err = s.store.DB.ListDirections()
	} else {
		directions, err = s.store.DB.ListDirectionsByLinkIDs(linkIDs)
	}
	if err != nil {
		return nil, err
	}
	agentIDSet := make(map[string]struct{})
	directionIDs := make([]string, 0, len(directions))
	directionsByLink := make(map[string][]*models.Direction)
	for _, direction := range directions {
		directionsByLink[direction.LinkID] = append(directionsByLink[direction.LinkID], direction)
		agentIDSet[direction.SourceAgentID] = struct{}{}
		agentIDSet[direction.DestinationAgentID] = struct{}{}
		directionIDs = append(directionIDs, direction.ID)
	}
	agentIDs := make([]string, 0, len(agentIDSet))
	for id := range agentIDSet {
		agentIDs = append(agentIDs, id)
	}
	var agents []*models.Agent
	if len(agentIDs) > 500 {
		agents, err = s.store.DB.ListAgents()
	} else {
		agents, err = s.store.DB.ListAgentsByIDs(agentIDs)
	}
	if err != nil {
		return nil, err
	}
	var latestPings map[string]*models.PingResult
	if len(directionIDs) > 500 {
		latestPings, err = s.store.DB.ListLatestPingResults()
	} else {
		latestPings, err = s.store.DB.ListLatestPingResultsByDirectionIDs(directionIDs)
	}
	if err != nil {
		return nil, err
	}
	agentsByID := make(map[string]*models.Agent, len(agents))
	for _, agent := range agents {
		agentsByID[agent.ID] = agent
	}

	result := make([]linkWithAgents, len(items))
	for i, item := range items {
		result[i] = linkWithAgents{
			Link: item.Link, Directions: make([]directionSummary, 0), Status: item.Status,
			MaxLoss: item.MaxLoss, MaxLatency: item.MaxLatency,
		}
		for _, d := range directionsByLink[item.Link.ID] {
			srcAgent := agentsByID[d.SourceAgentID]
			destAgent := agentsByID[d.DestinationAgentID]
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
				LatestPing:         latestPings[d.ID],
			})
		}
	}
	return result, nil
}

func (s *APIServer) HandleOverview(w http.ResponseWriter, r *http.Request) {
	limit := 10
	if value := r.URL.Query().Get("problem_limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 50 {
			http.Error(w, "problem_limit must be between 1 and 50", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	agentCounts, err := s.store.DB.CountAgentStatuses()
	if err != nil {
		writeInternalError(w, err, "failed to count agents")
		return
	}
	linkCounts, err := s.store.DB.CountLinkStatuses()
	if err != nil {
		writeInternalError(w, err, "failed to count links")
		return
	}
	problemItems, _, err := s.store.DB.ListLinksPage(storage.LinkListOptions{Limit: limit, Status: "problem", Sort: "status"})
	if err != nil {
		writeInternalError(w, err, "failed to list problem links")
		return
	}
	problemLinks, err := s.hydrateLinkItems(problemItems)
	if err != nil {
		writeInternalError(w, err, "failed to load problem links")
		return
	}
	offlineAgents, err := s.store.DB.ListAgentsPageFiltered(limit, 0, "", "offline")
	if err != nil {
		writeInternalError(w, err, "failed to list offline agents")
		return
	}
	json.NewEncoder(w).Encode(overviewResponse{
		Agents: agentCounts, Links: linkCounts, ProblemLinks: problemLinks, OfflineAgents: offlineAgents,
	})
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
		writeInternalError(w, err, "failed to update link")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) HandleDeleteLink(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DB.DeleteLink(id); err != nil {
		writeInternalError(w, err, "failed to delete link")
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
		writeInternalError(w, err, "failed to create direction")
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
		writeInternalError(w, err, "failed to update direction")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) HandleListDirections(w http.ResponseWriter, r *http.Request) {
	linkID := chi.URLParam(r, "link_id")
	dirs, err := s.store.DB.ListDirectionsByLink(linkID)
	if err != nil {
		writeInternalError(w, err, "failed to list directions")
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
		writeInternalError(w, err, "failed to load ping history")
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
		writeInternalError(w, err, "failed to load MTR history")
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
		writeInternalError(w, err, "failed to load retention settings")
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
		writeInternalError(w, err, "failed to update retention settings")
		return
	}
	cleanup, err := s.store.DB.CleanupExpiredProbeHistory(settings, time.Now().UTC())
	if err != nil {
		writeInternalError(w, err, "failed to clean probe history")
		return
	}
	json.NewEncoder(w).Encode(struct {
		*models.RetentionSettings
		Cleanup *models.RetentionCleanupResult `json:"cleanup"`
	}{settings, cleanup})
}
