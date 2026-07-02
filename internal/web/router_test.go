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
