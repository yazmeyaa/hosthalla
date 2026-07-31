package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/pages/help_page"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/layout"
)

type HelpHandler struct {
	authService *auth_service.Service
	logger      *slog.Logger
}

func NewHelpHandler(authService *auth_service.Service, logger *slog.Logger) *HelpHandler {
	return &HelpHandler{authService: authService, logger: logger}
}

func (h *HelpHandler) Help(w http.ResponseWriter, r *http.Request) {
	topic := r.PathValue("topic")
	if topic == "" {
		http.Redirect(w, r, "/help/install-server", http.StatusSeeOther)
		return
	}
	if !validHelpTopic(topic) {
		http.NotFound(w, r)
		return
	}

	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for help page", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.authService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for help page", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := help_page.HelpPageProps{
		ActiveTopic: topic,
		AuthLayoutProps: layout.AuthenticatedLayoutProps{
			GenericLayoutProps: layout.GenericLayoutProps{Title: "Help"},
			Profile:            profile,
			Path:               r.URL.Path,
		},
	}

	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(r.Context(), help_page.HelpPageContent(props)), w)
		return
	}

	help_page.HelpPage(props).Render(r.Context(), w)
}

func validHelpTopic(topic string) bool {
	switch topic {
	case "install-server", "install-agent", "create-api-key", "create-user", "cli-usage", "update-hosthalla", "report-bug":
		return true
	}
	return false
}
