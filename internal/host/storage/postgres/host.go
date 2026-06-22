package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
	"github.com/yazmeyaa/hosthalla/internal/host/storage"
)

const hostSelectColumns = "id, name, description, ip, port, created_at, updated_at"

func scanHost(row pgx.Row) (host.Host, error) {
	var result host.Host
	if err := row.Scan(
		&result.ID,
		&result.Name,
		&result.Description,
		&result.IP,
		&result.Port,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return host.Host{}, err
	}
	return result, nil
}

type HostRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

// CreateHost implements storage.HostRepository.
func (h HostRepositoryPostgresImpl) CreateHost(ctx context.Context, data storage.CreateHostDTO) (host.Host, error) {
	const insertHostQuery = "insert into host (name, description, ip, port) values ($1, $2, $3, $4) returning id, name, description, ip, port, created_at, updated_at"
	row := h.pool.QueryRow(ctx, insertHostQuery, data.Name, data.Description, data.IP, data.Port)
	return scanHost(row)
}

// DeleteHost implements storage.HostRepository.
func (h HostRepositoryPostgresImpl) DeleteHost(ctx context.Context, hostID host.HostID) error {
	const deleteHostQuery = "delete from host where id = $1"
	tag, err := h.pool.Exec(ctx, deleteHostQuery, uuid.UUID(hostID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("host not found: %s", hostID)
	}
	return nil
}

// GetHostByID implements storage.HostRepository.
func (h HostRepositoryPostgresImpl) GetHostByID(ctx context.Context, hostID host.HostID) (host.Host, error) {
	query := "select " + hostSelectColumns + " from host where id = $1"
	row := h.pool.QueryRow(ctx, query, uuid.UUID(hostID))
	return scanHost(row)
}

// ListHosts implements storage.HostRepository.
func (h HostRepositoryPostgresImpl) ListHosts(ctx context.Context) ([]host.Host, error) {
	query := "select " + hostSelectColumns + " from host order by created_at desc"
	rows, err := h.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []host.Host
	for rows.Next() {
		host, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, host)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

// UpdateHost implements storage.HostRepository.
func (h HostRepositoryPostgresImpl) UpdateHost(ctx context.Context, host *host.Host) error {
	const updateHostQuery = "update host set name = $2, description = $3, ip = $4, port = $5, updated_at = now() where id = $1 returning updated_at"
	row := h.pool.QueryRow(ctx, updateHostQuery, uuid.UUID(host.ID), host.Name, host.Description, host.IP, host.Port)
	if err := row.Scan(&host.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("host not found: %s", host.ID)
		}
		return err
	}
	return nil
}

func NewHostRepository(pool *pgxpool.Pool) *HostRepositoryPostgresImpl {
	return &HostRepositoryPostgresImpl{pool}
}

var _ storage.HostRepository = &HostRepositoryPostgresImpl{}
