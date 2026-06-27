package agent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type Service struct {
	config *AgentConfig
	logger *slog.Logger
}

func NewService(config *AgentConfig, logger *slog.Logger) *Service {
	logger = logger.With(slog.String("component", "agent"))
	return &Service{config: config, logger: logger}
}

func (s *Service) GetMetrics(ctx context.Context) (host.HostMetric, error) {
	metric := host.HostMetric{}

	if usage, err := cpu.PercentWithContext(ctx, 0, false); err != nil {
		return metric, fmt.Errorf("failed to collect cpu metrics: %w", err)
	} else if len(usage) > 0 {
		metric.CPUUsagePercentage = usage[0]
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err != nil {
		return metric, fmt.Errorf("failed to collect memory metrics: %w", err)
	} else {
		metric.MemoryUsageBytes = vm.Used
	}

	if du, err := disk.UsageWithContext(ctx, "/"); err != nil {
		return metric, fmt.Errorf("failed to collect disk metrics: %w", err)
	} else {
		metric.DiskUsageBytes = du.Used
	}

	if ioCounters, err := net.IOCountersWithContext(ctx, false); err != nil {
		return metric, fmt.Errorf("failed to collect network metrics: %w", err)
	} else if len(ioCounters) > 0 {
		metric.NetworkRxBytes = ioCounters[0].BytesRecv
		metric.NetworkTxBytes = ioCounters[0].BytesSent
	}

	return metric, nil
}
