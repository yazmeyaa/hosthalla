package handlers

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	"github.com/yazmeyaa/hosthalla/ui/pages/index_page"
)

type IndexHandler struct {
	r host.HostRepository
	l *slog.Logger
	a *auth_service.Service
}

func NewIndexHandler(r host.HostRepository, l *slog.Logger, a *auth_service.Service) *IndexHandler {
	return &IndexHandler{r, l, a}
}

func (h *IndexHandler) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	h.l.Debug("handling index page request")
	hosts, err := h.r.ListHosts(ctx, host.ListHostsFilter{})
	if err != nil {
		h.l.Error("failed to list hosts", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.l.Info("listed hosts", slog.Int("count", len(hosts)))

	session, err := middlewares.GetSessionFromContext(ctx)
	if err != nil {
		h.l.Error("failed to read session from context", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.a.GetProfileByID(ctx, session.ProfileID)
	if err != nil {
		h.l.Error("failed to get profile", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.l.Debug("rendering index page", slog.String("profile_id", profile.ID))

	pageProps := index_page.IndexPageProps{Profile: profile}
	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(ctx, index_page.IndexPageContent(pageProps)), w)
		return
	}

	index_page.IndexPage(pageProps).Render(ctx, w)
}
