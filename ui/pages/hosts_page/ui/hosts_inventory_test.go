package hosts_page_ui

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
	hosts_page_model "github.com/yazmeyaa/hosthalla/ui/pages/hosts_page/model"
)

func TestHostsInventoryRendersFilteringSortingAndActions(t *testing.T) {
	hostID := uuid.New()
	listedHost := host.Host{
		ID:          hostID,
		Name:        "db-primary",
		Description: "Postgres primary",
		Tags:        []string{"database", "prod", "critical", "eu"},
		IP:          netip.MustParseAddr("10.10.0.8"),
		CreatedAt:   time.Now().Add(-48 * time.Hour),
		UpdatedAt:   time.Now().Add(-2 * time.Minute),
	}
	metrics := hosts_page_model.HostLatestMetricsBadges{
		CPU:                   "91.0%",
		CPUUsagePercentage:    91,
		MemoryUsed:            "9.0 GB",
		MemoryTotal:           "10.0 GB",
		MemoryUsagePercentage: 90,
		Disk:                  "20.0 GB",
		DiskUsagePercentage:   50,
	}

	var body bytes.Buffer
	err := HostsInventory(HostsPageProps{
		Hosts:                      []host.Host{listedHost},
		TotalHosts:                 1,
		AvailableTags:              []host.Tag{{Name: "database"}, {Name: "prod"}},
		AvailableManagementFilters: []string{string(host.HostManagementMethodTypeSSHPassword)},
		SearchQuery:                "db",
		SelectedTags:               []string{"database"},
		SelectedStatuses:           []string{"warning"},
		SelectedManagementFilters:  []string{string(host.HostManagementMethodTypeSSHPassword)},
		SortKey:                    "name",
		SortDirection:              "asc",
		HostManagementMethodsByHostID: map[string][]host.HostManagementMethod{
			hostID.String(): {{Type: host.HostManagementMethodTypeSSHPassword}},
		},
		HostSystemInfoByHostID:    map[string]host.HostSystemInfo{},
		HostLatestMetricsByHostID: map[string]hosts_page_model.HostLatestMetricsBadges{hostID.String(): metrics},
	}).Render(t.Context(), &body)
	if err != nil {
		t.Fatalf("render hosts inventory: %v", err)
	}
	html := body.String()

	for _, expected := range []string{
		"data-hosts-search",
		`method="get"`,
		`name="q"`,
		`value="db"`,
		`data-hosts-multi-filter="status"`,
		`data-hosts-multi-filter="tag"`,
		`data-hosts-multi-filter="management"`,
		`name="status"`,
		`name="tag"`,
		`name="management"`,
		`href="/hosts?direction=desc`,
		`sort=name`,
		"data-hosts-bulk-toolbar",
		"data-hosts-ping-selected",
		"data-hosts-export-selected",
		"data-host-ping-form",
		`id="host-ping-result-card-` + hostID.String(),
		`hx-target="#host-ping-result-card-` + hostID.String(),
		`data-host-status="warning"`,
		"+2",
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected rendered inventory to contain %q", expected)
		}
	}
}

func TestBuildHostInventoryRowsProvidesSortAndFilterData(t *testing.T) {
	hostID := uuid.New()
	now := time.Now()
	rows := buildHostInventoryRows(
		[]host.Host{{
			ID:          hostID,
			Name:        "edge-01",
			Description: "front door",
			Tags:        []string{"edge"},
			IP:          netip.MustParseAddr("10.0.0.1"),
			CreatedAt:   now.Add(-2 * time.Hour),
			UpdatedAt:   now.Add(-30 * time.Minute),
		}},
		map[string][]host.HostManagementMethod{
			hostID.String(): {{Type: host.HostManagementMethodTypeSSHKey}},
		},
		map[string]host.HostSystemInfo{},
		map[string]hosts_page_model.HostLatestMetricsBadges{},
	)

	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	row := rows[0]
	if row.Status != "waiting" {
		t.Fatalf("expected waiting status without metrics but with management method, got %q", row.Status)
	}
	if !strings.Contains(row.SearchText, "front door") || !strings.Contains(row.TagsText, "edge") {
		t.Fatalf("expected search/filter text to include host description and tags, got search=%q tags=%q", row.SearchText, row.TagsText)
	}
	if row.ManagementFilter != string(host.HostManagementMethodTypeSSHKey) {
		t.Fatalf("expected management filter ssh key, got %q", row.ManagementFilter)
	}
}
