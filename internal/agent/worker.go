package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type Worker struct {
	config    *AgentConfig
	client    *Client
	collector MetricsCollector
	logger    *slog.Logger
}

func NewWorker(config *AgentConfig, collector MetricsCollector, logger *slog.Logger) *Worker {
	logger = logger.With(slog.String("component", "worker"))
	return &Worker{
		config:    config,
		client:    NewClient(config),
		collector: collector,
		logger:    logger,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.client.CloseIdleConnections()

	heartbeatTicker := time.NewTicker(w.config.Heartbeat.Interval)
	defer heartbeatTicker.Stop()

	metricsTicker := time.NewTicker(w.config.Metrics.Interval)
	defer metricsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-heartbeatTicker.C:
			if err := w.sendHeartbeat(ctx); err != nil {
				w.logger.Error("failed to send heartbeat", "error", err)
			}
		case <-metricsTicker.C:
			if err := w.sendMetrics(ctx); err != nil {
				w.logger.Error("failed to send metrics", "error", err)
			}
		}
	}
}

func (w *Worker) sendHeartbeat(ctx context.Context) error {
	_, err := w.client.SendHeartbeat(ctx)
	return err
}

func (w *Worker) sendMetrics(ctx context.Context) error {
	metric, err := w.collector.GetMetrics(ctx)
	if err != nil {
		return fmt.Errorf("collect metrics: %w", err)
	}
	if err := w.client.SendMetrics(ctx, metric); err != nil {
		return fmt.Errorf("send metrics request: %w", err)
	}
	return nil
}
