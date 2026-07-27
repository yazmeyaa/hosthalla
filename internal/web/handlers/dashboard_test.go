package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
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
						StatusLabel:   "No agent",
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
	assertWebSocketPayloadTopLevelTargets(t, body)
	if !strings.Contains(body, "<style") {
		t.Fatalf("expected nested templ styles to remain in websocket payload, got: %s", body)
	}
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

func TestRenderLiveUpdateOmitsTemplStylesFromFullUpdate(t *testing.T) {
	hostID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	handler := &DashboardHandler{
		cache: dashboardCache{
			expiresAt: time.Now().Add(time.Minute),
			data: dashboard_page.DashboardData{
				GeneratedAtLabel: "12:00:00",
				Summary: dashboard_page.DashboardSummary{
					TotalHosts:          1,
					ReportingHosts:      1,
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
				},
			},
		},
	}

	payload, err := handler.renderLiveUpdate(context.Background(), dashboardUpdate{sections: dashboardUpdateAll})
	if err != nil {
		t.Fatalf("render live update: %v", err)
	}

	body := string(payload)
	assertWebSocketPayloadTopLevelTargets(t, body)
	if !strings.Contains(body, "<style") {
		t.Fatalf("expected nested templ styles to remain in websocket payload, got: %s", body)
	}
	if !strings.Contains(body, `id="dashboard-generated-at"`) {
		t.Fatalf("expected generated-at fragment, got: %s", body)
	}
	if !strings.Contains(body, `id="dashboard-overview"`) {
		t.Fatalf("expected overview fragment, got: %s", body)
	}
	if !strings.Contains(body, `id="dashboard-hosts"`) {
		t.Fatalf("expected hosts fragment, got: %s", body)
	}
}

func assertWebSocketPayloadTopLevelTargets(t *testing.T, body string) {
	t.Helper()

	nodes, err := html.ParseFragment(strings.NewReader(body), &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Body,
		Data:     "body",
	})
	if err != nil {
		t.Fatalf("parse websocket payload: %v", err)
	}

	for _, node := range nodes {
		if node.Type != html.ElementNode {
			continue
		}
		if !hasHTMLAttr(node, "id") {
			t.Fatalf("expected every top-level websocket element to have an id for htmx OOB swap, got <%s>: %s", node.Data, body)
		}
	}
}

func hasHTMLAttr(node *html.Node, name string) bool {
	for _, attr := range node.Attr {
		if attr.Key == name && attr.Val != "" {
			return true
		}
	}
	return false
}
