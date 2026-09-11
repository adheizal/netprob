package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"netprob/internal/auth"
	"netprob/internal/models"
)

const (
	adminSessionCookie = "netprob_session"
	adminSessionTTL    = 7 * 24 * time.Hour
)

type adminContextKey struct{}

func (s *APIServer) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		security, err := s.store.DB.GetSecuritySettings()
		if err != nil {
			http.Error(w, "failed to load security settings", http.StatusInternalServerError)
			return
		}
		if !security.AuthEnabled {
			next.ServeHTTP(w, r)
			return
		}

		admin, _ := s.adminFromRequest(r)
		if admin == nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if admin.MustChangePassword && r.URL.Path != "/api/auth/account" {
			http.Error(w, "password change required", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminContextKey{}, admin)))
	})
}

func (s *APIServer) HandleGetAuthSession(w http.ResponseWriter, r *http.Request) {
	security, err := s.store.DB.GetSecuritySettings()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response := authSessionResponse{AuthEnabled: security.AuthEnabled}
	if admin, _ := s.adminFromRequest(r); admin != nil {
		response.Authenticated = true
		response.Email = admin.Email
		response.MustChangePassword = admin.MustChangePassword
	}
	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	admin, err := s.store.DB.GetAdminByEmail(req.Email)
	if err != nil || !auth.VerifyPassword(req.Password, admin.PasswordHash) {
		http.Error(w, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if err := s.issueAdminSession(w, r, admin); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(authSessionResponse{
		AuthEnabled: true, Authenticated: true, Email: admin.Email, MustChangePassword: admin.MustChangePassword,
	})
}

func (s *APIServer) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(adminSessionCookie); err == nil {
		_ = s.store.DB.DeleteAdminSession(auth.HashToken(cookie.Value))
	}
	clearAdminCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *APIServer) HandleUpdateAdminAccount(w http.ResponseWriter, r *http.Request) {
	admin, _ := r.Context().Value(adminContextKey{}).(*models.AdminUser)
	if admin == nil {
		admin, _ = s.adminFromRequest(r)
	}
	if admin == nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	var req updateAdminAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !strings.Contains(email, "@") {
		http.Error(w, "valid email is required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 8 {
		http.Error(w, "new password must contain at least 8 characters", http.StatusBadRequest)
		return
	}
	if !auth.VerifyPassword(req.CurrentPassword, admin.PasswordHash) {
		http.Error(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}
	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	if err := s.store.DB.UpdateAdminCredentials(admin.ID, email, passwordHash); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	admin.Email = email
	admin.PasswordHash = passwordHash
	admin.MustChangePassword = false
	if err := s.issueAdminSession(w, r, admin); err != nil {
		http.Error(w, "password changed but session renewal failed", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(authSessionResponse{AuthEnabled: true, Authenticated: true, Email: email})
}

func (s *APIServer) HandleUpdateSecuritySettings(w http.ResponseWriter, r *http.Request) {
	var req updateSecuritySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AuthEnabled == nil {
		http.Error(w, "auth_enabled is required", http.StatusBadRequest)
		return
	}
	settings, err := s.store.DB.UpdateSecuritySettings(*req.AuthEnabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(settings)
}

func (s *APIServer) adminFromRequest(r *http.Request) (*models.AdminUser, error) {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil || cookie.Value == "" {
		return nil, sql.ErrNoRows
	}
	return s.store.DB.GetAdminBySession(auth.HashToken(cookie.Value), time.Now().UTC())
}

func (s *APIServer) issueAdminSession(w http.ResponseWriter, r *http.Request, admin *models.AdminUser) error {
	token, err := auth.GenerateToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(adminSessionTTL)
	if err := s.store.DB.CreateAdminSession(auth.HashToken(token), admin.ID, expiresAt); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: adminSessionCookie, Value: token, Path: "/", Expires: expiresAt,
		HttpOnly: true, Secure: requestIsHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func clearAdminCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: adminSessionCookie, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: requestIsHTTPS(r), SameSite: http.SameSiteStrictMode,
	})
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
