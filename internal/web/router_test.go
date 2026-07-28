package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
)

func TestCSSMiddlewareSuppressesDashboardFragmentStyles(t *testing.T) {
	handler := templ.NewCSSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := dashboard_page.DashboardLiveUpdate(dashboard_page.DashboardData{
			GeneratedAtLabel: "12:00:00",
		}).Render(r.Context(), w)
		if err != nil {
			t.Fatalf("render dashboard fragment: %v", err)
		}
	}), cssClasses()...)

	request := httptest.NewRequest(http.MethodGet, "/dashboard/subscribe-test", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if strings.Contains(response.Body.String(), "<style") {
		t.Fatalf("dashboard fragment unexpectedly rendered inline styles: %s", response.Body.String())
	}
}

func TestDashboardCSSClassesIncludeSparklineStyles(t *testing.T) {
	handler := templ.NewCSSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := dashboard_page.DashboardPage(dashboard_page.DashboardPageProps{
			Data: dashboard_page.DashboardData{
				GeneratedAtLabel: "12:00:00",
				Hosts: []dashboard_page.DashboardHostRow{
					{
						ID:                "11111111-1111-1111-1111-111111111111",
						Name:              "agent1host1",
						IP:                "10.0.0.10",
						StatusLabel:       "Reporting",
						StatusVariant:     "success",
						LastMetricLabel:   "just now",
						CPUUsageLabel:     "9.1%",
						CPUUsageSeries:    []float64{1, 2, 3},
						MemoryUsageLabel:  "1.0 GB / 4.0 GB",
						MemoryUsageSeries: []float64{25, 50, 75},
						DiskUsageLabel:    "8.0 GB / 40.0 GB",
						NetworkUsageLabel: "1.0 KB in / 2.0 KB out",
						SystemLabel:       "linux",
					},
				},
			},
		}).Render(r.Context(), w)
		if err != nil {
			t.Fatalf("render dashboard page: %v", err)
		}
	}), cssClasses()...)

	request := httptest.NewRequest(http.MethodGet, "/styles/templ.css", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	body := response.Body.String()
	for _, expected := range []string{
		`height:28px`,
		`grid-template-columns:repeat(auto-fit, minmax(118px, 1fr))`,
		`stroke:currentColor`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("dashboard CSS is missing %q: %s", expected, body)
		}
	}
}
