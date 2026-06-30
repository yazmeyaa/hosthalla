package handlers

import (
	"log/slog"
	"net/http"

	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type IndexHandler struct {
	s *host.Service
	l *slog.Logger
	a *auth_service.Service
}

func NewIndexHandler(s *host.Service, l *slog.Logger, a *auth_service.Service) *IndexHandler {
	return &IndexHandler{s, l, a}
}

func (h *IndexHandler) Index(w http.ResponseWriter, r *http.Request) {
	h.l.Debug("handling index page request")
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}
