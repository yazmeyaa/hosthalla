package hosts_page_model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

func TestBuildHostLatestMetricsBadgesKeepsSortableMetricValues(t *testing.T) {
	systemInfo := &host.HostSystemInfo{
		HostID:           uuid.New(),
		TotalMemoryBytes: 100,
		TotalDiskBytes:   200,
	}
	metrics := BuildHostLatestMetricsBadges(host.HostMetric{
		CPUUsagePercentage: 86.2,
		MemoryUsageBytes:   90,
		DiskUsageBytes:     150,
	}, systemInfo)

	if metrics.CPU != "86.2%" {
		t.Fatalf("expected formatted CPU, got %q", metrics.CPU)
	}
	if metrics.CPUUsagePercentage != 86.2 {
		t.Fatalf("expected sortable CPU value, got %f", metrics.CPUUsagePercentage)
	}
	if metrics.MemoryUsagePercentage != 90 {
		t.Fatalf("expected memory percentage 90, got %f", metrics.MemoryUsagePercentage)
	}
	if metrics.DiskUsagePercentage != 75 {
		t.Fatalf("expected disk percentage 75, got %f", metrics.DiskUsagePercentage)
	}
}
