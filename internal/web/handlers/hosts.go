package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
	"github.com/yazmeyaa/hosthalla/ui/shared/ui/layout"
)

type HostsHandler struct {
	hostService    *host.Service
	profileService *auth_service.Service
	logger         *slog.Logger
}

type createAgentRegisterCommandResponse struct {
	Command string `json:"command"`
}

type getHostManagementMethodSecretResponse struct {
	Password   string `json:"password,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}

type importHostsResponse struct {
	Imported      int `json:"imported"`
	Skipped       int `json:"skipped"`
	TotalReceived int `json:"totalReceived"`
}

type hostsExportResponse struct {
	Hosts    []hostImportExportDTO `json:"hosts"`
	Warnings []hostsExportWarning  `json:"warnings,omitempty"`
}

type hostsExportWarning struct {
	Host             string `json:"host"`
	IP               string `json:"ip"`
	ManagementMethod string `json:"managementMethod,omitempty"`
	Message          string `json:"message"`
}

type hostImportExportDTO struct {
	Name              string                          `json:"name"`
	Description       string                          `json:"description,omitempty"`
	Tags              []string                        `json:"tags,omitempty"`
	IP                netip.Addr                      `json:"ip"`
	ManagementMethods []hostManagementMethodExportDTO `json:"managementMethods,omitempty"`
}

type hostManagementMethodExportDTO struct {
	Type        host.HostManagementMethodType `json:"type"`
	Username    string                        `json:"username"`
	Port        uint16                        `json:"port,omitempty"`
	Description string                        `json:"description,omitempty"`
	Secret      hostManagementMethodSecretDTO `json:"secret"`
}

type hostManagementMethodSecretDTO struct {
	Password   string `json:"password,omitempty"`
	PublicKey  string `json:"publicKey,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
}

type importHostCandidate struct {
	Index   int
	Host    host.CreateHostDTO
	Methods []hostManagementMethodExportDTO
}

func NewHostsHandler(hostService *host.Service, profileService *auth_service.Service, logger *slog.Logger) *HostsHandler {
	return &HostsHandler{hostService: hostService, profileService: profileService, logger: logger}
}

func (h *HostsHandler) ListHosts(w http.ResponseWriter, r *http.Request) {
	tags := parseHostTagsValues(r.URL.Query()["tag"])
	tags = append(tags, parseHostTagsValues(r.URL.Query()["tags"])...)

	hosts, err := h.hostService.ListHosts(r.Context(), host.ListHostsFilter{Tags: tags})
	if err != nil {
		h.logger.Error("failed to list hosts in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hostIDs := make([]uuid.UUID, 0, len(hosts))
	for _, listedHost := range hosts {
		hostIDs = append(hostIDs, listedHost.ID)
	}

	hostManagementMethodsByUUID, err := h.hostService.ListHostManagementMethodsByHostIDs(r.Context(), hostIDs)
	if err != nil {
		h.logger.Error("failed to list host management methods by host ids in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hostSystemInfoByUUID, err := h.hostService.ListHostSystemInfosByHostIDs(r.Context(), hostIDs)
	if err != nil {
		h.logger.Error("failed to list host system infos by host ids in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	latestSnapshotsByUUID, err := h.hostService.ListLatestHostMetricSnapshotsByHostIDs(r.Context(), hostIDs)
	if err != nil {
		h.logger.Error("failed to list latest host metrics by host ids in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hostManagementMethodsByHostID := make(map[string][]host.HostManagementMethod, len(hosts))
	hostSystemInfoByHostID := make(map[string]host.HostSystemInfo, len(hosts))
	hostLatestMetricsByHostID := make(map[string]hosts_page.HostLatestMetricsBadges, len(hosts))
	for _, listedHost := range hosts {
		hostIDStr := listedHost.ID.String()
		if methods, ok := hostManagementMethodsByUUID[listedHost.ID]; ok {
			hostManagementMethodsByHostID[hostIDStr] = methods
		}
		systemInfo, hasSystemInfo := hostSystemInfoByUUID[listedHost.ID]
		if hasSystemInfo {
			hostSystemInfoByHostID[hostIDStr] = systemInfo
		}
		latestSnapshot, hasSnapshot := latestSnapshotsByUUID[listedHost.ID]
		if !hasSnapshot || len(latestSnapshot.Metrics) == 0 {
			continue
		}
		if hasSystemInfo {
			hostLatestMetricsByHostID[hostIDStr] = hosts_page.BuildHostLatestMetricsBadges(latestSnapshot.Metrics[0], &systemInfo)
		} else {
			hostLatestMetricsByHostID[hostIDStr] = hosts_page.BuildHostLatestMetricsBadges(latestSnapshot.Metrics[0], nil)
		}
	}

	availableTags, err := h.hostService.ListTags(r.Context())
	if err != nil {
		h.logger.Error("failed to list tags in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session from context", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.profileService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for hosts page", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("rendering hosts page", slog.Int("hosts", len(hosts)), slog.Int("available_tags", len(availableTags)), slog.String("profile_id", profile.ID))

	pageProps := hosts_page.HostsPageProps{
		Hosts:                         hosts,
		AvailableTags:                 availableTags,
		SelectedTags:                  tags,
		HostManagementMethodsByHostID: hostManagementMethodsByHostID,
		HostSystemInfoByHostID:        hostSystemInfoByHostID,
		HostLatestMetricsByHostID:     hostLatestMetricsByHostID,
		AuthLayoutProps: layout.AuthenticatedLayoutProps{
			GenericLayoutProps: layout.GenericLayoutProps{Title: "Hosts"},
			Profile:            profile,
			Path:               r.URL.Path,
		},
	}
	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(r.Context(), hosts_page.HostsPageContent(pageProps)), w)
		return
	}

	hosts_page.HostsPage(pageProps).Render(r.Context(), w)
}

func (h *HostsHandler) CreateHost(w http.ResponseWriter, r *http.Request) {
	data, err := parseHostForm(r)
	if err != nil {
		h.logger.Warn("invalid create host payload", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err = h.hostService.CreateHost(r.Context(), data); err != nil {
		h.logger.Error("failed to create host in handler", slog.String("name", data.Name), slog.String("ip", data.IP.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Info("create host request completed", slog.String("name", data.Name), slog.String("ip", data.IP.String()))

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in update host request", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := parseHostForm(r)
	if err != nil {
		h.logger.Warn("invalid update host payload", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	currentHost, err := h.hostService.GetHostByID(r.Context(), hostID)
	if err != nil {
		h.logger.Warn("host not found in update host request", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	currentHost.Name = data.Name
	currentHost.Description = data.Description
	if hostTagsSubmitted(r) {
		currentHost.Tags = data.Tags
	}
	currentHost.IP = data.IP

	if err := h.hostService.UpdateHost(r.Context(), &currentHost); err != nil {
		h.logger.Error("failed to update host in handler", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Info("update host request completed", slog.String("host_id", hostID.String()))

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in delete host request", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.hostService.DeleteHost(r.Context(), hostID); err != nil {
		h.logger.Error("failed to delete host in handler", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Warn("delete host request completed", slog.String("host_id", hostID.String()))

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) PingHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in ping host request", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.hostService.PingHost(r.Context(), hostID)
	if err != nil {
		h.logger.Error("failed to ping host in handler", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Debug("ping host request completed", slog.String("host_id", hostID.String()), slog.Bool("reachable", result.Reachable), slog.Int64("duration_ms", result.Duration.Milliseconds()))

	pingResult := &hosts_page.PingResult{
		HostID:     result.HostID.String(),
		Reachable:  result.Reachable,
		DurationMS: result.Duration.Milliseconds(),
		Message:    result.ErrorMessage,
	}
	if err := hosts_page.HostPingResult(result.HostID.String(), pingResult).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render ping host result", slog.String("host_id", result.HostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *HostsHandler) PingAllHosts(w http.ResponseWriter, r *http.Request) {
	results, err := h.hostService.PingAllHosts(r.Context())
	if err != nil {
		h.logger.Error("failed to ping all hosts in handler", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageResults := make([]hosts_page.PingResult, 0, len(results))
	for _, result := range results {
		pageResults = append(pageResults, hosts_page.PingResult{
			HostID:     result.HostID.String(),
			Reachable:  result.Reachable,
			DurationMS: result.Duration.Milliseconds(),
			Message:    result.ErrorMessage,
		})
	}

	if err := hosts_page.HostPingResultsBatch(pageResults).Render(r.Context(), w); err != nil {
		h.logger.Error("failed to render ping all results", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *HostsHandler) CreateHostManagementMethod(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in create management method request", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Warn("failed to parse create management method form", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	methodType := strings.TrimSpace(r.FormValue("methodType"))
	switch methodType {
	case string(host.HostManagementMethodTypeSSHPassword):
		port, err := host.ParsePort(r.FormValue("port"))
		if err != nil {
			h.logger.Warn("invalid port in ssh password method request", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = h.hostService.CreateSSHPasswordManagementMethod(r.Context(), hostID, host.CreateSSHPasswordManagementMethodDTO{
			Username:    r.FormValue("username"),
			Password:    r.FormValue("password"),
			Port:        port,
			Description: r.FormValue("description"),
		})
		if err != nil {
			h.logger.Warn("failed to create ssh password method in handler", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Info("created ssh password method request completed", slog.String("host_id", hostID.String()))
	case string(host.HostManagementMethodTypeSSHKey):
		port, err := host.ParsePort(r.FormValue("port"))
		if err != nil {
			h.logger.Warn("invalid port in ssh key method request", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err = h.hostService.CreateSSHKeyManagementMethod(r.Context(), hostID, host.CreateSSHKeyManagementMethodDTO{
			Username:    r.FormValue("username"),
			PublicKey:   r.FormValue("publicKey"),
			PrivateKey:  r.FormValue("privateKey"),
			Port:        port,
			Description: r.FormValue("description"),
		})
		if err != nil {
			h.logger.Warn("failed to create ssh key method in handler", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logger.Info("created ssh key method request completed", slog.String("host_id", hostID.String()))
	default:
		h.logger.Warn("unsupported management method type", slog.String("host_id", hostID.String()), slog.String("method_type", methodType))
		http.Error(w, "unsupported management method type", http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func (h *HostsHandler) GetHostManagementMethodSecret(w http.ResponseWriter, r *http.Request) {
	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in management method secret request", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	methodID, err := parseHostID(r.PathValue("methodID"))
	if err != nil {
		h.logger.Warn("invalid management method id in secret request", slog.String("host_id", hostID.String()), slog.String("method_id", r.PathValue("methodID")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	secret, err := h.hostService.GetHostManagementMethodSecret(r.Context(), hostID, methodID)
	if err != nil {
		h.logger.Warn("failed to get management method secret in handler", slog.String("host_id", hostID.String()), slog.String("method_id", methodID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(getHostManagementMethodSecretResponse{
		Password:   secret.Password,
		PublicKey:  secret.PublicKey,
		PrivateKey: secret.PrivateKey,
	}); err != nil {
		h.logger.Error("failed to encode management method secret response", slog.String("host_id", hostID.String()), slog.String("method_id", methodID.String()), slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *HostsHandler) CreateAgentRegisterCommand(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session for create agent register command", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hostID, err := parseHostID(r.PathValue("id"))
	if err != nil {
		h.logger.Warn("invalid host id in create agent register command", slog.String("host_id", r.PathValue("id")), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := h.hostService.GetHostByID(r.Context(), hostID); err != nil {
		h.logger.Warn("host not found in create agent register command", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	tokenName := fmt.Sprintf("Agent token for host %s (%s)", hostID.String(), time.Now().UTC().Format(time.RFC3339))
	createdToken, err := h.profileService.CreateAPIToken(r.Context(), auth_service.CreateAPITokenDTO{
		ProfileID: session.ProfileID,
		Name:      tokenName,
		Scopes:    []string{"hosts:register"},
		ExpiresIn: 0,
	})
	if err != nil {
		h.logger.Error("failed to create api token for agent register command", slog.String("host_id", hostID.String()), slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	serverURL := resolvePublicServerURL(r)
	command := fmt.Sprintf(
		"hosthalla agent register --host=%s --host-id=%s --token=%s",
		serverURL,
		hostID.String(),
		createdToken.PlainToken,
	)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(createAgentRegisterCommandResponse{Command: command}); err != nil {
		h.logger.Error("failed to encode create agent register command response", slog.String("host_id", hostID.String()), slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func parseHostForm(r *http.Request) (host.CreateHostDTO, error) {
	if err := r.ParseForm(); err != nil {
		return host.CreateHostDTO{}, err
	}

	ip, err := netip.ParseAddr(r.FormValue("ip"))
	if err != nil {
		return host.CreateHostDTO{}, err
	}

	return host.CreateHostDTO{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Tags:        append(parseHostTagsValues(r.Form["tag"]), parseHostTagsValues(r.Form["tags"])...),
		IP:          ip,
	}, nil
}

func parseHostTagsValues(values []string) []string {
	tags := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		for _, tag := range parseHostTags(value) {
			normalized := strings.ToLower(strings.TrimSpace(tag))
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			tags = append(tags, normalized)
		}
	}
	return tags
}

func parseHostTags(rawTags string) []string {
	return strings.FieldsFunc(rawTags, func(r rune) bool {
		return r == ',' || r == '\n'
	})
}

func hostTagsSubmitted(r *http.Request) bool {
	_, hasTag := r.Form["tag"]
	_, hasTags := r.Form["tags"]
	return hasTag || hasTags
}

func parseHostID(rawHostID string) (uuid.UUID, error) {
	hostUUID, err := uuid.Parse(rawHostID)
	if err != nil {
		return uuid.UUID{}, err
	}
	return hostUUID, nil
}

func resolvePublicServerURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	if forwardedProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwardedProto != "" {
		scheme = strings.Split(forwardedProto, ",")[0]
	}

	host := strings.TrimSpace(r.Host)
	if forwardedHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
		host = strings.Split(forwardedHost, ",")[0]
	}

	if host == "" {
		host = "localhost"
	}

	return scheme + "://" + host
}

func (h *HostsHandler) ExportHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := h.hostService.ListHosts(r.Context(), host.ListHostsFilter{})
	if err != nil {
		h.logger.Error("failed to list hosts in export hosts", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	hostIDs := make([]uuid.UUID, 0, len(hosts))
	for _, exportedHost := range hosts {
		hostIDs = append(hostIDs, exportedHost.ID)
	}

	managementMethodsByHostID, err := h.hostService.ListHostManagementMethodsByHostIDs(r.Context(), hostIDs)
	if err != nil {
		h.logger.Error("failed to list management methods in export hosts", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	exportedHosts := make([]hostImportExportDTO, 0, len(hosts))
	exportWarnings := make([]hostsExportWarning, 0)
	for _, exportedHost := range hosts {
		managementMethods := managementMethodsByHostID[exportedHost.ID]
		exportedManagementMethods := make([]hostManagementMethodExportDTO, 0, len(managementMethods))
		for _, method := range managementMethods {
			secret, err := h.hostService.GetHostManagementMethodSecret(r.Context(), exportedHost.ID, method.ID)
			if err != nil {
				h.logger.Warn("skipping host management method in export because secret cannot be decrypted", slog.String("host_id", exportedHost.ID.String()), slog.String("method_id", method.ID.String()), slog.String("error", err.Error()))
				exportWarnings = append(exportWarnings, hostsExportWarning{
					Host:             exportedHost.Name,
					IP:               exportedHost.IP.String(),
					ManagementMethod: formatExportManagementMethod(method),
					Message:          "management method secret could not be decrypted and was skipped",
				})
				continue
			}

			exportedManagementMethods = append(exportedManagementMethods, hostManagementMethodExportDTO{
				Type:        method.Type,
				Username:    method.Username,
				Port:        method.Port,
				Description: method.Description,
				Secret: hostManagementMethodSecretDTO{
					Password:   secret.Password,
					PublicKey:  secret.PublicKey,
					PrivateKey: secret.PrivateKey,
				},
			})
		}
		exportedHosts = append(exportedHosts, hostImportExportDTO{
			Name:              exportedHost.Name,
			Description:       exportedHost.Description,
			Tags:              exportedHost.Tags,
			IP:                exportedHost.IP,
			ManagementMethods: exportedManagementMethods,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="hh_hosts_export_%d.json"`, time.Now().UTC().Unix()))
	if err := json.NewEncoder(w).Encode(hostsExportResponse{Hosts: exportedHosts, Warnings: exportWarnings}); err != nil {
		h.logger.Error("failed to encode hosts export response", slog.String("error", err.Error()))
	}
}

func (h *HostsHandler) ImportHosts(w http.ResponseWriter, r *http.Request) {
	var reader io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		file, _, err := r.FormFile("hosts")
		if err != nil {
			h.logger.Warn("invalid import hosts payload", slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		reader = file
	}

	importedHosts, err := decodeHostsImportPayload(reader)
	if err != nil {
		h.logger.Warn("failed to decode hosts import payload", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	existingHosts, err := h.hostService.ListHosts(r.Context(), host.ListHostsFilter{})
	if err != nil {
		h.logger.Error("failed to list hosts in import hosts", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	existingIPs := make(map[string]struct{}, len(existingHosts)+len(importedHosts))
	for _, existingHost := range existingHosts {
		existingIPs[existingHost.IP.String()] = struct{}{}
	}

	seenImportIPs := make(map[string]struct{}, len(importedHosts))
	candidates := make([]importHostCandidate, 0, len(importedHosts))
	skippedCount := 0
	for idx, importedHost := range importedHosts {
		if strings.TrimSpace(importedHost.Name) == "" || !importedHost.IP.IsValid() {
			h.logger.Warn("invalid host in import payload", slog.Int("index", idx), slog.String("name", importedHost.Name), slog.String("ip", importedHost.IP.String()))
			http.Error(w, fmt.Sprintf("invalid host at index %d", idx), http.StatusBadRequest)
			return
		}

		importedIP := importedHost.IP.String()
		if _, ok := existingIPs[importedIP]; ok {
			skippedCount++
			continue
		}
		if _, ok := seenImportIPs[importedIP]; ok {
			skippedCount++
			continue
		}
		seenImportIPs[importedIP] = struct{}{}
		if err := validateImportedManagementMethods(importedHost.ManagementMethods); err != nil {
			h.logger.Warn("invalid host management method in import payload", slog.Int("index", idx), slog.String("error", err.Error()))
			http.Error(w, fmt.Sprintf("invalid host at index %d: %s", idx, err.Error()), http.StatusBadRequest)
			return
		}
		candidates = append(candidates, importHostCandidate{Index: idx, Host: host.CreateHostDTO{
			Name:        importedHost.Name,
			Description: importedHost.Description,
			Tags:        importedHost.Tags,
			IP:          importedHost.IP,
		}, Methods: importedHost.ManagementMethods})
	}

	importedCount := 0
	for _, candidate := range candidates {
		createdHost, err := h.hostService.CreateHost(r.Context(), candidate.Host)
		if err != nil {
			h.logger.Error("failed to import host", slog.Int("index", candidate.Index), slog.String("name", candidate.Host.Name), slog.String("ip", candidate.Host.IP.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for methodIdx, method := range candidate.Methods {
			if err := h.importHostManagementMethod(r.Context(), createdHost.ID, candidate.Index, methodIdx, method); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		importedCount++
	}

	h.logger.Info("hosts import request completed", slog.Int("imported", importedCount), slog.Int("skipped", skippedCount), slog.Int("total_received", len(importedHosts)))
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(importHostsResponse{Imported: importedCount, Skipped: skippedCount, TotalReceived: len(importedHosts)}); err != nil {
			h.logger.Error("failed to encode hosts import response", slog.String("error", err.Error()))
		}
		return
	}
	http.Redirect(w, r, "/hosts", http.StatusSeeOther)
}

func decodeHostsImportPayload(reader io.Reader) ([]hostImportExportDTO, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(reader).Decode(&raw); err != nil {
		return nil, err
	}

	var importedHosts []hostImportExportDTO
	if err := json.Unmarshal(raw, &importedHosts); err == nil {
		return importedHosts, nil
	}

	var payload hostsExportResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload.Hosts == nil {
		return nil, fmt.Errorf("JSON must contain a hosts array")
	}
	return payload.Hosts, nil
}

func formatExportManagementMethod(method host.HostManagementMethod) string {
	parts := []string{string(method.Type), method.Username}
	if method.Description != "" {
		parts = append(parts, method.Description)
	}
	return strings.Join(parts, " / ")
}

func validateImportedManagementMethods(methods []hostManagementMethodExportDTO) error {
	for idx, method := range methods {
		if strings.TrimSpace(method.Username) == "" {
			return fmt.Errorf("management method %d username is required", idx)
		}
		switch method.Type {
		case host.HostManagementMethodTypeSSHPassword:
			if strings.TrimSpace(method.Secret.Password) == "" {
				return fmt.Errorf("management method %d password is required", idx)
			}
		case host.HostManagementMethodTypeSSHKey:
			if strings.TrimSpace(method.Secret.PublicKey) == "" {
				return fmt.Errorf("management method %d public key is required", idx)
			}
			if strings.TrimSpace(method.Secret.PrivateKey) == "" {
				return fmt.Errorf("management method %d private key is required", idx)
			}
		default:
			return fmt.Errorf("management method %d has unsupported type %q", idx, method.Type)
		}
	}
	return nil
}

func (h *HostsHandler) importHostManagementMethod(ctx context.Context, hostID uuid.UUID, hostIndex int, methodIndex int, method hostManagementMethodExportDTO) error {
	switch method.Type {
	case host.HostManagementMethodTypeSSHPassword:
		_, err := h.hostService.CreateSSHPasswordManagementMethod(ctx, hostID, host.CreateSSHPasswordManagementMethodDTO{
			Username:    method.Username,
			Password:    method.Secret.Password,
			Port:        method.Port,
			Description: method.Description,
		})
		if err != nil {
			h.logger.Warn("failed to import ssh password management method", slog.String("host_id", hostID.String()), slog.Int("host_index", hostIndex), slog.Int("method_index", methodIndex), slog.String("error", err.Error()))
			return fmt.Errorf("failed to import management method %d for host %d: %w", methodIndex, hostIndex, err)
		}
	case host.HostManagementMethodTypeSSHKey:
		_, err := h.hostService.CreateSSHKeyManagementMethod(ctx, hostID, host.CreateSSHKeyManagementMethodDTO{
			Username:    method.Username,
			PublicKey:   method.Secret.PublicKey,
			PrivateKey:  method.Secret.PrivateKey,
			Port:        method.Port,
			Description: method.Description,
		})
		if err != nil {
			h.logger.Warn("failed to import ssh key management method", slog.String("host_id", hostID.String()), slog.Int("host_index", hostIndex), slog.Int("method_index", methodIndex), slog.String("error", err.Error()))
			return fmt.Errorf("failed to import management method %d for host %d: %w", methodIndex, hostIndex, err)
		}
	default:
		return fmt.Errorf("unsupported management method type %q", method.Type)
	}
	return nil
}
