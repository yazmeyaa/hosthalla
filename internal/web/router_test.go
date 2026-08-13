package web

import (
	"bytes"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	dashboard_page "github.com/yazmeyaa/hosthalla/ui/pages/dashboard"
)

func TestRouterServesRobotsTxt(t *testing.T) {
	handler := NewRouter(NewRouterParams{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "User-agent: TelegramBot\nAllow: /") ||
		!strings.Contains(response.Body.String(), "User-agent: Twitterbot\nAllow: /") ||
		!strings.Contains(response.Body.String(), "User-agent: *\nDisallow: /") {
		t.Fatalf("unexpected robots.txt: %q", response.Body.String())
	}
}

func TestRouterServesLLMsTxt(t *testing.T) {
	handler := NewRouter(NewRouterParams{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/llms.txt", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	for _, expected := range []string{
		"# Hosthalla",
		"[Dashboard](/dashboard)",
		"[Hosts](/hosts)",
		"[Administration](/administration)",
		"[Help](/help/install-server)",
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("llms.txt is missing %q", expected)
		}
	}
}

func TestRouterServesFavicon(t *testing.T) {
	handler := NewRouter(NewRouterParams{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/assets/favicon.svg", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Fatalf("unexpected favicon: %q", response.Body.String())
	}
}

func TestRouterServesOpenGraphImage(t *testing.T) {
	handler := NewRouter(NewRouterParams{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	request := httptest.NewRequest(http.MethodGet, "/assets/static/og-preview.png", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/png") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	imageConfig, err := png.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode Open Graph image: %v", err)
	}
	if imageConfig.Width != 1200 || imageConfig.Height != 630 {
		t.Fatalf("unexpected Open Graph image dimensions: %dx%d", imageConfig.Width, imageConfig.Height)
	}
}

func TestAuthPageHasAbsoluteSocialMetadata(t *testing.T) {
	handler := NewRouter(NewRouterParams{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		WebOrigin: "https://hosthalla.example.com",
	})

	request := httptest.NewRequest(http.MethodGet, "/auth", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d, want %d", response.Code, http.StatusOK)
	}
	for _, expected := range []string{
		`<html lang="en" prefix="og: https://ogp.me/ns#">`,
		`<meta property="og:url" content="https://hosthalla.example.com/auth">`,
		`<meta property="og:image" content="https://hosthalla.example.com/assets/static/og-preview.png">`,
		`<meta property="og:image:width" content="1200">`,
		`<meta property="og:image:height" content="630">`,
		`<meta name="twitter:card" content="summary_large_image">`,
	} {
		if !strings.Contains(response.Body.String(), expected) {
			t.Fatalf("auth page is missing %q", expected)
		}
	}
}

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
