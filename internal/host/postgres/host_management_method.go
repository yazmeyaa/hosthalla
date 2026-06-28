package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

const hostManagementMethodSelectColumns = "id, host_id, type, username, port, secret, description, created_at, updated_at"

func scanHostManagementMethod(row pgx.Row) (host.HostManagementMethod, error) {
	var result host.HostManagementMethod
	if err := row.Scan(
		&result.ID,
		&result.HostID,
		&result.Type,
		&result.Username,
		&result.Port,
		&result.Secret,
		&result.Description,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return host.HostManagementMethod{}, err
	}
	return result, nil
}

type HostManagementMethodRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (h HostManagementMethodRepositoryPostgresImpl) ListHostManagementMethods(ctx context.Context, hostID uuid.UUID) ([]host.HostManagementMethod, error) {
	query := "select " + hostManagementMethodSelectColumns + " from host_credential where host_id = $1 order by created_at asc"
	rows, err := h.pool.Query(ctx, query, uuid.UUID(hostID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	methods := make([]host.HostManagementMethod, 0)
	for rows.Next() {
		method, err := scanHostManagementMethod(rows)
		if err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return methods, nil
}

func (h HostManagementMethodRepositoryPostgresImpl) ListHostManagementMethodsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID][]host.HostManagementMethod, error) {
	if len(hostIDs) == 0 {
		return map[uuid.UUID][]host.HostManagementMethod{}, nil
	}
	query := "select " + hostManagementMethodSelectColumns + " from host_credential where host_id = any($1) order by host_id asc, created_at asc"
	rows, err := h.pool.Query(ctx, query, hostIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID][]host.HostManagementMethod, len(hostIDs))
	for rows.Next() {
		method, err := scanHostManagementMethod(rows)
		if err != nil {
			return nil, err
		}
		hostID := uuid.UUID(method.HostID)
		result[hostID] = append(result[hostID], method)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h HostManagementMethodRepositoryPostgresImpl) CreateHostManagementMethod(ctx context.Context, hostID uuid.UUID, data host.CreateHostManagementMethodDTO) (host.HostManagementMethod, error) {
	const insertManagementMethodQuery = "insert into host_credential (id, host_id, type, username, port, secret, description) values ($1, $2, $3, $4, $5, $6, $7) returning id, host_id, type, username, port, secret, description, created_at, updated_at"
	row := h.pool.QueryRow(
		ctx,
		insertManagementMethodQuery,
		uuid.New(),
		uuid.UUID(hostID),
		data.Type,
		data.Username,
		data.Port,
		data.Secret,
		data.Description,
	)
	return scanHostManagementMethod(row)
}

func NewHostManagementMethodRepository(pool *pgxpool.Pool) *HostManagementMethodRepositoryPostgresImpl {
	return &HostManagementMethodRepositoryPostgresImpl{pool: pool}
}

var _ host.HostManagementMethodRepository = &HostManagementMethodRepositoryPostgresImpl{}
