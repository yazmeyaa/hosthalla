package handlers

import (
	"net/http"

	"github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
	"github.com/yazmeyaa/hosthalla/ui/widgets/layouts"
)

type HostsHandler struct {
	hostRepository storage.HostRepository
	profileService *service.Service
}

func NewHostsHandler(hostRepository storage.HostRepository, profileService *service.Service) *HostsHandler {
	return &HostsHandler{hostRepository, profileService}
}

func (h *HostsHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.hostRepository.ListHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.profileService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hosts_page.HostsPage(hosts_page.HostsPageProps{
		Hosts: hosts,
		AuthLayoutProps: layouts.AuthenticatedLayoutProps{
			GenericLayoutProps: layouts.GenericLayoutProps{Title: "Hosts"},
			Profile:            profile,
		},
	}).Render(r.Context(), w)
}
