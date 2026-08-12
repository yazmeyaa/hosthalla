package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
	hostsqlite "github.com/yazmeyaa/hosthalla/internal/host/sqlite"
	"github.com/yazmeyaa/hosthalla/internal/testsqlite"
)

func TestHostAndManagementMethodRepositories(t *testing.T) {
	ctx := context.Background()
	db := testsqlite.Open(t)
	repositories := hostsqlite.NewRepositories(db)

	first, err := repositories.Host.CreateHost(ctx, host.CreateHostDTO{
		Name: "first", Description: "primary", IP: netip.MustParseAddr("192.0.2.10"), Tags: []string{"linux", "prod"},
	})
	must(t, err)
	second, err := repositories.Host.CreateHost(ctx, host.CreateHostDTO{
		Name: "second", IP: netip.MustParseAddr("192.0.2.11"), Tags: []string{"linux"},
	})
	must(t, err)
	if first.ID == uuid.Nil || !reflect.DeepEqual(first.Tags, []string{"linux", "prod"}) {
		t.Fatalf("unexpected created host: %+v", first)
	}

	byID, err := repositories.Host.GetHostByID(ctx, first.ID)
	must(t, err)
	if byID.ID != first.ID || byID.IP != first.IP {
		t.Fatal("host lookup returned another host")
	}
	all, err := repositories.Host.ListHosts(ctx, host.ListHostsFilter{})
	must(t, err)
	linux, err := repositories.Host.ListHosts(ctx, host.ListHostsFilter{Tags: []string{"linux"}})
	must(t, err)
	prodLinux, err := repositories.Host.ListHosts(ctx, host.ListHostsFilter{Tags: []string{"prod", "linux"}})
	must(t, err)
	if len(all) != 2 || len(linux) != 2 || len(prodLinux) != 1 || prodLinux[0].ID != first.ID {
		t.Fatalf("unexpected host filter counts: %d, %d, %d", len(all), len(linux), len(prodLinux))
	}
	tags, err := repositories.Host.ListTags(ctx)
	must(t, err)
	if len(tags) != 2 || tags[0].Name != "linux" || tags[1].Name != "prod" {
		t.Fatalf("unexpected tags: %+v", tags)
	}

	first.Name = "first-updated"
	first.IP = netip.MustParseAddr("192.0.2.12")
	first.Tags = []string{"staging"}
	must(t, repositories.Host.UpdateHost(ctx, &first))
	updated, err := repositories.Host.GetHostByID(ctx, first.ID)
	must(t, err)
	if updated.Name != first.Name || updated.IP != first.IP || !reflect.DeepEqual(updated.Tags, []string{"staging"}) {
		t.Fatalf("host was not updated: %+v", updated)
	}
	if _, err := repositories.Host.CreateHost(ctx, host.CreateHostDTO{Name: "duplicate", IP: second.IP}); err == nil {
		t.Fatal("expected unique host ip error")
	}

	method, err := repositories.HostManagementMethod.CreateHostManagementMethod(ctx, first.ID, host.CreateHostManagementMethodDTO{
		Name: "root ssh", Type: host.HostManagementMethodTypeSSHPassword, Username: "root", Port: 22, Secret: []byte("encrypted"), Description: "main",
	})
	must(t, err)
	byMethodID, err := repositories.HostManagementMethod.GetHostManagementMethodByID(ctx, method.ID)
	must(t, err)
	methods, err := repositories.HostManagementMethod.ListHostManagementMethods(ctx, first.ID)
	must(t, err)
	batch, err := repositories.HostManagementMethod.ListHostManagementMethodsByHostIDs(ctx, []uuid.UUID{first.ID, second.ID})
	must(t, err)
	emptyBatch, err := repositories.HostManagementMethod.ListHostManagementMethodsByHostIDs(ctx, nil)
	must(t, err)
	if byMethodID.ID != method.ID || len(methods) != 1 || len(batch[first.ID]) != 1 || len(emptyBatch) != 0 {
		t.Fatal("management method lookup/list failed")
	}
	method, err = repositories.HostManagementMethod.UpdateHostManagementMethod(ctx, method.ID, host.UpdateHostManagementMethodDTO{
		Name: "admin ssh", Username: "admin", Port: 2222, Secret: []byte("new-secret"), Description: "updated",
	})
	must(t, err)
	if method.Name != "admin ssh" || method.Port != 2222 || string(method.Secret) != "new-secret" {
		t.Fatalf("management method was not updated: %+v", method)
	}
	must(t, repositories.HostManagementMethod.DeleteHostManagementMethod(ctx, method.ID))
	if _, err := repositories.HostManagementMethod.GetHostManagementMethodByID(ctx, method.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted method error = %v", err)
	}

	if _, err := repositories.Host.GetHostByID(ctx, uuid.New()); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing host error = %v", err)
	}
	must(t, repositories.Host.DeleteHost(ctx, first.ID))
	if err := repositories.Host.DeleteHost(ctx, first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("second host delete error = %v", err)
	}
}

func TestHostSystemInfoRepository(t *testing.T) {
	ctx := context.Background()
	db := testsqlite.Open(t)
	repositories := hostsqlite.NewRepositories(db)
	first := createHost(t, ctx, repositories.Host, "first", "192.0.2.20")
	second := createHost(t, ctx, repositories.Host, "second", "192.0.2.21")

	info := host.HostSystemInfo{
		HostID: first.ID, Hostname: "node-1", OS: host.OSSystemInfo{Name: "Linux", Version: "1", Kernel: "6.0"},
		TotalMemoryBytes: 1024, CPU: host.CPUSystemInfo{Name: "CPU", Architecture: "amd64", Cores: 4, Frequency: 3200, Threads: 8},
		GPUs: []host.GPUSystemInfo{{Name: "GPU 1"}, {Name: "GPU 2"}}, TotalDiskBytes: 2048,
	}
	created, err := repositories.HostSystemInfo.UpsertHostSystemInfo(ctx, info)
	must(t, err)
	if !reflect.DeepEqual(created.GPUs, info.GPUs) {
		t.Fatalf("unexpected GPUs: %+v", created.GPUs)
	}
	byHost, err := repositories.HostSystemInfo.GetHostSystemInfoByHostID(ctx, first.ID)
	must(t, err)
	if byHost.Hostname != info.Hostname {
		t.Fatalf("system info lookup = %+v", byHost)
	}

	info.Hostname = "node-1-updated"
	info.GPUs = []host.GPUSystemInfo{{Name: "GPU 3"}}
	updated, err := repositories.HostSystemInfo.UpsertHostSystemInfo(ctx, info)
	must(t, err)
	if updated.Hostname != info.Hostname || !reflect.DeepEqual(updated.GPUs, info.GPUs) {
		t.Fatalf("system info was not replaced: %+v", updated)
	}
	batch, err := repositories.HostSystemInfo.ListHostSystemInfosByHostIDs(ctx, []uuid.UUID{first.ID, second.ID})
	must(t, err)
	empty, err := repositories.HostSystemInfo.ListHostSystemInfosByHostIDs(ctx, nil)
	must(t, err)
	if len(batch) != 1 || len(empty) != 0 || batch[first.ID].Hostname != info.Hostname {
		t.Fatalf("unexpected system info batch: %+v", batch)
	}
	if _, err := repositories.HostSystemInfo.GetHostSystemInfoByHostID(ctx, second.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing system info error = %v", err)
	}
}

func TestHostMetricSnapshotRepository(t *testing.T) {
	ctx := context.Background()
	db := testsqlite.Open(t)
	repositories := hostsqlite.NewRepositories(db)
	first := createHost(t, ctx, repositories.Host, "first", "192.0.2.30")
	second := createHost(t, ctx, repositories.Host, "second", "192.0.2.31")
	base := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	firstMetric := host.HostMetric{CPUUsagePercentage: 10, MemoryUsageBytes: 20, DiskUsageBytes: 30, NetworkRxBytes: 40, NetworkTxBytes: 50}
	created, err := repositories.HostMetricSnapshot.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{
		HostID: first.ID, Timestamp: base, Metrics: []host.HostMetric{firstMetric, {CPUUsagePercentage: 11, MemoryUsageBytes: 21, DiskUsageBytes: 31, NetworkRxBytes: 41, NetworkTxBytes: 51}},
	})
	must(t, err)
	if len(created.Metrics) != 2 || created.Metrics[0] != firstMetric {
		t.Fatalf("metric order was not preserved: %+v", created)
	}
	_, err = repositories.HostMetricSnapshot.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{HostID: first.ID, Timestamp: base.Add(time.Second), Metrics: []host.HostMetric{{CPUUsagePercentage: 12}}})
	must(t, err)
	_, err = repositories.HostMetricSnapshot.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{HostID: second.ID, Timestamp: base.Add(2 * time.Second), Metrics: []host.HostMetric{{CPUUsagePercentage: 99}}})
	must(t, err)

	all, err := repositories.HostMetricSnapshot.ListHostMetricSnapshots(ctx, first.ID)
	must(t, err)
	recent, err := repositories.HostMetricSnapshot.ListRecentHostMetricSnapshotsByHostIDs(ctx, []uuid.UUID{first.ID, second.ID}, 1)
	must(t, err)
	latest, err := repositories.HostMetricSnapshot.ListLatestHostMetricSnapshotsByHostIDs(ctx, []uuid.UUID{first.ID, second.ID})
	must(t, err)
	if len(all) != 2 || !all[0].Timestamp.Equal(base.Add(time.Second)) || len(recent[first.ID]) != 1 || len(recent[second.ID]) != 1 || latest[second.ID].Metrics[0].CPUUsagePercentage != 99 {
		t.Fatalf("unexpected metric queries: all=%+v recent=%+v latest=%+v", all, recent, latest)
	}
	emptyRecent, err := repositories.HostMetricSnapshot.ListRecentHostMetricSnapshotsByHostIDs(ctx, nil, 1)
	must(t, err)
	emptyLatest, err := repositories.HostMetricSnapshot.ListLatestHostMetricSnapshotsByHostIDs(ctx, nil)
	must(t, err)
	if len(emptyRecent) != 0 || len(emptyLatest) != 0 {
		t.Fatal("empty metric batch must be empty")
	}

	_, err = repositories.HostMetricSnapshot.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{
		HostID: first.ID, Timestamp: base.Add(3 * time.Second), Metrics: []host.HostMetric{{CPUUsagePercentage: 1}, {CPUUsagePercentage: -1}},
	})
	if err == nil {
		t.Fatal("expected invalid metric transaction to fail")
	}
	all, err = repositories.HostMetricSnapshot.ListHostMetricSnapshots(ctx, first.ID)
	must(t, err)
	if len(all) != 2 {
		t.Fatalf("failed transaction left a snapshot: %d", len(all))
	}

	for i := 0; i < 501; i++ {
		_, err := repositories.HostMetricSnapshot.CreateHostMetricSnapshot(ctx, host.HostMetricSnapshot{HostID: first.ID, Timestamp: base.Add(time.Duration(i+10) * time.Second)})
		must(t, err)
	}
	all, err = repositories.HostMetricSnapshot.ListHostMetricSnapshots(ctx, first.ID)
	must(t, err)
	if len(all) != 500 {
		t.Fatalf("retained snapshot count = %d", len(all))
	}

	must(t, repositories.Host.DeleteHost(ctx, second.ID))
	var count int
	must(t, db.QueryRowContext(ctx, `select count(*) from host_metric_snapshot where host_id = ?`, second.ID.String()).Scan(&count))
	if count != 0 {
		t.Fatalf("metric cascade count = %d", count)
	}
}

func createHost(t *testing.T, ctx context.Context, repository host.HostRepository, name, ip string) host.Host {
	t.Helper()
	value, err := repository.CreateHost(ctx, host.CreateHostDTO{Name: name, IP: netip.MustParseAddr(ip)})
	must(t, err)
	return value
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
