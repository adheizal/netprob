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

func TestListLinksPaginationAndOverviewAreBounded(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if err := store.DB.CreateAgent(models.NewAgent("offline-agent", "test", []string{}, []string{})); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"link-a", "link-b", "link-c"} {
		if err := store.DB.CreateLink(models.NewLink(name, "")); err != nil {
			t.Fatal(err)
		}
	}

	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/links?page=1&page_size=2&status=inactive", nil)
	response := httptest.NewRecorder()
	api.HandleListLinks(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("links response status = %d: %s", response.Code, response.Body.String())
	}
	var linksPage linkPageResponse
	if err := json.NewDecoder(response.Body).Decode(&linksPage); err != nil {
		t.Fatal(err)
	}
	if linksPage.Total != 3 || linksPage.TotalPages != 2 || len(linksPage.Links) != 2 {
		t.Fatalf("links page = %#v", linksPage)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/overview?problem_limit=2", nil)
	response = httptest.NewRecorder()
	api.HandleOverview(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("overview response status = %d: %s", response.Code, response.Body.String())
	}
	var overview overviewResponse
	if err := json.NewDecoder(response.Body).Decode(&overview); err != nil {
		t.Fatal(err)
	}
	if overview.Agents.Total != 12 || overview.Links.Total != 3 || len(overview.ProblemLinks) != 2 || len(overview.OfflineAgents) != 2 {
		t.Fatalf("overview = %#v", overview)
	}
}

func TestListLinksPaginationRejectsInvalidValues(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())
	for _, path := range []string{
		"/api/links?page=0", "/api/links?page_size=101", "/api/links?status=unknown", "/api/links?sort=unknown", "/api/links?order=sideways",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		api.HandleListLinks(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", path, response.Code)
		}
	}
}

func TestLegacyListLinksSupportsMoreThanSQLiteVariableLimit(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 501; i++ {
		if err := store.DB.CreateLink(models.NewLink("legacy-link", "")); err != nil {
			t.Fatal(err)
		}
	}

	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())
	request := httptest.NewRequest(http.MethodGet, "/api/links", nil)
	response := httptest.NewRecorder()
	api.HandleListLinks(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response status = %d: %s", response.Code, response.Body.String())
	}
	var links []linkWithAgents
	if err := json.NewDecoder(response.Body).Decode(&links); err != nil {
		t.Fatal(err)
	}
	if len(links) != 501 {
		t.Fatalf("legacy links length = %d, want 501", len(links))
	}
}
