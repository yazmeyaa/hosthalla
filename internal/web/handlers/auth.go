package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/ui/pages/auth_page"
)

type AuthHandler struct {
	l   *slog.Logger
	svc *auth_service.Service
}

func NewAuthHandler(l *slog.Logger, svc *auth_service.Service) *AuthHandler {
	return &AuthHandler{l, svc}
}

func (h *AuthHandler) Auth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	auth_page.AuthPage().Render(ctx, w)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	h.l.Debug("logging in", slog.String("username", username))

	valid, err := h.svc.ValidatePassword(r.Context(), username, password)
	if err != nil {
		h.l.Error("failed to validate password", slog.String("error", err.Error()))
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	if !valid {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	profile, err := h.svc.GetProfileByUsername(r.Context(), username)
	if err != nil {
		h.l.Error("failed to load profile", slog.String("error", err.Error()))
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	session, err := h.svc.CreateSession(r.Context(), auth_service.CreateSessionDTO{
		ProfileID: profile.ID,
	})
	if err != nil {
		h.l.Error("failed to create session", slog.String("error", err.Error()))
		http.Error(w, "Failed to login", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPSRequest(r),
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil && strings.TrimSpace(cookie.Value) != "" {
		if err := h.svc.DeleteSession(r.Context(), cookie.Value); err != nil {
			h.l.Warn("failed to delete session", slog.String("error", err.Error()))
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPSRequest(r),
		MaxAge:   -1,
	})
	http.Redirect(w, r, "/auth", http.StatusSeeOther)
}

func isHTTPSRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if forwardedProto == "" {
		return false
	}
	return strings.EqualFold(strings.Split(forwardedProto, ",")[0], "https")
}
