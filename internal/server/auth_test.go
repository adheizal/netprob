package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"netprob/internal/auth"
	"netprob/internal/config"
	"netprob/internal/storage"
)

func TestAdminAuthenticationAndSecurityToggle(t *testing.T) {
	store := storage.NewStore(filepath.Join(t.TempDir(), "netprob.db"), nil)
	if err := store.DB.Open(); err != nil {
		t.Fatal(err)
	}
	defer store.DB.DB().Close()
	if err := store.DB.ApplyMigrations(store.DB.DB()); err != nil {
		t.Fatal(err)
	}
	passwordHash, err := auth.HashPassword("changeme")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DB.EnsureDefaultAdmin("admin@netprob.local", passwordHash); err != nil {
		t.Fatal(err)
	}

	api := NewAPIServer(store, NewHub(store), config.DefaultConfig())

	unauthorized := performRequest(api.Handler(), http.MethodGet, "/api/agents", nil, nil)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated agents response = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	login := performRequest(api.Handler(), http.MethodPost, "/api/auth/login", map[string]any{
		"email": "admin@netprob.local", "password": "changeme",
	}, nil)
	if login.Code != http.StatusOK {
		t.Fatalf("login response = %d: %s", login.Code, login.Body.String())
	}
	loginCookies := login.Result().Cookies()
	if len(loginCookies) != 1 || !loginCookies[0].HttpOnly || loginCookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login cookies = %#v", loginCookies)
	}

	mustChange := performRequest(api.Handler(), http.MethodGet, "/api/agents", nil, loginCookies[0])
	if mustChange.Code != http.StatusForbidden {
		t.Fatalf("temporary-password response = %d, want %d", mustChange.Code, http.StatusForbidden)
	}

	account := performRequest(api.Handler(), http.MethodPut, "/api/auth/account", map[string]any{
		"email": "owner@example.com", "current_password": "changeme", "new_password": "a-new-secure-password",
	}, loginCookies[0])
	if account.Code != http.StatusOK {
		t.Fatalf("account update response = %d: %s", account.Code, account.Body.String())
	}
	accountCookies := account.Result().Cookies()
	if len(accountCookies) != 1 {
		t.Fatalf("account update cookies = %#v", accountCookies)
	}

	authorized := performRequest(api.Handler(), http.MethodGet, "/api/agents", nil, accountCookies[0])
	if authorized.Code != http.StatusOK {
		t.Fatalf("authenticated agents response = %d: %s", authorized.Code, authorized.Body.String())
	}

	disable := performRequest(api.Handler(), http.MethodPut, "/api/settings/security", map[string]any{
		"auth_enabled": false,
	}, accountCookies[0])
	if disable.Code != http.StatusOK {
		t.Fatalf("disable auth response = %d: %s", disable.Code, disable.Body.String())
	}

	public := performRequest(api.Handler(), http.MethodGet, "/api/agents", nil, nil)
	if public.Code != http.StatusOK {
		t.Fatalf("auth-disabled agents response = %d: %s", public.Code, public.Body.String())
	}

	api.loginLimiter = newLoginRateLimiter(1, time.Minute)
	firstInvalid := performRequest(api.Handler(), http.MethodPost, "/api/auth/login", map[string]any{
		"email": "missing@example.com", "password": "wrong",
	}, nil)
	if firstInvalid.Code != http.StatusUnauthorized {
		t.Fatalf("first invalid login = %d, want %d", firstInvalid.Code, http.StatusUnauthorized)
	}
	rateLimited := performRequest(api.Handler(), http.MethodPost, "/api/auth/login", map[string]any{
		"email": "missing@example.com", "password": "wrong",
	}, nil)
	if rateLimited.Code != http.StatusTooManyRequests || rateLimited.Header().Get("Retry-After") == "" {
		t.Fatalf("rate-limited login = %d headers=%v", rateLimited.Code, rateLimited.Header())
	}
}

func performRequest(handler http.Handler, method, path string, body map[string]any, cookie *http.Cookie) *httptest.ResponseRecorder {
	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		encoded, _ := json.Marshal(body)
		requestBody = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, path, requestBody)
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
