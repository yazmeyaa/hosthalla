package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"
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

func NewDashboardHandler(params DashboardHandlerParams) *DashboardHandler {
	h := &DashboardHandler{
		logger:         params.Logger.With("component", "dashboard_handler"),
		hostService:    params.HostService,
		profileService: params.ProfileService,
	}

	if params.EventBus != nil {
		h.subscribeInvalidation(params.EventBus, host.CreateHostEvent{})
		h.subscribeInvalidation(params.EventBus, host.UpdateHostEvent{})
		h.subscribeInvalidation(params.EventBus, host.DeleteHostEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostMetricReceivedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostMonitoringAgentAssignedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostManagementMethodCreatedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostPingCompletedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostsPingCompletedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostSystemInfoUpdatedEvent{})
		h.subscribeInvalidation(params.EventBus, host.HostMetricSnapshotCreatedEvent{})
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

func (h *DashboardHandler) subscribeInvalidation(eventBus events.EventBus, event events.Event) {
	if err := eventBus.Subscribe(event, func(ctx context.Context, event events.Event) error {
		h.invalidateCache()
		h.logger.Debug("dashboard cache invalidated", slog.String("event", event.EventName()))
		return nil
	}); err != nil {
		h.logger.Error("failed to subscribe dashboard cache invalidation", slog.String("event", event.EventName()), slog.String("error", err.Error()))
	}
}

func (h *DashboardHandler) invalidateCache() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache = dashboardCache{}
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
