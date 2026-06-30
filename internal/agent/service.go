package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/yazmeyaa/hosthalla/internal/events"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type Service struct {
	agentRepository       Repository
	agentConfigRepository AgentConfigRepository
	eventBus              events.EventBus
	logger                *slog.Logger
}

type ArgusService struct {
	config *AgentConfig
	logger *slog.Logger
}

func NewArgusService(config *AgentConfig, logger *slog.Logger) *ArgusService {
	return &ArgusService{
		config: config,
		logger: logger.With(slog.String("component", "argus-agent")),
	}
}

type NewServiceParams struct {
	AgentRepository       Repository
	AgentConfigRepository AgentConfigRepository
	EventBus              events.EventBus
	Logger                *slog.Logger
}

func NewService(params NewServiceParams) *Service {
	eventBus := params.EventBus
	if eventBus == nil {
		eventBus = events.NewInMemoryEventBus()
	}

	return &Service{
		agentRepository:       params.AgentRepository,
		agentConfigRepository: params.AgentConfigRepository,
		eventBus:              eventBus,
		logger:                params.Logger.With(slog.String("component", "agent")),
	}
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Agent, error) {
	agent, err := s.agentRepository.GetByID(ctx, id)
	if err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) GetAgentByHostID(ctx context.Context, hostID uuid.UUID) (Agent, error) {
	agent, err := s.agentRepository.GetByHostID(ctx, hostID)
	if err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) CreateAgent(ctx context.Context, agent CreateAgentDTO) (Agent, error) {
	createdAgent, err := s.agentRepository.Create(ctx, agent)
	if err != nil {
		return Agent{}, err
	}
	s.logger.Info("agent created", slog.String("agent_id", createdAgent.ID.String()), slog.String("host_id", createdAgent.HostID.String()))
	if err := s.eventBus.Publish(ctx, AgentRegisteredEvent{AgentID: createdAgent.ID, HostID: createdAgent.HostID}); err != nil {
		s.logger.Error("failed to publish agent registered event", slog.String("agent_id", createdAgent.ID.String()), slog.String("error", err.Error()))
	}
	return createdAgent, nil
}

func (s *Service) UpdateAgent(ctx context.Context, target *Agent) error {
	err := s.agentRepository.Update(ctx, target)
	if err != nil {
		return err
	}
	s.logger.Info("agent updated", slog.String("agent_id", target.ID.String()), slog.String("host_id", target.HostID.String()))
	if err := s.eventBus.Publish(ctx, AgentUpdatedEvent{Agent: *target}); err != nil {
		s.logger.Error("failed to publish agent updated event", slog.String("agent_id", target.ID.String()), slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) UpdateLastSeenAt(ctx context.Context, currentAgent Agent, lastSeenAt time.Time) error {
	err := s.agentRepository.UpdateLastSeenAt(ctx, currentAgent.ID, lastSeenAt)
	if err != nil {
		return err
	}
	if err := s.eventBus.Publish(ctx, AgentLastSeenUpdatedEvent{AgentID: currentAgent.ID, HostID: currentAgent.HostID}); err != nil {
		s.logger.Error("failed to publish agent last seen updated event", slog.String("agent_id", currentAgent.ID.String()), slog.String("error", err.Error()))
	}
	return nil
}

func (s *Service) RegisterHostAgent(ctx context.Context, hostID uuid.UUID, version string) (Agent, error) {
	currentAgent, err := s.GetAgentByHostID(ctx, hostID)
	if err == nil {
		currentAgent.Version = version
		if err := s.UpdateAgent(ctx, &currentAgent); err != nil {
			return Agent{}, err
		}
		return currentAgent, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, err
	}

	return s.CreateAgent(ctx, CreateAgentDTO{
		HostID:  hostID,
		Version: version,
	})
}

func (s *Service) GetConfigByAgentID(ctx context.Context, agentID uuid.UUID) (AgentConfig, error) {
	config, err := s.agentConfigRepository.GetByAgentID(ctx, agentID)
	if err != nil {
		return AgentConfig{}, err
	}
	return config, nil
}

func (s *ArgusService) GetMetrics(ctx context.Context) (host.HostMetric, error) {
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
