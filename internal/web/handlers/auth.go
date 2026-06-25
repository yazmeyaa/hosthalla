package handlers

import (
	"log/slog"
	"net/http"

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
	h.l.Info("logging in", slog.String("username", username))

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
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}
