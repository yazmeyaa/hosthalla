package handlers

import (
	"net/http"

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

	administration_page.AdministrationPage(administration_page.AdministrationPageProps{
		Profile: profile,
	}).Render(r.Context(), w)
}
