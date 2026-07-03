package handlers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/agent"
	auth_service "github.com/yazmeyaa/hosthalla/internal/authentication/service"
	"github.com/yazmeyaa/hosthalla/internal/events"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/web/middlewares"
	"github.com/yazmeyaa/hosthalla/ui/app/layout"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
)

const dashboardCacheTTL = 30 * time.Second

type DashboardHandler struct {
	logger         *slog.Logger
	hostService    *host.Service
	profileService *auth_service.Service

	mu    sync.RWMutex
	cache dashboardCache

	clientsMu sync.RWMutex
	clients   map[*dashboardClient]struct{}
}

type DashboardHandlerParams struct {
	Logger         *slog.Logger
	HostService    *host.Service
	ProfileService *auth_service.Service
	EventBus       events.EventBus
}

type dashboardCache struct {
	data      dashboard_page.DashboardData
	expiresAt time.Time
}

type dashboardUpdateSections uint8

const (
	dashboardUpdateGeneratedAt dashboardUpdateSections = 1 << iota
	dashboardUpdateOverview
	dashboardUpdateHosts
	dashboardUpdateHostRows

	dashboardUpdateAll = dashboardUpdateGeneratedAt | dashboardUpdateOverview | dashboardUpdateHosts
)

type dashboardUpdate struct {
	sections dashboardUpdateSections
	hostIDs  map[uuid.UUID]struct{}
}

type dashboardClient struct {
	updateCh chan dashboardUpdate
}

func NewDashboardHandler(params DashboardHandlerParams) *DashboardHandler {
	h := &DashboardHandler{
		logger:         params.Logger.With("component", "dashboard_handler"),
		hostService:    params.HostService,
		profileService: params.ProfileService,
		clients:        make(map[*dashboardClient]struct{}),
	}

	if params.EventBus != nil {
		h.subscribeDashboardEvent(params.EventBus, host.CreateHostEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.UpdateHostEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.DeleteHostEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostMetricReceivedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostMonitoringAgentAssignedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostManagementMethodCreatedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostPingCompletedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostsPingCompletedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostSystemInfoUpdatedEvent{})
		h.subscribeDashboardEvent(params.EventBus, host.HostMetricSnapshotCreatedEvent{})
		h.subscribeDashboardEvent(params.EventBus, agent.AgentRegisteredEvent{})
		h.subscribeDashboardEvent(params.EventBus, agent.AgentUpdatedEvent{})
		h.subscribeDashboardEvent(params.EventBus, agent.AgentDeletedEvent{})
		h.subscribeDashboardEvent(params.EventBus, agent.AgentLastSeenUpdatedEvent{})
	}

	return h
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	session, err := middlewares.GetSessionFromContext(r.Context())
	if err != nil {
		h.logger.Error("failed to get session from context", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	profile, err := h.profileService.GetProfileByID(r.Context(), session.ProfileID)
	if err != nil {
		h.logger.Error("failed to load profile for dashboard", slog.String("profile_id", session.ProfileID), slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := h.dashboardData(r.Context())
	if err != nil {
		h.logger.Error("failed to build dashboard data", slog.String("error", err.Error()))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageProps := dashboard_page.DashboardPageProps{
		Data: data,
		AuthLayoutProps: layout.AuthenticatedLayoutProps{
			GenericLayoutProps: layout.GenericLayoutProps{Title: "Dashboard"},
			Profile:            profile,
		},
	}

	if isHTMXBoostedNavigationRequest(r) {
		layout.AppContent().Render(templ.WithChildren(r.Context(), dashboard_page.DashboardPageContent(pageProps)), w)
		return
	}

	dashboard_page.DashboardPage(pageProps).Render(r.Context(), w)
}

func (h *DashboardHandler) SubscribeToDashboard(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		h.logger.Error("failed to accept dashboard websocket", slog.String("error", err.Error()))
		return
	}

	client := &dashboardClient{updateCh: make(chan dashboardUpdate, 8)}
	h.registerClient(client)
	defer h.unregisterClient(client)
	defer conn.Close(websocket.StatusNormalClosure, "dashboard subscription closed")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	ctx = conn.CloseRead(ctx)

	h.signalClient(client, dashboardUpdate{sections: dashboardUpdateAll})
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-client.updateCh:
			payload, err := h.renderLiveUpdate(ctx, update)
			if err != nil {
				h.logger.Error("failed to render dashboard websocket update", slog.String("error", err.Error()))
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err = conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				h.logger.Debug("dashboard websocket disconnected", slog.String("error", err.Error()))
				return
			}
		}
	}
}

func (h *DashboardHandler) dashboardData(ctx context.Context) (dashboard_page.DashboardData, error) {
	now := time.Now()

	h.mu.RLock()
	if now.Before(h.cache.expiresAt) {
		data := h.cache.data
		h.mu.RUnlock()
		return data, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	now = time.Now()
	if now.Before(h.cache.expiresAt) {
		return h.cache.data, nil
	}

	data, err := h.collectDashboardData(ctx, now)
	if err != nil {
		return dashboard_page.DashboardData{}, err
	}
	h.cache = dashboardCache{
		data:      data,
		expiresAt: now.Add(dashboardCacheTTL),
	}
	return data, nil
}

func (h *DashboardHandler) collectDashboardData(ctx context.Context, now time.Time) (dashboard_page.DashboardData, error) {
	hosts, err := h.hostService.ListHosts(ctx, host.ListHostsFilter{})
	if err != nil {
		return dashboard_page.DashboardData{}, err
	}

	hostIDs := make([]uuid.UUID, 0, len(hosts))
	for _, listedHost := range hosts {
		hostIDs = append(hostIDs, listedHost.ID)
	}

	methodsByHostID, err := h.hostService.ListHostManagementMethodsByHostIDs(ctx, hostIDs)
	if err != nil {
		return dashboard_page.DashboardData{}, err
	}
	systemInfoByHostID, err := h.hostService.ListHostSystemInfosByHostIDs(ctx, hostIDs)
	if err != nil {
		return dashboard_page.DashboardData{}, err
	}
	latestSnapshotsByHostID, err := h.hostService.ListLatestHostMetricSnapshotsByHostIDs(ctx, hostIDs)
	if err != nil {
		return dashboard_page.DashboardData{}, err
	}

	rows := make([]dashboard_page.DashboardHostRow, 0, len(hosts))
	totalMethods := 0
	monitoredHosts := 0
	reportingHosts := 0
	staleHosts := 0
	systemInfoHosts := 0
	latestMetricTime := time.Time{}

	for _, listedHost := range hosts {
		methods := methodsByHostID[listedHost.ID]
		totalMethods += len(methods)
		if listedHost.MonitoringAgentID != uuid.Nil {
			monitoredHosts++
		}

		row := dashboard_page.DashboardHostRow{
			ID:                    listedHost.ID.String(),
			Name:                  listedHost.Name,
			IP:                    listedHost.IP.String(),
			Tags:                  listedHost.Tags,
			ManagementMethodCount: len(methods),
			HasMonitoringAgent:    listedHost.MonitoringAgentID != uuid.Nil,
			Status:                "waiting",
			StatusLabel:           "Waiting data",
			StatusVariant:         "neutral",
			LastMetricLabel:       "No metrics yet",
			CPUUsageLabel:         "n/a",
			MemoryUsageLabel:      "n/a",
			DiskUsageLabel:        "n/a",
			NetworkUsageLabel:     "n/a",
			SystemLabel:           "Unknown system",
		}

		if systemInfo, ok := systemInfoByHostID[listedHost.ID]; ok {
			systemInfoHosts++
			row.SystemLabel = formatSystemLabel(systemInfo)
		}

		if snapshot, ok := latestSnapshotsByHostID[listedHost.ID]; ok && len(snapshot.Metrics) > 0 {
			metric := snapshot.Metrics[0]
			row.LastMetricLabel = formatMetricAge(now, snapshot.Timestamp)
			row.CPUUsageLabel = fmt.Sprintf("%.1f%%", metric.CPUUsagePercentage)
			row.MemoryUsageLabel = formatUsageBytes(metric.MemoryUsageBytes, systemInfoByHostID[listedHost.ID].TotalMemoryBytes)
			row.DiskUsageLabel = formatUsageBytes(metric.DiskUsageBytes, systemInfoByHostID[listedHost.ID].TotalDiskBytes)
			row.NetworkUsageLabel = fmt.Sprintf("%s in / %s out", formatBytes(metric.NetworkRxBytes), formatBytes(metric.NetworkTxBytes))
			if latestMetricTime.IsZero() || snapshot.Timestamp.After(latestMetricTime) {
				latestMetricTime = snapshot.Timestamp
			}

			if now.Sub(snapshot.Timestamp) <= 2*time.Minute {
				row.Status = "reporting"
				row.StatusLabel = "Reporting"
				row.StatusVariant = "success"
				reportingHosts++
			} else {
				row.Status = "stale"
				row.StatusLabel = "Stale metrics"
				row.StatusVariant = "warning"
				staleHosts++
			}
		}

		rows = append(rows, row)
	}

	totalHosts := len(hosts)
	waitingHosts := totalHosts - reportingHosts - staleHosts
	data := dashboard_page.DashboardData{
		GeneratedAtLabel: now.Format("15:04:05"),
		Summary: dashboard_page.DashboardSummary{
			TotalHosts:          totalHosts,
			ReportingHosts:      reportingHosts,
			StaleHosts:          staleHosts,
			WaitingHosts:        waitingHosts,
			MonitoredHosts:      monitoredHosts,
			SystemInfoHosts:     systemInfoHosts,
			ManagementMethods:   totalMethods,
			LatestMetricAtLabel: "No metrics yet",
		},
		Hosts: rows,
	}
	if !latestMetricTime.IsZero() {
		data.Summary.LatestMetricAtLabel = formatMetricAge(now, latestMetricTime)
	}

	return data, nil
}

func (h *DashboardHandler) subscribeDashboardEvent(eventBus events.EventBus, event events.Event) {
	if err := eventBus.Subscribe(event, func(ctx context.Context, event events.Event) error {
		update := dashboardUpdateForEvent(event)
		if update.sections == 0 {
			h.logger.Debug("dashboard update skipped", slog.String("event", event.EventName()))
			return nil
		}
		h.invalidateCache()
		h.queueLiveUpdate(update)
		h.logger.Debug("dashboard update queued", slog.String("event", event.EventName()), slog.Uint64("sections", uint64(update.sections)), slog.Int("host_ids", len(update.hostIDs)))
		return nil
	}); err != nil {
		h.logger.Error("failed to subscribe dashboard updates", slog.String("event", event.EventName()), slog.String("error", err.Error()))
	}
}

func dashboardUpdateForEvent(event events.Event) dashboardUpdate {
	switch value := event.(type) {
	case host.CreateHostEvent,
		host.DeleteHostEvent:
		return dashboardUpdate{sections: dashboardUpdateAll}
	case host.UpdateHostEvent:
		return dashboardHostRowUpdate(dashboardUpdateGeneratedAt, value.Host.ID)
	case host.HostMetricSnapshotCreatedEvent:
		return dashboardHostRowUpdate(dashboardUpdateGeneratedAt|dashboardUpdateOverview, value.HostID)
	case host.HostSystemInfoUpdatedEvent:
		return dashboardHostRowUpdate(dashboardUpdateGeneratedAt|dashboardUpdateOverview, value.HostID)
	case host.HostMonitoringAgentAssignedEvent:
		return dashboardHostRowUpdate(dashboardUpdateGeneratedAt|dashboardUpdateOverview, value.HostID)
	case host.HostManagementMethodCreatedEvent:
		return dashboardHostRowUpdate(dashboardUpdateGeneratedAt|dashboardUpdateOverview, value.HostID)
	case host.HostPingCompletedEvent,
		host.HostsPingCompletedEvent,
		host.HostMetricReceivedEvent,
		agent.AgentRegisteredEvent,
		agent.AgentUpdatedEvent,
		agent.AgentDeletedEvent,
		agent.AgentLastSeenUpdatedEvent:
		return dashboardUpdate{}
	default:
		return dashboardUpdate{sections: dashboardUpdateAll}
	}
}

func dashboardHostRowUpdate(sections dashboardUpdateSections, hostIDs ...uuid.UUID) dashboardUpdate {
	update := dashboardUpdate{
		sections: sections | dashboardUpdateHostRows,
		hostIDs:  make(map[uuid.UUID]struct{}, len(hostIDs)),
	}
	for _, hostID := range hostIDs {
		if hostID == uuid.Nil {
			continue
		}
		update.hostIDs[hostID] = struct{}{}
	}
	if len(update.hostIDs) == 0 {
		return dashboardUpdate{sections: sections | dashboardUpdateHosts}
	}
	return update
}

func (h *DashboardHandler) invalidateCache() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache = dashboardCache{}
}

func (h *DashboardHandler) registerClient(client *dashboardClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[client] = struct{}{}
	h.logger.Debug("dashboard websocket client connected", slog.Int("clients", len(h.clients)))
}

func (h *DashboardHandler) unregisterClient(client *dashboardClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	delete(h.clients, client)
	h.logger.Debug("dashboard websocket client disconnected", slog.Int("clients", len(h.clients)))
}

func (h *DashboardHandler) queueLiveUpdate(update dashboardUpdate) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for client := range h.clients {
		h.signalClient(client, update)
	}
}

func (h *DashboardHandler) signalClient(client *dashboardClient, update dashboardUpdate) {
	select {
	case client.updateCh <- update:
	default:
		select {
		case pending := <-client.updateCh:
			merged := mergeDashboardUpdates(pending, update)
			select {
			case client.updateCh <- merged:
			default:
			}
		default:
		}
	}
}

func mergeDashboardUpdates(left dashboardUpdate, right dashboardUpdate) dashboardUpdate {
	merged := dashboardUpdate{
		sections: left.sections | right.sections,
		hostIDs:  make(map[uuid.UUID]struct{}, len(left.hostIDs)+len(right.hostIDs)),
	}
	for hostID := range left.hostIDs {
		merged.hostIDs[hostID] = struct{}{}
	}
	for hostID := range right.hostIDs {
		merged.hostIDs[hostID] = struct{}{}
	}
	return merged
}

func (h *DashboardHandler) renderLiveUpdate(ctx context.Context, update dashboardUpdate) ([]byte, error) {
	data, err := h.dashboardData(ctx)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	if update.sections&dashboardUpdateGeneratedAt != 0 {
		if err := dashboard_page.DashboardGeneratedAtLiveUpdate(data).Render(ctx, &body); err != nil {
			return nil, err
		}
	}
	if update.sections&dashboardUpdateOverview != 0 {
		if err := dashboard_page.DashboardOverviewLiveUpdate(data).Render(ctx, &body); err != nil {
			return nil, err
		}
	}
	if update.sections&dashboardUpdateHosts != 0 {
		if err := dashboard_page.DashboardHostsLiveUpdate(data).Render(ctx, &body); err != nil {
			return nil, err
		}
	} else if update.sections&dashboardUpdateHostRows != 0 {
		rowsByID := dashboardHostRowsByID(data.Hosts)
		for _, hostID := range sortedDashboardHostIDs(update.hostIDs) {
			row, ok := rowsByID[hostID]
			if !ok {
				continue
			}
			if err := dashboard_page.DashboardHostRowLiveUpdate(row).Render(ctx, &body); err != nil {
				return nil, err
			}
		}
	}
	if body.Len() == 0 {
		if err := dashboard_page.DashboardLiveUpdate(data).Render(ctx, &body); err != nil {
			return nil, err
		}
	}
	return body.Bytes(), nil
}

func dashboardHostRowsByID(rows []dashboard_page.DashboardHostRow) map[string]dashboard_page.DashboardHostRow {
	result := make(map[string]dashboard_page.DashboardHostRow, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		result[row.ID] = row
	}
	return result
}

func sortedDashboardHostIDs(hostIDs map[uuid.UUID]struct{}) []string {
	result := make([]string, 0, len(hostIDs))
	for hostID := range hostIDs {
		result = append(result, hostID.String())
	}
	sort.Strings(result)
	return result
}

func formatSystemLabel(info host.HostSystemInfo) string {
	if info.Hostname != "" && info.OS.Name != "" {
		return info.Hostname + " · " + info.OS.Name
	}
	if info.Hostname != "" {
		return info.Hostname
	}
	if info.OS.Name != "" {
		return info.OS.Name
	}
	return "System info received"
}

func formatMetricAge(now time.Time, value time.Time) string {
	if value.IsZero() {
		return "No metrics yet"
	}
	duration := now.Sub(value)
	if duration < time.Minute {
		return "just now"
	}
	if duration < time.Hour {
		return fmt.Sprintf("%d min ago", int(duration.Minutes()))
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%d h ago", int(duration.Hours()))
	}
	return value.Format("2006-01-02 15:04")
}

func formatUsageBytes(used uint64, total uint64) string {
	if total == 0 {
		return formatBytes(used)
	}
	return fmt.Sprintf("%s / %s", formatBytes(used), formatBytes(total))
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	suffix := [...]string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffix[exp])
}
