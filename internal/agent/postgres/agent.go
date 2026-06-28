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
	agentSelectColumns = "id, host_id, version, created_at, last_seen_at"

	insertAgentQuery           = "insert into agent (host_id, version) values ($1, $2) returning " + agentSelectColumns
	getAgentByIDQuery          = "select " + agentSelectColumns + " from agent where id = $1"
	updateAgentQuery           = "update agent set host_id = $2, version = $3 where id = $1 returning created_at, last_seen_at"
	deleteAgentQuery           = "delete from agent where id = $1"
	updateAgentLastSeenAtQuery = "update agent set last_seen_at = $2 where id = $1"
)

func scanAgent(row pgx.Row) (agent.Agent, error) {
	var value agent.Agent
	if err := row.Scan(
		&value.ID,
		&value.HostID,
		&value.Version,
		&value.CreatedAt,
		&value.LastSeenAt,
	); err != nil {
		return agent.Agent{}, err
	}
	return value, nil
}

type AgentRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (r *AgentRepositoryPostgresImpl) Create(ctx context.Context, data agent.CreateAgentDTO) (agent.Agent, error) {
	row := r.pool.QueryRow(ctx, insertAgentQuery, data.HostID, data.Version)
	return scanAgent(row)
}

func (r *AgentRepositoryPostgresImpl) GetByID(ctx context.Context, id uuid.UUID) (agent.Agent, error) {
	row := r.pool.QueryRow(ctx, getAgentByIDQuery, id)
	return scanAgent(row)
}

func (r *AgentRepositoryPostgresImpl) Update(ctx context.Context, value *agent.Agent) error {
	row := r.pool.QueryRow(ctx, updateAgentQuery, value.ID, value.HostID, value.Version)
	if err := row.Scan(&value.CreatedAt, &value.LastSeenAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("agent not found: %s", value.ID)
		}
		return err
	}
	return nil
}

func (r *AgentRepositoryPostgresImpl) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, deleteAgentQuery, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}
	return nil
}

func (r *AgentRepositoryPostgresImpl) UpdateLastSeenAt(ctx context.Context, id uuid.UUID, lastSeenAt time.Time) error {
	tag, err := r.pool.Exec(ctx, updateAgentLastSeenAtQuery, id, lastSeenAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("agent not found: %s", id)
	}
	return nil
}

func NewAgentRepository(pool *pgxpool.Pool) *AgentRepositoryPostgresImpl {
	return &AgentRepositoryPostgresImpl{pool: pool}
}

var _ agent.Repository = &AgentRepositoryPostgresImpl{}
