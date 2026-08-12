package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostManagementMethodRepositorySQLiteImpl struct{ db *sql.DB }

func NewHostManagementMethodRepository(db *sql.DB) *HostManagementMethodRepositorySQLiteImpl {
	return &HostManagementMethodRepositorySQLiteImpl{db: db}
}

func (r *HostManagementMethodRepositorySQLiteImpl) ListHostManagementMethods(ctx context.Context, hostID uuid.UUID) ([]host.HostManagementMethod, error) {
	return r.list(ctx, managementMethodSelect+` where host_id = ? order by created_at asc`, hostID.String())
}

func (r *HostManagementMethodRepositorySQLiteImpl) ListHostManagementMethodsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID][]host.HostManagementMethod, error) {
	result := make(map[uuid.UUID][]host.HostManagementMethod, len(hostIDs))
	if len(hostIDs) == 0 {
		return result, nil
	}
	methods, err := r.list(ctx, managementMethodSelect+` where host_id in (`+placeholders(len(hostIDs))+`) order by host_id asc, created_at asc`, uuidStrings(hostIDs)...)
	if err != nil {
		return nil, err
	}
	for _, method := range methods {
		result[method.HostID] = append(result[method.HostID], method)
	}
	return result, nil
}

func (r *HostManagementMethodRepositorySQLiteImpl) GetHostManagementMethodByID(ctx context.Context, methodID uuid.UUID) (host.HostManagementMethod, error) {
	return scanManagementMethod(r.db.QueryRowContext(ctx, managementMethodSelect+` where id = ?`, methodID.String()))
}

func (r *HostManagementMethodRepositorySQLiteImpl) CreateHostManagementMethod(ctx context.Context, hostID uuid.UUID, data host.CreateHostManagementMethodDTO) (host.HostManagementMethod, error) {
	now := time.Now().UTC()
	value := host.HostManagementMethod{
		ID: uuid.New(), HostID: hostID, Name: data.Name, Type: data.Type, Username: data.Username,
		Port: data.Port, Secret: append([]byte(nil), data.Secret...), Description: data.Description, CreatedAt: now, UpdatedAt: now,
	}
	_, err := r.db.ExecContext(ctx, `
insert into host_credential (id, host_id, name, type, username, port, secret, description, created_at, updated_at)
values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, value.ID.String(), hostID.String(), value.Name, value.Type, value.Username, value.Port, value.Secret, value.Description, now, now)
	return value, err
}

func (r *HostManagementMethodRepositorySQLiteImpl) UpdateHostManagementMethod(ctx context.Context, methodID uuid.UUID, data host.UpdateHostManagementMethodDTO) (host.HostManagementMethod, error) {
	updatedAt := time.Now().UTC()
	result, err := r.db.ExecContext(ctx, `
update host_credential set name = ?, username = ?, port = ?, secret = ?, description = ?, updated_at = ? where id = ?`, data.Name, data.Username, data.Port, data.Secret, data.Description, updatedAt, methodID.String())
	if err != nil {
		return host.HostManagementMethod{}, err
	}
	if err := requireAffected(result, fmt.Sprintf("host management method not found: %s", methodID)); err != nil {
		return host.HostManagementMethod{}, err
	}
	return r.GetHostManagementMethodByID(ctx, methodID)
}

func (r *HostManagementMethodRepositorySQLiteImpl) DeleteHostManagementMethod(ctx context.Context, methodID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `delete from host_credential where id = ?`, methodID.String())
	return err
}

func (r *HostManagementMethodRepositorySQLiteImpl) list(ctx context.Context, query string, args ...any) ([]host.HostManagementMethod, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]host.HostManagementMethod, 0)
	for rows.Next() {
		value, err := scanManagementMethod(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

const managementMethodSelect = `select id, host_id, name, type, username, port, secret, description, created_at, updated_at from host_credential`

func scanManagementMethod(row scanner) (host.HostManagementMethod, error) {
	var value host.HostManagementMethod
	var id, hostID string
	var port int64
	if err := row.Scan(&id, &hostID, &value.Name, &value.Type, &value.Username, &port, &value.Secret, &value.Description, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return host.HostManagementMethod{}, err
	}
	var err error
	value.ID, err = parseUUID(id, "host management method id")
	if err != nil {
		return host.HostManagementMethod{}, err
	}
	value.HostID, err = parseUUID(hostID, "host management method host id")
	if err != nil {
		return host.HostManagementMethod{}, err
	}
	if port < 0 || port > 65535 {
		return host.HostManagementMethod{}, fmt.Errorf("invalid host management method port: %d", port)
	}
	value.Port = uint16(port)
	return value, nil
}

var _ host.HostManagementMethodRepository = (*HostManagementMethodRepositorySQLiteImpl)(nil)
