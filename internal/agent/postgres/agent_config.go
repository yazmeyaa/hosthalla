package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/agent"
)

const (
	agentConfigSelectColumns = "id, agent_id, heartbeat_interval_seconds, metrics_interval_seconds, version"

	insertAgentConfigQuery = `
		insert into agent_config (agent_id, heartbeat_interval_seconds, metrics_interval_seconds, version)
		values ($1, $2, $3, $4)
		returning ` + agentConfigSelectColumns
	getAgentConfigByAgentIDQuery = `
		select ` + agentConfigSelectColumns + `
		from agent_config
		where agent_id = $1`
	updateAgentConfigQuery = `
		update agent_config
		set heartbeat_interval_seconds = $2,
		    metrics_interval_seconds = $3,
		    version = $4
		where id = $1
		returning agent_id`
)

func scanAgentConfig(row pgx.Row) (agent.AgentConfig, error) {
	var (
		value                  agent.AgentConfig
		heartbeatIntervalSecs  int
		metricsIntervalSeconds int
	)
	if err := row.Scan(
		&value.ID,
		&value.AgentID,
		&heartbeatIntervalSecs,
		&metricsIntervalSeconds,
		&value.Version,
	); err != nil {
		return agent.AgentConfig{}, err
	}

	value.Heartbeat.Interval = time.Duration(heartbeatIntervalSecs) * time.Second
	value.Metrics.Interval = time.Duration(metricsIntervalSeconds) * time.Second

	return value, nil
}

type AgentConfigRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (r *AgentConfigRepositoryPostgresImpl) Create(ctx context.Context, data agent.CreateAgentConfigDTO) (agent.AgentConfig, error) {
	defaults := agent.NewAgentConfig()

	heartbeatInterval := data.Heartbeat.Interval
	if heartbeatInterval == 0 {
		heartbeatInterval = defaults.Heartbeat.Interval
	}

	metricsInterval := data.Metrics.Interval
	if metricsInterval == 0 {
		metricsInterval = defaults.Metrics.Interval
	}

	version := data.Version
	if version == 0 {
		version = 1
	}

	row := r.pool.QueryRow(
		ctx,
		insertAgentConfigQuery,
		data.AgentID,
		int(heartbeatInterval/time.Second),
		int(metricsInterval/time.Second),
		version,
	)

	return scanAgentConfig(row)
}

func (r *AgentConfigRepositoryPostgresImpl) GetByAgentID(ctx context.Context, agentID uuid.UUID) (agent.AgentConfig, error) {
	row := r.pool.QueryRow(ctx, getAgentConfigByAgentIDQuery, agentID)
	value, err := scanAgentConfig(row)
	if err == nil {
		return value, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return agent.AgentConfig{}, err
	}

	// Bootstrap a default config for existing agents that have no config row yet.
	return r.Create(ctx, agent.CreateAgentConfigDTO{AgentID: agentID, Version: 1})
}

func (r *AgentConfigRepositoryPostgresImpl) Update(ctx context.Context, value *agent.AgentConfig) error {
	row := r.pool.QueryRow(
		ctx,
		updateAgentConfigQuery,
		value.ID,
		int(value.Heartbeat.Interval/time.Second),
		int(value.Metrics.Interval/time.Second),
		value.Version,
	)

	var agentID uuid.UUID
	if err := row.Scan(&agentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("agent config not found: %s", value.ID)
		}
		return err
	}

	value.AgentID = agentID
	return nil
}

func NewAgentConfigRepository(pool *pgxpool.Pool) *AgentConfigRepositoryPostgresImpl {
	return &AgentConfigRepositoryPostgresImpl{pool: pool}
}

var _ agent.AgentConfigRepository = &AgentConfigRepositoryPostgresImpl{}
