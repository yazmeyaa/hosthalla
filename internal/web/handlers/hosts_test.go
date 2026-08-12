package handlers

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/ui/pages/hosts_page"
)

func TestParseHostsPageQueryKeepsMultiFilterState(t *testing.T) {
	request := httptest.NewRequest("GET", "/hosts?q=db&tag=prod&tag=db&status=reporting&status=warning&management=agent&management=ssh_key&sort=cpu&direction=asc", nil)

	query := parseHostsPageQuery(request)

	if query.Search != "db" {
		t.Fatalf("expected search query, got %q", query.Search)
	}
	if got := len(query.Tags); got != 2 {
		t.Fatalf("expected two tags, got %d", got)
	}
	if got := len(query.Statuses); got != 2 {
		t.Fatalf("expected two statuses, got %d", got)
	}
	if got := len(query.ManagementFilters); got != 2 {
		t.Fatalf("expected two management filters, got %d", got)
	}
	if query.SortKey != "cpu" || query.SortDirection != "asc" {
		t.Fatalf("expected cpu asc sort, got %s %s", query.SortKey, query.SortDirection)
	}
}

func TestHostsOpenDialogIDFromRoutes(t *testing.T) {
	hostID := uuid.New().String()
	methodID := uuid.New().String()
	tests := []struct {
		path   string
		host   string
		method string
		want   string
	}{
		{path: "/hosts", want: ""},
		{path: "/hosts/create", want: "create-host-dialog"},
		{path: "/hosts/" + hostID, host: hostID, want: "host-details-dialog-" + hostID},
		{path: "/hosts/" + hostID + "/update", host: hostID, want: "update-host-dialog-" + hostID},
		{path: "/hosts/" + hostID + "/methods/create", host: hostID, want: "add-host-management-method-dialog-" + hostID},
		{path: "/hosts/" + hostID + "/methods/" + methodID, host: hostID, method: methodID, want: "host-management-method-details-" + methodID},
		{path: "/hosts/" + hostID + "/methods/" + methodID + "/update", host: hostID, method: methodID, want: "update-host-management-method-" + methodID},
	}

	for _, tt := range tests {
		request := httptest.NewRequest("GET", tt.path, nil)
		if tt.host != "" {
			request.SetPathValue("id", tt.host)
		}
		if tt.method != "" {
			request.SetPathValue("methodID", tt.method)
		}
		if got := hostsOpenDialogID(request); got != tt.want {
			t.Fatalf("expected %q for %s, got %q", tt.want, tt.path, got)
		}
	}
}

func TestFilterAndSortHostsForHostsPageUsesServerSideDerivedData(t *testing.T) {
	now := time.Now()
	firstID := uuid.New()
	secondID := uuid.New()
	hosts := []host.Host{
		{
			ID:        firstID,
			Name:      "db-low",
			Tags:      []string{"prod"},
			IP:        netip.MustParseAddr("10.0.0.10"),
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now.Add(-10 * time.Minute),
		},
		{
			ID:                secondID,
			Name:              "db-hot",
			Tags:              []string{"prod"},
			IP:                netip.MustParseAddr("10.0.0.20"),
			MonitoringAgentID: uuid.New(),
			CreatedAt:         now.Add(-1 * time.Hour),
			UpdatedAt:         now.Add(-5 * time.Minute),
		},
	}
	methods := map[string][]host.HostManagementMethod{
		firstID.String(): {{Type: host.HostManagementMethodTypeSSHKey}},
	}
	metrics := map[string]hosts_page.HostLatestMetricsBadges{
		firstID.String():  {CPUUsagePercentage: 10},
		secondID.String(): {CPUUsagePercentage: 95},
	}
	query := hostsPageQuery{
		Search:            "db",
		Statuses:          []string{"warning", "reporting"},
		ManagementFilters: []string{"agent", "ssh_key"},
		SortKey:           "cpu",
		SortDirection:     "desc",
	}

	filtered := filterHostsForHostsPage(hosts, query, methods, nil, metrics)
	sortHostsForHostsPage(filtered, query, methods, nil, metrics)

	if len(filtered) != 2 {
		t.Fatalf("expected both hosts to match OR filters, got %d", len(filtered))
	}
	if filtered[0].Name != "db-hot" {
		t.Fatalf("expected highest CPU host first, got %q", filtered[0].Name)
	}
}
