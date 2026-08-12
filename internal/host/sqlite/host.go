package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostRepositorySQLiteImpl struct{ db *sql.DB }

func NewHostRepository(db *sql.DB) *HostRepositorySQLiteImpl {
	return &HostRepositorySQLiteImpl{db: db}
}

func (r *HostRepositorySQLiteImpl) CreateHost(ctx context.Context, data host.CreateHostDTO) (host.Host, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return host.Host{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	id := uuid.New()
	_, err = tx.ExecContext(ctx, `insert into host (id, name, description, ip, created_at, updated_at) values (?, ?, ?, ?, ?, ?)`, id.String(), data.Name, data.Description, data.IP.String(), now, now)
	if err != nil {
		return host.Host{}, err
	}
	if err := syncHostTags(ctx, tx, id, data.Tags); err != nil {
		return host.Host{}, err
	}
	value, err := getHostByID(ctx, tx, id)
	if err != nil {
		return host.Host{}, err
	}
	if err := tx.Commit(); err != nil {
		return host.Host{}, err
	}
	return value, nil
}

func (r *HostRepositorySQLiteImpl) ListHosts(ctx context.Context, filter host.ListHostsFilter) ([]host.Host, error) {
	query := hostSelect
	args := make([]any, 0, len(filter.Tags)+1)
	if len(filter.Tags) > 0 {
		query += ` where h.id in (
select ht.host_id from host_tag ht
join tag t on t.id = ht.tag_id
where t.name in (` + placeholders(len(filter.Tags)) + `)
group by ht.host_id
having count(distinct t.name) = ?)`
		for _, tag := range filter.Tags {
			args = append(args, tag)
		}
		args = append(args, len(filter.Tags))
	}
	query += ` order by h.created_at desc`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]host.Host, 0)
	for rows.Next() {
		value, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range result {
		result[i].Tags, err = listTagNames(ctx, r.db, result[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *HostRepositorySQLiteImpl) ListTags(ctx context.Context) ([]host.Tag, error) {
	rows, err := r.db.QueryContext(ctx, `select id, name, created_at, updated_at from tag order by name asc`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]host.Tag, 0)
	for rows.Next() {
		var value host.Tag
		var id string
		if err := rows.Scan(&id, &value.Name, &value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		value.ID, err = parseUUID(id, "tag id")
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (r *HostRepositorySQLiteImpl) GetHostByID(ctx context.Context, hostID uuid.UUID) (host.Host, error) {
	return getHostByID(ctx, r.db, hostID)
}

func (r *HostRepositorySQLiteImpl) DeleteHost(ctx context.Context, hostID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `delete from host where id = ?`, hostID.String())
	if err != nil {
		return err
	}
	return requireAffected(result, fmt.Sprintf("host not found: %s", hostID))
}

func (r *HostRepositorySQLiteImpl) UpdateHost(ctx context.Context, value *host.Host) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	updatedAt := time.Now().UTC()
	var monitoringAgentID any
	if value.MonitoringAgentID != uuid.Nil {
		monitoringAgentID = value.MonitoringAgentID.String()
	}
	result, err := tx.ExecContext(ctx, `update host set name = ?, description = ?, ip = ?, monitoring_agent_id = ?, updated_at = ? where id = ?`, value.Name, value.Description, value.IP.String(), monitoringAgentID, updatedAt, value.ID.String())
	if err != nil {
		return err
	}
	if err := requireAffected(result, fmt.Sprintf("host not found: %s", value.ID)); err != nil {
		return err
	}
	if err := syncHostTags(ctx, tx, value.ID, value.Tags); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	value.UpdatedAt = updatedAt
	return nil
}

const hostSelect = `select h.id, h.name, h.description, h.ip, h.monitoring_agent_id, h.created_at, h.updated_at from host h`

func getHostByID(ctx context.Context, q queryer, hostID uuid.UUID) (host.Host, error) {
	value, err := scanHost(q.QueryRowContext(ctx, hostSelect+` where h.id = ?`, hostID.String()))
	if err != nil {
		return host.Host{}, err
	}
	value.Tags, err = listTagNames(ctx, q, hostID)
	return value, err
}

func scanHost(row scanner) (host.Host, error) {
	var value host.Host
	var id, ip string
	var monitoringAgentID sql.NullString
	if err := row.Scan(&id, &value.Name, &value.Description, &ip, &monitoringAgentID, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return host.Host{}, err
	}
	var err error
	value.ID, err = parseUUID(id, "host id")
	if err != nil {
		return host.Host{}, err
	}
	value.IP, err = netip.ParseAddr(ip)
	if err != nil {
		return host.Host{}, fmt.Errorf("parse host ip: %w", err)
	}
	if monitoringAgentID.Valid {
		value.MonitoringAgentID, err = parseUUID(monitoringAgentID.String, "monitoring agent id")
		if err != nil {
			return host.Host{}, err
		}
	}
	return value, nil
}

func listTagNames(ctx context.Context, q queryer, hostID uuid.UUID) ([]string, error) {
	rows, err := q.QueryContext(ctx, `select t.name from tag t join host_tag ht on ht.tag_id = t.id where ht.host_id = ? order by t.name asc`, hostID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func syncHostTags(ctx context.Context, tx *sql.Tx, hostID uuid.UUID, tags []string) error {
	if _, err := tx.ExecContext(ctx, `delete from host_tag where host_id = ?`, hostID.String()); err != nil {
		return err
	}
	for _, name := range tags {
		now := time.Now().UTC()
		if _, err := tx.ExecContext(ctx, `insert into tag (id, name, created_at, updated_at) values (?, ?, ?, ?) on conflict(name) do nothing`, uuid.NewString(), name, now, now); err != nil {
			return err
		}
		var tagID string
		if err := tx.QueryRowContext(ctx, `select id from tag where name = ?`, name).Scan(&tagID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `insert into host_tag (host_id, tag_id) values (?, ?) on conflict do nothing`, hostID.String(), tagID); err != nil {
			return err
		}
	}
	return nil
}

var _ host.HostRepository = (*HostRepositorySQLiteImpl)(nil)
