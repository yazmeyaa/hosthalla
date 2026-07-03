package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
)

func TestRenderLiveUpdateCanRenderSingleHostRow(t *testing.T) {
	hostID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	otherHostID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	handler := &DashboardHandler{
		cache: dashboardCache{
			expiresAt: time.Now().Add(time.Minute),
			data: dashboard_page.DashboardData{
				GeneratedAtLabel: "12:00:00",
				Summary: dashboard_page.DashboardSummary{
					TotalHosts:          2,
					ReportingHosts:      1,
					WaitingHosts:        1,
					LatestMetricAtLabel: "just now",
				},
				Hosts: []dashboard_page.DashboardHostRow{
					{
						ID:                hostID.String(),
						Name:              "agent1host1",
						IP:                "10.0.0.10",
						StatusLabel:       "Reporting",
						StatusVariant:     "success",
						LastMetricLabel:   "just now",
						CPUUsageLabel:     "9.1%",
						MemoryUsageLabel:  "1.0 GB / 4.0 GB",
						DiskUsageLabel:    "8.0 GB / 40.0 GB",
						NetworkUsageLabel: "1.0 KB in / 2.0 KB out",
						SystemLabel:       "linux",
					},
					{
						ID:            otherHostID.String(),
						Name:          "quiet-host",
						IP:            "10.0.0.20",
						StatusLabel:   "Waiting data",
						StatusVariant: "neutral",
						SystemLabel:   "Unknown system",
					},
				},
			},
		},
	}

	payload, err := handler.renderLiveUpdate(context.Background(), dashboardHostRowUpdate(
		dashboardUpdateGeneratedAt|dashboardUpdateOverview,
		hostID,
	))
	if err != nil {
		t.Fatalf("render live update: %v", err)
	}

	body := string(payload)
	if !strings.Contains(body, `id="dashboard-generated-at"`) {
		t.Fatalf("expected generated-at fragment, got: %s", body)
	}
	if !strings.Contains(body, `id="dashboard-overview"`) {
		t.Fatalf("expected overview fragment, got: %s", body)
	}
	if strings.Contains(body, `id="dashboard-hosts"`) {
		t.Fatalf("unexpected full hosts fragment, got: %s", body)
	}
	if !strings.Contains(body, `id="dashboard-host-`+hostID.String()+`"`) {
		t.Fatalf("expected target host row fragment, got: %s", body)
	}
	if strings.Contains(body, `id="dashboard-host-`+otherHostID.String()+`"`) {
		t.Fatalf("unexpected unrelated host row fragment, got: %s", body)
	}
	if !strings.Contains(body, `href="#ui-icon-eye-closed"`) {
		t.Fatalf("expected reusable eye icon reference, got: %s", body)
	}
	if strings.Contains(body, "<path") {
		t.Fatalf("unexpected inline svg path in live update, got: %s", body)
	}
}
