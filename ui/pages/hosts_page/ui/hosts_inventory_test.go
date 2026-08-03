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

func TestAddHostManagementMethodDialogStartsWithPasswordFieldsOnly(t *testing.T) {
	hostID := uuid.New()

	var body bytes.Buffer
	err := AddHostManagementMethodDialog(host.Host{ID: hostID}, "add-method-dialog", false).Render(t.Context(), &body)
	if err != nil {
		t.Fatalf("render add management method dialog: %v", err)
	}
	html := body.String()

	for _, expected := range []string{
		`name="methodType"`,
		`hx-get="/hosts/` + hostID.String() + `/management-method-fields"`,
		`hx-target="#management-method-auth-fields-` + hostID.String() + `"`,
		`name="password"`,
		`required`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected add management method dialog to contain %q", expected)
		}
	}
	for _, unexpected := range []string{`name="publicKey"`, `name="privateKey"`, `disabled`} {
		if strings.Contains(html, unexpected) {
			t.Fatalf("expected initial add management method dialog not to contain %q", unexpected)
		}
	}
}

func TestAddHostManagementMethodFieldsSwitchesByMethodType(t *testing.T) {
	var passwordFields bytes.Buffer
	err := AddHostManagementMethodFields("host-1", "ssh_password").Render(t.Context(), &passwordFields)
	if err != nil {
		t.Fatalf("render password fields: %v", err)
	}
	passwordHTML := passwordFields.String()
	if !strings.Contains(passwordHTML, `name="password"`) || strings.Contains(passwordHTML, `name="publicKey"`) || strings.Contains(passwordHTML, `name="privateKey"`) {
		t.Fatalf("expected password fields only, got %s", passwordHTML)
	}

	var keyFields bytes.Buffer
	err = AddHostManagementMethodFields("host-1", "ssh_key").Render(t.Context(), &keyFields)
	if err != nil {
		t.Fatalf("render key fields: %v", err)
	}
	keyHTML := keyFields.String()
	if !strings.Contains(keyHTML, `name="publicKey"`) || !strings.Contains(keyHTML, `name="privateKey"`) || strings.Contains(keyHTML, `name="password"`) {
		t.Fatalf("expected key fields only, got %s", keyHTML)
	}

	var passwordFieldsAgain bytes.Buffer
	err = AddHostManagementMethodFields("host-1", "ssh_password").Render(t.Context(), &passwordFieldsAgain)
	if err != nil {
		t.Fatalf("render password fields again: %v", err)
	}
	passwordAgainHTML := passwordFieldsAgain.String()
	if !strings.Contains(passwordAgainHTML, `name="password"`) || strings.Contains(passwordAgainHTML, `name="publicKey"`) || strings.Contains(passwordAgainHTML, `name="privateKey"`) {
		t.Fatalf("expected password fields after switching back, got %s", passwordAgainHTML)
	}
}

func TestHostManagementMethodDetailsDialogRendersDeleteAction(t *testing.T) {
	methodID := uuid.New()
	hostID := uuid.New()
	var body bytes.Buffer
	method := host.HostManagementMethod{
		ID:       methodID,
		HostID:   hostID,
		Name:     "Primary SSH",
		Type:     host.HostManagementMethodTypeSSHPassword,
		Username: "root",
		Port:     22,
	}
	err := HostManagementMethodDetailsDialog("192.0.2.10", method, "method-details-dialog", "update-method-dialog", true).Render(t.Context(), &body)
	if err != nil {
		t.Fatalf("render management method details dialog: %v", err)
	}
	html := body.String()
	for _, expected := range []string{
		`data-delete-management-method-form`,
		`data-delete-management-method-button`,
		`action="/hosts/` + hostID.String() + `/management-methods/` + methodID.String() + `/delete"`,
		`hx-post="/hosts/` + hostID.String() + `/management-methods/` + methodID.String() + `/delete"`,
		`hx-confirm="Delete this authorization method?"`,
		`aria-label="Delete Primary SSH"`,
		`href="#ui-icon-trash"`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected management method details dialog to contain %q", expected)
		}
	}

	body.Reset()
	err = UpdateHostManagementMethodDialog(method, "update-method-dialog", true).Render(t.Context(), &body)
	if err != nil {
		t.Fatalf("render update management method dialog: %v", err)
	}
	if strings.Contains(body.String(), `data-delete-management-method-form`) {
		t.Fatal("expected update management method dialog not to contain delete form")
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
