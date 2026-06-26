package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

const hostSelectColumns = "h.id, h.name, h.description, coalesce(array_agg(t.name order by t.name) filter (where t.id is not null), '{}'::text[]) as tags, h.ip, h.created_at, h.updated_at"
const hostGroupByColumns = "h.id, h.name, h.description, h.ip, h.created_at, h.updated_at"

type hostQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type hostExecer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func scanHost(row pgx.Row) (host.Host, error) {
	var result host.Host
	if err := row.Scan(
		&result.ID,
		&result.Name,
		&result.Description,
		&result.Tags,
		&result.IP,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return host.Host{}, err
	}
	return result, nil
}

func scanTag(row pgx.Row) (host.Tag, error) {
	var result host.Tag
	if err := row.Scan(
		&result.ID,
		&result.Name,
		&result.CreatedAt,
		&result.UpdatedAt,
	); err != nil {
		return host.Tag{}, err
	}
	return result, nil
}

type HostRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (h HostRepositoryPostgresImpl) CreateHost(ctx context.Context, data host.CreateHostDTO) (host.Host, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return host.Host{}, err
	}
	defer tx.Rollback(ctx)

	const insertHostQuery = "insert into host (name, description, ip) values ($1, $2, $3) returning id"
	var hostID uuid.UUID
	if err := tx.QueryRow(ctx, insertHostQuery, data.Name, data.Description, data.IP).Scan(&hostID); err != nil {
		return host.Host{}, err
	}

	if err := syncHostTags(ctx, tx, hostID, data.Tags); err != nil {
		return host.Host{}, err
	}

	result, err := getHostByID(ctx, tx, hostID)
	if err != nil {
		return host.Host{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return host.Host{}, err
	}
	return result, nil
}

func (h HostRepositoryPostgresImpl) DeleteHost(ctx context.Context, hostID uuid.UUID) error {
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

func (h HostRepositoryPostgresImpl) GetHostByID(ctx context.Context, hostID uuid.UUID) (host.Host, error) {
	return getHostByID(ctx, h.pool, hostID)
}

func (h HostRepositoryPostgresImpl) ListHosts(ctx context.Context, filter host.ListHostsFilter) ([]host.Host, error) {
	query := "select " + hostSelectColumns + `
from host h
left join host_tag ht on ht.host_id = h.id
left join tag t on t.id = ht.tag_id`

	args := []any{}
	if len(filter.Tags) > 0 {
		query += `
where h.id in (
    select filtered_ht.host_id
    from host_tag filtered_ht
    join tag filtered_t on filtered_t.id = filtered_ht.tag_id
    where filtered_t.name = any($1::text[])
    group by filtered_ht.host_id
    having count(distinct filtered_t.name) = $2
)`
		args = append(args, filter.Tags, len(filter.Tags))
	}

	query += " group by " + hostGroupByColumns + " order by h.created_at desc"
	rows, err := h.pool.Query(ctx, query, args...)
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

func (h HostRepositoryPostgresImpl) ListTags(ctx context.Context) ([]host.Tag, error) {
	const query = "select id, name, created_at, updated_at from tag order by name asc"
	rows, err := h.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []host.Tag
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tags, nil
}

func (h HostRepositoryPostgresImpl) UpdateHost(ctx context.Context, targetHost *host.Host) error {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	const updateHostQuery = "update host set name = $2, description = $3, ip = $4, updated_at = now() where id = $1 returning updated_at"
	row := tx.QueryRow(ctx, updateHostQuery, uuid.UUID(targetHost.ID), targetHost.Name, targetHost.Description, targetHost.IP)
	if err := row.Scan(&targetHost.UpdatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("host not found: %s", targetHost.ID)
		}
		return err
	}

	if err := syncHostTags(ctx, tx, targetHost.ID, targetHost.Tags); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func getHostByID(ctx context.Context, q hostQueryer, hostID uuid.UUID) (host.Host, error) {
	query := "select " + hostSelectColumns + `
from host h
left join host_tag ht on ht.host_id = h.id
left join tag t on t.id = ht.tag_id
where h.id = $1
group by ` + hostGroupByColumns
	row := q.QueryRow(ctx, query, uuid.UUID(hostID))
	return scanHost(row)
}

func syncHostTags(ctx context.Context, tx interface {
	hostQueryer
	hostExecer
}, hostID uuid.UUID, tags []string) error {
	if _, err := tx.Exec(ctx, "delete from host_tag where host_id = $1", uuid.UUID(hostID)); err != nil {
		return err
	}

	for _, tagName := range tags {
		tagID, err := upsertTag(ctx, tx, tagName)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "insert into host_tag (host_id, tag_id) values ($1, $2) on conflict do nothing", uuid.UUID(hostID), tagID); err != nil {
			return err
		}
	}
	return nil
}

func upsertTag(ctx context.Context, q hostQueryer, name string) (uuid.UUID, error) {
	const upsertTagQuery = `
insert into tag (name)
values ($1)
on conflict (name) do update set name = excluded.name
returning id`

	var tagID uuid.UUID
	if err := q.QueryRow(ctx, upsertTagQuery, name).Scan(&tagID); err != nil {
		return uuid.Nil, err
	}
	return tagID, nil
}

func NewHostRepository(pool *pgxpool.Pool) *HostRepositoryPostgresImpl {
	return &HostRepositoryPostgresImpl{pool}
}

var _ host.HostRepository = &HostRepositoryPostgresImpl{}
