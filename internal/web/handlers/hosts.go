package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	"github.com/yazmeyaa/hosthalla/ui/features/host_actions"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
	"github.com/yazmeyaa/hosthalla/ui/widgets/hosts_list"
)

type HostsHandler struct {
	hostService    *host.Service
	profileService *auth_service.Service
	logger         *slog.Logger
}

type createAgentRegisterCommandResponse struct {
	Command string `json:"command"`
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
	hostManagementMethodsByHostID := make(map[string][]host.HostManagementMethod, len(hosts))
	hostSystemInfoByHostID := make(map[string]host.HostSystemInfo, len(hosts))
	hostLatestMetricsByHostID := make(map[string]hosts_list.HostLatestMetricsBadges, len(hosts))
	for _, listedHost := range hosts {
		methods, err := h.hostService.ListHostManagementMethods(r.Context(), listedHost.ID)
		if err != nil {
			h.logger.Error("failed to list host management methods in handler", slog.String("host_id", listedHost.ID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hostManagementMethodsByHostID[listedHost.ID.String()] = methods

		systemInfo, err := h.hostService.GetHostSystemInfoByHostID(r.Context(), listedHost.ID)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				h.logger.Error("failed to get host system info in handler", slog.String("host_id", listedHost.ID.String()), slog.String("error", err.Error()))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			hostSystemInfoByHostID[listedHost.ID.String()] = systemInfo
		}

		snapshots, err := h.hostService.ListHostMetricSnapshots(r.Context(), listedHost.ID)
		if err != nil {
			h.logger.Error("failed to list host metric snapshots in handler", slog.String("host_id", listedHost.ID.String()), slog.String("error", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(snapshots) == 0 || len(snapshots[0].Metrics) == 0 {
			continue
		}
		if systemInfo, ok := hostSystemInfoByHostID[listedHost.ID.String()]; ok {
			hostLatestMetricsByHostID[listedHost.ID.String()] = hosts_list.BuildHostLatestMetricsBadges(snapshots[0].Metrics[0], &systemInfo)
		} else {
			hostLatestMetricsByHostID[listedHost.ID.String()] = hosts_list.BuildHostLatestMetricsBadges(snapshots[0].Metrics[0], nil)
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

	pingResult := &host_actions.PingResult{
		HostID:     result.HostID.String(),
		Reachable:  result.Reachable,
		DurationMS: result.Duration.Milliseconds(),
		Message:    result.ErrorMessage,
	}
	if err := host_actions.HostPingResult(result.HostID.String(), pingResult).Render(r.Context(), w); err != nil {
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
		h.logger.Error("failed to render ping all results", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.logger.Info("ping all hosts request completed", slog.Int("count", len(pageResults)))
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
