package handlers

import (
	"net/http"
	"net/netip"
	"strconv"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
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

	if _, err = h.hostRepository.CreateHost(r.Context(), data); err != nil {
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

	currentHost, err := h.hostRepository.GetHostByID(r.Context(), hostID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	currentHost.Name = data.Name
	currentHost.Description = data.Description
	currentHost.IP = data.IP
	currentHost.Port = data.Port

	if err := h.hostRepository.UpdateHost(r.Context(), &currentHost); err != nil {
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

	if err := h.hostRepository.DeleteHost(r.Context(), hostID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func parseHostForm(r *http.Request) (storage.CreateHostDTO, error) {
	if err := r.ParseForm(); err != nil {
		return storage.CreateHostDTO{}, err
	}

	portValue, err := strconv.ParseUint(r.FormValue("port"), 10, 16)
	if err != nil {
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
		Port:        uint16(portValue),
	}, nil
}

func parseHostID(rawHostID string) (host.HostID, error) {
	hostUUID, err := uuid.Parse(rawHostID)
	if err != nil {
		return host.HostID{}, err
	}
	return host.HostID(hostUUID), nil
}
