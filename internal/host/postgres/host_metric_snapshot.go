package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type hostMetricSnapshotQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type HostMetricSnapshotRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

const maxSnapshotsPerHost = 500

func (h HostMetricSnapshotRepositoryPostgresImpl) ListHostMetricSnapshots(ctx context.Context, hostID uuid.UUID) ([]host.HostMetricSnapshot, error) {
	const listSnapshotsQuery = `
select id, host_id, timestamp
from host_metric_snapshot
where host_id = $1
order by timestamp desc`
	rows, err := h.pool.Query(ctx, listSnapshotsQuery, uuid.UUID(hostID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	snapshots := make([]host.HostMetricSnapshot, 0)
	snapshotIDs := make([]int64, 0)
	for rows.Next() {
		var snapshotID int64
		var snapshot host.HostMetricSnapshot
		if err := rows.Scan(&snapshotID, &snapshot.HostID, &snapshot.Timestamp); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
		snapshotIDs = append(snapshotIDs, snapshotID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for idx, snapshotID := range snapshotIDs {
		metrics, err := listHostMetricsBySnapshotID(ctx, h.pool, snapshotID)
		if err != nil {
			return nil, err
		}
		snapshots[idx].Metrics = metrics
	}

	return snapshots, nil
}

func (h HostMetricSnapshotRepositoryPostgresImpl) ListLatestHostMetricSnapshotsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]host.HostMetricSnapshot, error) {
	if len(hostIDs) == 0 {
		return map[uuid.UUID]host.HostMetricSnapshot{}, nil
	}

	rows, err := h.pool.Query(ctx, `
select distinct on (host_id) id, host_id, timestamp
from host_metric_snapshot
where host_id = any($1)
order by host_id asc, timestamp desc`, hostIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[uuid.UUID]host.HostMetricSnapshot, len(hostIDs))
	snapshotIDs := make([]int64, 0, len(hostIDs))
	snapshotHostIDs := make(map[int64]uuid.UUID, len(hostIDs))
	for rows.Next() {
		var (
			snapshotID int64
			snapshot   host.HostMetricSnapshot
		)
		if err := rows.Scan(&snapshotID, &snapshot.HostID, &snapshot.Timestamp); err != nil {
			return nil, err
		}
		hostID := uuid.UUID(snapshot.HostID)
		result[hostID] = snapshot
		snapshotIDs = append(snapshotIDs, snapshotID)
		snapshotHostIDs[snapshotID] = hostID
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(snapshotIDs) == 0 {
		return result, nil
	}

	metricRows, err := h.pool.Query(ctx, `
select snapshot_id,
       cpu_usage_percentage,
       memory_usage_bytes,
       disk_usage_bytes,
       network_rx_bytes,
       network_tx_bytes
from host_metric
where snapshot_id = any($1)
order by snapshot_id asc, position asc`, snapshotIDs)
	if err != nil {
		return nil, err
	}
	defer metricRows.Close()

	type hostMetricRow struct {
		Metric           host.HostMetric
		MemoryUsageBytes int64
		DiskUsageBytes   int64
		NetworkRxBytes   int64
		NetworkTxBytes   int64
		SnapshotID       int64
		CPUUsagePercent  float64
	}
	for metricRows.Next() {
		var row hostMetricRow
		if err := metricRows.Scan(
			&row.SnapshotID,
			&row.CPUUsagePercent,
			&row.MemoryUsageBytes,
			&row.DiskUsageBytes,
			&row.NetworkRxBytes,
			&row.NetworkTxBytes,
		); err != nil {
			return nil, err
		}
		hostID, ok := snapshotHostIDs[row.SnapshotID]
		if !ok {
			continue
		}
		snapshot := result[hostID]
		metric := host.HostMetric{CPUUsagePercentage: row.CPUUsagePercent}

		metric.MemoryUsageBytes, err = nonNegativeMetricInt64ToUint64(row.MemoryUsageBytes, "memory_usage_bytes")
		if err != nil {
			return nil, err
		}
		metric.DiskUsageBytes, err = nonNegativeMetricInt64ToUint64(row.DiskUsageBytes, "disk_usage_bytes")
		if err != nil {
			return nil, err
		}
		metric.NetworkRxBytes, err = nonNegativeMetricInt64ToUint64(row.NetworkRxBytes, "network_rx_bytes")
		if err != nil {
			return nil, err
		}
		metric.NetworkTxBytes, err = nonNegativeMetricInt64ToUint64(row.NetworkTxBytes, "network_tx_bytes")
		if err != nil {
			return nil, err
		}

		snapshot.Metrics = append(snapshot.Metrics, metric)
		result[hostID] = snapshot
	}
	if err := metricRows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h HostMetricSnapshotRepositoryPostgresImpl) CreateHostMetricSnapshot(ctx context.Context, data host.HostMetricSnapshot) (host.HostMetricSnapshot, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	defer tx.Rollback(ctx)

	const insertSnapshotQuery = `
insert into host_metric_snapshot (host_id, timestamp)
values ($1, $2)
returning id`
	var snapshotID int64
	if err := tx.QueryRow(ctx, insertSnapshotQuery, uuid.UUID(data.HostID), data.Timestamp).Scan(&snapshotID); err != nil {
		return host.HostMetricSnapshot{}, err
	}

	const insertMetricQuery = `
insert into host_metric (
    snapshot_id,
    position,
    cpu_usage_percentage,
    memory_usage_bytes,
    disk_usage_bytes,
    network_rx_bytes,
    network_tx_bytes
)
values ($1, $2, $3, $4, $5, $6, $7)`
	for idx, metric := range data.Metrics {
		if _, err := tx.Exec(
			ctx,
			insertMetricQuery,
			snapshotID,
			idx,
			metric.CPUUsagePercentage,
			int64(metric.MemoryUsageBytes),
			int64(metric.DiskUsageBytes),
			int64(metric.NetworkRxBytes),
			int64(metric.NetworkTxBytes),
		); err != nil {
			return host.HostMetricSnapshot{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
delete from host_metric_snapshot
where host_id = $1
  and id not in (
      select id
      from host_metric_snapshot
      where host_id = $1
      order by timestamp desc
      limit $2
  )`, uuid.UUID(data.HostID), maxSnapshotsPerHost); err != nil {
		return host.HostMetricSnapshot{}, err
	}

	createdSnapshot, err := getHostMetricSnapshotByID(ctx, tx, snapshotID)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return host.HostMetricSnapshot{}, err
	}
	return createdSnapshot, nil
}

func getHostMetricSnapshotByID(ctx context.Context, q hostMetricSnapshotQueryer, snapshotID int64) (host.HostMetricSnapshot, error) {
	const query = "select host_id, timestamp from host_metric_snapshot where id = $1"
	row := q.QueryRow(ctx, query, snapshotID)

	var snapshot host.HostMetricSnapshot
	if err := row.Scan(&snapshot.HostID, &snapshot.Timestamp); err != nil {
		return host.HostMetricSnapshot{}, err
	}

	metrics, err := listHostMetricsBySnapshotID(ctx, q, snapshotID)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	snapshot.Metrics = metrics

	return snapshot, nil
}

func listHostMetricsBySnapshotID(ctx context.Context, q hostMetricSnapshotQueryer, snapshotID int64) ([]host.HostMetric, error) {
	const listMetricsQuery = `
select
    cpu_usage_percentage,
    memory_usage_bytes,
    disk_usage_bytes,
    network_rx_bytes,
    network_tx_bytes
from host_metric
where snapshot_id = $1
order by position asc`
	rows, err := q.Query(ctx, listMetricsQuery, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	metrics := make([]host.HostMetric, 0)
	for rows.Next() {
		var metric host.HostMetric
		var memoryUsageBytes int64
		var diskUsageBytes int64
		var networkRxBytes int64
		var networkTxBytes int64

		if err := rows.Scan(
			&metric.CPUUsagePercentage,
			&memoryUsageBytes,
			&diskUsageBytes,
			&networkRxBytes,
			&networkTxBytes,
		); err != nil {
			return nil, err
		}

		metric.MemoryUsageBytes, err = nonNegativeMetricInt64ToUint64(memoryUsageBytes, "memory_usage_bytes")
		if err != nil {
			return nil, err
		}
		metric.DiskUsageBytes, err = nonNegativeMetricInt64ToUint64(diskUsageBytes, "disk_usage_bytes")
		if err != nil {
			return nil, err
		}
		metric.NetworkRxBytes, err = nonNegativeMetricInt64ToUint64(networkRxBytes, "network_rx_bytes")
		if err != nil {
			return nil, err
		}
		metric.NetworkTxBytes, err = nonNegativeMetricInt64ToUint64(networkTxBytes, "network_tx_bytes")
		if err != nil {
			return nil, err
		}

		metrics = append(metrics, metric)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return metrics, nil
}

func nonNegativeMetricInt64ToUint64(value int64, fieldName string) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s is negative: %d", fieldName, value)
	}
	return uint64(value), nil
}

func NewHostMetricSnapshotRepository(pool *pgxpool.Pool) *HostMetricSnapshotRepositoryPostgresImpl {
	return &HostMetricSnapshotRepositoryPostgresImpl{pool: pool}
}

var _ host.HostMetricSnapshotRepository = &HostMetricSnapshotRepositoryPostgresImpl{}
