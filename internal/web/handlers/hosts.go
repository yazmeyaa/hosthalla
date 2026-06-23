package handlers

import (
	"net/http"
	"net/netip"

	"github.com/google/uuid"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	host_service "github.com/yazmeyaa/hosthalla/internal/host/service"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	"github.com/yazmeyaa/hosthalla/ui/features/host_actions"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
)

type HostsHandler struct {
	hostService    *host_service.Service
	profileService *auth_service.Service
}

func NewHostsHandler(hostService *host_service.Service, profileService *auth_service.Service) *HostsHandler {
	return &HostsHandler{hostService, profileService}
}

func (h *HostsHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.hostService.ListHosts(r.Context())
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
		AuthLayoutProps: layout.AuthenticatedLayoutProps{
			GenericLayoutProps: layout.GenericLayoutProps{Title: "Hosts"},
			Profile:            profile,
		},
	}).Render(r.Context(), w)
}

func (h *HostsHandler) CreateHost(w http.ResponseWriter, r *http.Request) {
	data, err := parseHostForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err = h.hostService.CreateHost(r.Context(), data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := parseHostForm(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentHost, err := h.hostService.GetHostByID(r.Context(), hostID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	currentHost.Name = data.Name
	currentHost.Description = data.Description
	currentHost.IP = data.IP

	if err := h.hostService.UpdateHost(r.Context(), &currentHost); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.hostService.DeleteHost(r.Context(), hostID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) PingHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.hostService.PingHost(r.Context(), hostID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pingResult := &host_actions.PingResult{
		HostID:     result.HostID.String(),
		Reachable:  result.Reachable,
		DurationMS: result.Duration.Milliseconds(),
		Message:    result.ErrorMessage,
	}
	if err := host_actions.HostPingResult(result.HostID.String(), pingResult).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *HostsHandler) PingAllHosts(w http.ResponseWriter, r *http.Request) {
	results, err := h.hostService.PingAllHosts(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageResults := make([]host_actions.PingResult, 0, len(results))
	for _, result := range results {
		pageResults = append(pageResults, host_actions.PingResult{
			HostID:     result.HostID.String(),
			Reachable:  result.Reachable,
			DurationMS: result.Duration.Milliseconds(),
			Message:    result.ErrorMessage,
		})
	}

	if err := host_actions.HostPingResultsBatch(pageResults).Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseHostForm(r *http.Request) (storage.CreateHostDTO, error) {
	if err := r.ParseForm(); err != nil {
		return storage.CreateHostDTO{}, err
	}

	ip, err := netip.ParseAddr(r.FormValue("ip"))
	if err != nil {
		return storage.CreateHostDTO{}, err
	}

	return storage.CreateHostDTO{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		IP:          ip,
	}, nil
}

func parseHostID(rawHostID string) (host.HostID, error) {
	hostUUID, err := uuid.Parse(rawHostID)
	if err != nil {
		return host.HostID{}, err
	}
	return host.HostID(hostUUID), nil
}
