package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/agent"
)

type scanner interface {
	Scan(dest ...any) error
}

type AgentRepositorySQLiteImpl struct{ db *sql.DB }

func NewAgentRepository(db *sql.DB) *AgentRepositorySQLiteImpl {
	return &AgentRepositorySQLiteImpl{db: db}
}

func (r *AgentRepositorySQLiteImpl) Create(ctx context.Context, data agent.CreateAgentDTO) (agent.Agent, error) {
	now := time.Now().UTC()
	value := agent.Agent{ID: uuid.New(), HostID: data.HostID, Version: data.Version, CreatedAt: now, LastSeenAt: now}
	_, err := r.db.ExecContext(ctx, `insert into agent (id, host_id, version, created_at, last_seen_at) values (?, ?, ?, ?, ?)`, value.ID.String(), value.HostID.String(), value.Version, now, now)
	return value, err
}

func (r *AgentRepositorySQLiteImpl) List(ctx context.Context) ([]agent.Agent, error) {
	rows, err := r.db.QueryContext(ctx, agentSelect+` order by created_at desc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]agent.Agent, 0)
	for rows.Next() {
		value, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *AgentRepositorySQLiteImpl) GetByID(ctx context.Context, id uuid.UUID) (agent.Agent, error) {
	return scanAgent(r.db.QueryRowContext(ctx, agentSelect+` where id = ?`, id.String()))
}

func (r *AgentRepositorySQLiteImpl) GetByHostID(ctx context.Context, hostID uuid.UUID) (agent.Agent, error) {
	return scanAgent(r.db.QueryRowContext(ctx, agentSelect+` where host_id = ?`, hostID.String()))
}

func (r *AgentRepositorySQLiteImpl) Update(ctx context.Context, value *agent.Agent) error {
	result, err := r.db.ExecContext(ctx, `update agent set host_id = ?, version = ? where id = ?`, value.HostID.String(), value.Version, value.ID.String())
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("agent not found: %s", value.ID))
}

func (r *AgentRepositorySQLiteImpl) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `delete from agent where id = ?`, id.String())
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("agent not found: %s", id))
}

func (r *AgentRepositorySQLiteImpl) UpdateLastSeenAt(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `update agent set last_seen_at = ? where id = ?`, lastSeenAt, id.String())
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("agent not found: %s", id))
}

const agentSelect = `select id, host_id, version, created_at, last_seen_at from agent`

func scanAgent(row scanner) (agent.Agent, error) {
	var value agent.Agent
	var id, hostID string
	if err := row.Scan(&id, &hostID, &value.Version, &value.CreatedAt, &value.LastSeenAt); err != nil {
		return agent.Agent{}, err
	}
	var err error
	value.ID, err = uuid.Parse(id)
	if err != nil {
		return agent.Agent{}, fmt.Errorf("parse agent id: %w", err)
	}
	value.HostID, err = uuid.Parse(hostID)
	if err != nil {
		return agent.Agent{}, fmt.Errorf("parse agent host id: %w", err)
	}
	return value, nil
}

type AgentConfigRepositorySQLiteImpl struct{ db *sql.DB }

func NewAgentConfigRepository(db *sql.DB) *AgentConfigRepositorySQLiteImpl {
	return &AgentConfigRepositorySQLiteImpl{db: db}
}

func (r *AgentConfigRepositorySQLiteImpl) Create(ctx context.Context, data agent.CreateAgentConfigDTO) (agent.AgentConfig, error) {
	defaults := agent.NewAgentConfig()
	heartbeat := data.Heartbeat.Interval
	if heartbeat == 0 {
		heartbeat = defaults.Heartbeat.Interval
	}
	metrics := data.Metrics.Interval
	if metrics == 0 {
		metrics = defaults.Metrics.Interval
	}
	version := data.Version
	if version == 0 {
		version = 1
	}

	value := agent.AgentConfig{
		ID: uuid.New(), AgentID: data.AgentID,
		Heartbeat: agent.AgentHeartbeatConfig{Interval: heartbeat},
		Metrics:   agent.AgentMetricsConfig{Interval: metrics}, Version: version,
	}
	_, err := r.db.ExecContext(ctx, `
insert into agent_config (id, agent_id, heartbeat_interval_seconds, metrics_interval_seconds, version)
values (?, ?, ?, ?, ?)`, value.ID.String(), value.AgentID.String(), int(heartbeat/time.Second), int(metrics/time.Second), version)
	return value, err
}

func (r *AgentConfigRepositorySQLiteImpl) GetByAgentID(ctx context.Context, agentID uuid.UUID) (agent.AgentConfig, error) {
	value, err := scanAgentConfig(r.db.QueryRowContext(ctx, agentConfigSelect+` where agent_id = ?`, agentID.String()))
	if err == nil {
		return value, nil
	}
	if err != sql.ErrNoRows {
		return agent.AgentConfig{}, err
	}
	return r.Create(ctx, agent.CreateAgentConfigDTO{AgentID: agentID, Version: 1})
}

func (r *AgentConfigRepositorySQLiteImpl) Update(ctx context.Context, value *agent.AgentConfig) error {
	var agentID string
	err := r.db.QueryRowContext(ctx, `
update agent_config
set heartbeat_interval_seconds = ?, metrics_interval_seconds = ?, version = ?
where id = ?
returning agent_id`, int(value.Heartbeat.Interval/time.Second), int(value.Metrics.Interval/time.Second), value.Version, value.ID.String()).Scan(&agentID)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent config not found: %s: %w", value.ID, sql.ErrNoRows)
	}
	if err != nil {
		return err
	}
	value.AgentID, err = uuid.Parse(agentID)
	return err
}

const agentConfigSelect = `select id, agent_id, heartbeat_interval_seconds, metrics_interval_seconds, version from agent_config`

func scanAgentConfig(row scanner) (agent.AgentConfig, error) {
	var value agent.AgentConfig
	var id, agentID string
	var heartbeat, metrics int
	if err := row.Scan(&id, &agentID, &heartbeat, &metrics, &value.Version); err != nil {
		return agent.AgentConfig{}, err
	}
	var err error
	value.ID, err = uuid.Parse(id)
	if err != nil {
		return agent.AgentConfig{}, fmt.Errorf("parse agent config id: %w", err)
	}
	value.AgentID, err = uuid.Parse(agentID)
	if err != nil {
		return agent.AgentConfig{}, fmt.Errorf("parse agent config agent id: %w", err)
	}
	value.Heartbeat.Interval = time.Duration(heartbeat) * time.Second
	value.Metrics.Interval = time.Duration(metrics) * time.Second
	return value, nil
}

func requireAffected(result sql.Result, message string) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("%s: %w", message, sql.ErrNoRows)
	}
	return nil
}

var (
	_ agent.Repository            = (*AgentRepositorySQLiteImpl)(nil)
	_ agent.AgentConfigRepository = (*AgentConfigRepositorySQLiteImpl)(nil)
)
