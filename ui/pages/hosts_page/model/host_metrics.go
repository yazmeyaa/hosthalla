package hosts_page_model

import (
	"fmt"

	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostLatestMetricsBadges struct {
	CPU                   string
	CPUUsagePercentage    float64
	MemoryUsed            string
	MemoryTotal           string
	MemoryUsageBytes      uint64
	MemoryTotalBytes      uint64
	Disk                  string
	DiskUsageBytes        uint64
	DiskTotalBytes        uint64
	MemoryUsagePercentage float64
	DiskUsagePercentage   float64
}

func BuildHostLatestMetricsBadges(metric host.HostMetric, systemInfo *host.HostSystemInfo) HostLatestMetricsBadges {
	memoryTotal := ""
	var memoryTotalBytes uint64
	var diskTotalBytes uint64
	if systemInfo != nil && systemInfo.TotalMemoryBytes > 0 {
		memoryTotal = formatBytes(systemInfo.TotalMemoryBytes)
		memoryTotalBytes = systemInfo.TotalMemoryBytes
	}
	if systemInfo != nil && systemInfo.TotalDiskBytes > 0 {
		diskTotalBytes = systemInfo.TotalDiskBytes
	}
	memoryUsagePercentage := percentage(metric.MemoryUsageBytes, memoryTotalBytes)
	diskUsagePercentage := percentage(metric.DiskUsageBytes, diskTotalBytes)
	return HostLatestMetricsBadges{
		CPU:                   fmt.Sprintf("%.1f%%", metric.CPUUsagePercentage),
		CPUUsagePercentage:    metric.CPUUsagePercentage,
		MemoryUsed:            formatBytes(metric.MemoryUsageBytes),
		MemoryTotal:           memoryTotal,
		MemoryUsageBytes:      metric.MemoryUsageBytes,
		MemoryTotalBytes:      memoryTotalBytes,
		Disk:                  formatBytes(metric.DiskUsageBytes),
		DiskUsageBytes:        metric.DiskUsageBytes,
		DiskTotalBytes:        diskTotalBytes,
		MemoryUsagePercentage: memoryUsagePercentage,
		DiskUsagePercentage:   diskUsagePercentage,
	}
}

func percentage(used uint64, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
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
