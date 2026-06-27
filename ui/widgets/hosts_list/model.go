package hosts_list

import (
	"fmt"

	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostLatestMetricsBadges struct {
	CPU         string
	MemoryUsed  string
	MemoryTotal string
	Disk        string
}

func BuildHostLatestMetricsBadges(metric host.HostMetric, systemInfo *host.HostSystemInfo) HostLatestMetricsBadges {
	memoryTotal := ""
	if systemInfo != nil && systemInfo.TotalMemoryBytes > 0 {
		memoryTotal = formatBytes(systemInfo.TotalMemoryBytes)
	}
	return HostLatestMetricsBadges{
		CPU:         fmt.Sprintf("%.1f%%", metric.CPUUsagePercentage),
		MemoryUsed:  formatBytes(metric.MemoryUsageBytes),
		MemoryTotal: memoryTotal,
		Disk:        formatBytes(metric.DiskUsageBytes),
	}
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	suffix := [...]string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffix[exp])
}
