package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"netprob/internal/config"
	"netprob/internal/models"
	"netprob/internal/storage"
)

func TestListAgentsPagination(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	for _, hostname := range []string{"agent-a", "agent-b", "agent-c"} {
		if err := store.DB.CreateAgent(models.NewAgent(hostname, "test", []string{}, []string{})); err != nil {
			t.Fatal(err)
		}
	}

	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/agents?page=2&page_size=2", nil)
	response := httptest.NewRecorder()
	api.HandleListAgents(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d: %s", response.Code, response.Body.String())
	}

	var page agentPageResponse
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if page.Page != 2 || page.PageSize != 2 || page.Total != 3 || page.TotalPages != 2 || len(page.Agents) != 1 {
		t.Fatalf("unexpected page response: %#v", page)
	}
}

func TestListAgentsPaginationRejectsInvalidValues(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}

	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())
	for _, path := range []string{"/api/agents?page=0", "/api/agents?page_size=101"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		api.HandleListAgents(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, response.Code)
		}
	}
}
