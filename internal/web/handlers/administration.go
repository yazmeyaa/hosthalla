package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/pages/administration_page"
)

type AdministrationHandler struct {
	authService *auth_service.Service
}

func NewAdministrationHandler(authService *auth_service.Service) *AdministrationHandler {
	return &AdministrationHandler{authService: authService}
}

func (h *AdministrationHandler) Administration(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	apiTokens, err := h.authService.ListAPITokensByProfileID(r.Context(), profile.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	administration_page.AdministrationPage(administration_page.AdministrationPageProps{
		Profile:   profile,
		APITokens: apiTokens,
	}).Render(r.Context(), w)
}

func (h *AdministrationHandler) CreateAPIToken(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expiresIn, err := parseTokenExpiresInDays(r.FormValue("expiresInDays"))
	if err != nil {
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	apiTokens, err := h.authService.ListAPITokensByProfileID(r.Context(), profile.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	administration_page.AdministrationPage(administration_page.AdministrationPageProps{
		Profile:         profile,
		APITokens:       apiTokens,
		CreatedAPIToken: createdToken.PlainToken,
	}).Render(r.Context(), w)
}

func (h *AdministrationHandler) RevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	token, err := h.authService.GetAPITokenByID(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if token.ProfileID != session.ProfileID {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}

	if err := h.authService.RevokeAPIToken(r.Context(), token.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

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
