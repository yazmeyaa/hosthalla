package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	"github.com/yazmeyaa/hosthalla/ui/pages/administration_page"
)

type AdministrationHandler struct {
	authService *auth_service.Service
	logger      *slog.Logger
}

func NewAdministrationHandler(authService *auth_service.Service, logger *slog.Logger) *AdministrationHandler {
	return &AdministrationHandler{authService: authService, logger: logger}
}

func (h *AdministrationHandler) Administration(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for administration page", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for administration page", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTokens, err := h.authService.ListAPITokensByProfileID(r.Context(), profile.ID)
	if err != nil {
		h.logger.Error("failed to list api tokens", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("rendering administration page", slog.String("profile_id", profile.ID), slog.Int("api_tokens_count", len(apiTokens)))

	pageProps := administration_page.AdministrationPageProps{
		Profile:   profile,
		APITokens: apiTokens,
	}
	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(r.Context(), administration_page.AdministrationPageContent(pageProps)), w)
		return
	}

	administration_page.AdministrationPage(pageProps).Render(r.Context(), w)
}

func (h *AdministrationHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for create api token", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for create api token", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Warn("invalid create api token payload", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expiresIn, err := parseTokenExpiresInDays(r.FormValue("expiresInDays"))
	if err != nil {
		h.logger.Warn("invalid expiresInDays value", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	createdToken, err := h.authService.CreateAPIToken(r.Context(), auth_service.CreateAPITokenDTO{
		ProfileID: profile.ID,
		Name:      r.FormValue("name"),
		Scopes:    parseScopes(r.Form["scope"]),
		ExpiresIn: expiresIn,
	})
	if err != nil {
		h.logger.Warn("failed to create api token", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiTokens, err := h.authService.ListAPITokensByProfileID(r.Context(), profile.ID)
	if err != nil {
		h.logger.Error("failed to list api tokens after token creation", slog.String("profile_id", profile.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Info("api token created", slog.String("profile_id", profile.ID), slog.String("token_id", createdToken.Token.ID))

	administration_page.AdministrationPage(administration_page.AdministrationPageProps{
		Profile:         profile,
		APITokens:       apiTokens,
		CreatedAPIToken: createdToken.PlainToken,
	}).Render(r.Context(), w)
}

func (h *AdministrationHandler) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for revoke api token", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := h.authService.GetAPITokenByID(r.Context(), r.PathValue("id"))
	if err != nil {
		h.logger.Warn("api token not found for revoke", slog.String("token_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if token.ProfileID != session.ProfileID {
		h.logger.Warn("api token revoke denied: profile mismatch", slog.String("token_id", token.ID), slog.String("session_profile_id", session.ProfileID), slog.String("token_profile_id", token.ProfileID))
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}

	if err := h.authService.RevokeAPIToken(r.Context(), token.ID); err != nil {
		h.logger.Warn("failed to revoke api token", slog.String("token_id", token.ID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logger.Warn("api token revoked", slog.String("token_id", token.ID), slog.String("profile_id", token.ProfileID))

	http.Redirect(w, r, "/administration", http.StatusSeeOther)
}

func parseScopes(rawScopes []string) []string {
	return rawScopes
}

func parseTokenExpiresInDays(rawValue string) (time.Duration, error) {
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return 0, nil
	}

	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if days < 0 {
		return 0, errors.New("expiresInDays must be greater or equal to 0")
	}
	return time.Duration(days) * 24 * time.Hour, nil
}
