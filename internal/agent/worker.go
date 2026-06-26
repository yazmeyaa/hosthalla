package agent

import (
	"context"
	"fmt"
	"time"
)

func RunWorker(ctx context.Context, config *AgentConfig) {
	heartbeatTicker := time.NewTicker(config.Heartbeat.Interval)
	defer heartbeatTicker.Stop()

	metricsTicker := time.NewTicker(config.Metrics.Interval)
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeatTicker.C:
			sendHeartbeat(ctx, config)
		case <-metricsTicker.C:
			sendMetrics(ctx, config)
		}
	}
}

func sendHeartbeat(ctx context.Context, config *AgentConfig) {
	fmt.Println("Sending heartbeat")
}

func sendMetrics(ctx context.Context, config *AgentConfig) {
	fmt.Println("Sending metrics")
}
