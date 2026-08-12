package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

const maxSnapshotsPerHost = 500

type HostMetricSnapshotRepositorySQLiteImpl struct{ db *sql.DB }

func NewHostMetricSnapshotRepository(db *sql.DB) *HostMetricSnapshotRepositorySQLiteImpl {
	return &HostMetricSnapshotRepositorySQLiteImpl{db: db}
}

func (r *HostMetricSnapshotRepositorySQLiteImpl) ListHostMetricSnapshots(ctx context.Context, hostID uuid.UUID) ([]host.HostMetricSnapshot, error) {
	records, err := listSnapshotRecords(ctx, r.db, `select id, host_id, timestamp from host_metric_snapshot where host_id = ? order by timestamp desc`, hostID.String())
	if err != nil {
		return nil, err
	}
	if err := loadMetrics(ctx, r.db, records); err != nil {
		return nil, err
	}
	return snapshotValues(records), nil
}

func (r *HostMetricSnapshotRepositorySQLiteImpl) ListRecentHostMetricSnapshotsByHostIDs(ctx context.Context, hostIDs []uuid.UUID, limitPerHost int) (map[uuid.UUID][]host.HostMetricSnapshot, error) {
	result := make(map[uuid.UUID][]host.HostMetricSnapshot, len(hostIDs))
	if len(hostIDs) == 0 || limitPerHost <= 0 {
		return result, nil
	}
	args := uuidStrings(hostIDs)
	args = append(args, limitPerHost)
	records, err := listSnapshotRecords(ctx, r.db, `
select id, host_id, timestamp from (
    select id, host_id, timestamp,
           row_number() over (partition by host_id order by timestamp desc) as snapshot_rank
    from host_metric_snapshot
    where host_id in (`+placeholders(len(hostIDs))+`)
) where snapshot_rank <= ?
order by host_id asc, timestamp desc`, args...)
	if err != nil {
		return nil, err
	}
	if err := loadMetrics(ctx, r.db, records); err != nil {
		return nil, err
	}
	for _, record := range records {
		result[record.Value.HostID] = append(result[record.Value.HostID], record.Value)
	}
	return result, nil
}

func (r *HostMetricSnapshotRepositorySQLiteImpl) ListLatestHostMetricSnapshotsByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]host.HostMetricSnapshot, error) {
	result := make(map[uuid.UUID]host.HostMetricSnapshot, len(hostIDs))
	if len(hostIDs) == 0 {
		return result, nil
	}
	records, err := listSnapshotRecords(ctx, r.db, `
select id, host_id, timestamp from (
    select id, host_id, timestamp,
           row_number() over (partition by host_id order by timestamp desc) as snapshot_rank
    from host_metric_snapshot
    where host_id in (`+placeholders(len(hostIDs))+`)
) where snapshot_rank = 1
order by host_id asc`, uuidStrings(hostIDs)...)
	if err != nil {
		return nil, err
	}
	if err := loadMetrics(ctx, r.db, records); err != nil {
		return nil, err
	}
	for _, record := range records {
		result[record.Value.HostID] = record.Value
	}
	return result, nil
}

func (r *HostMetricSnapshotRepositorySQLiteImpl) CreateHostMetricSnapshot(ctx context.Context, data host.HostMetricSnapshot) (host.HostMetricSnapshot, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `insert into host_metric_snapshot (host_id, timestamp) values (?, ?)`, data.HostID.String(), data.Timestamp)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	snapshotID, err := result.LastInsertId()
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	for position, metric := range data.Metrics {
		_, err := tx.ExecContext(ctx, `
insert into host_metric (
    snapshot_id, position, cpu_usage_percentage, memory_usage_bytes,
    disk_usage_bytes, network_rx_bytes, network_tx_bytes
) values (?, ?, ?, ?, ?, ?, ?)`, snapshotID, position, metric.CPUUsagePercentage, int64(metric.MemoryUsageBytes), int64(metric.DiskUsageBytes), int64(metric.NetworkRxBytes), int64(metric.NetworkTxBytes))
		if err != nil {
			return host.HostMetricSnapshot{}, err
		}
	}
	_, err = tx.ExecContext(ctx, `
delete from host_metric_snapshot
where host_id = ? and id not in (
    select id from host_metric_snapshot where host_id = ? order by timestamp desc limit ?
)`, data.HostID.String(), data.HostID.String(), maxSnapshotsPerHost)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	value, err := getSnapshotByID(ctx, tx, snapshotID)
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return host.HostMetricSnapshot{}, err
	}
	return value, nil
}

type snapshotRecord struct {
	ID    int64
	Value host.HostMetricSnapshot
}

func listSnapshotRecords(ctx context.Context, q queryer, query string, args ...any) ([]snapshotRecord, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]snapshotRecord, 0)
	for rows.Next() {
		var record snapshotRecord
		var rawHostID string
		if err := rows.Scan(&record.ID, &rawHostID, &record.Value.Timestamp); err != nil {
			return nil, err
		}
		record.Value.HostID, err = parseUUID(rawHostID, "metric snapshot host id")
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func loadMetrics(ctx context.Context, q queryer, records []snapshotRecord) error {
	if len(records) == 0 {
		return nil
	}
	args := make([]any, len(records))
	positions := make(map[int64]int, len(records))
	for i, record := range records {
		args[i] = record.ID
		positions[record.ID] = i
	}
	rows, err := q.QueryContext(ctx, `
select snapshot_id, cpu_usage_percentage, memory_usage_bytes, disk_usage_bytes, network_rx_bytes, network_tx_bytes
from host_metric where snapshot_id in (`+placeholders(len(records))+`)
order by snapshot_id asc, position asc`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var snapshotID int64
		var metric host.HostMetric
		var memory, disk, rx, tx int64
		if err := rows.Scan(&snapshotID, &metric.CPUUsagePercentage, &memory, &disk, &rx, &tx); err != nil {
			return err
		}
		if memory < 0 || disk < 0 || rx < 0 || tx < 0 {
			return fmt.Errorf("host metric contains a negative unsigned value")
		}
		metric.MemoryUsageBytes = uint64(memory)
		metric.DiskUsageBytes = uint64(disk)
		metric.NetworkRxBytes = uint64(rx)
		metric.NetworkTxBytes = uint64(tx)
		position, ok := positions[snapshotID]
		if ok {
			records[position].Value.Metrics = append(records[position].Value.Metrics, metric)
		}
	}
	return rows.Err()
}

func getSnapshotByID(ctx context.Context, q queryer, snapshotID int64) (host.HostMetricSnapshot, error) {
	var record snapshotRecord
	var rawHostID string
	record.ID = snapshotID
	if err := q.QueryRowContext(ctx, `select host_id, timestamp from host_metric_snapshot where id = ?`, snapshotID).Scan(&rawHostID, &record.Value.Timestamp); err != nil {
		return host.HostMetricSnapshot{}, err
	}
	var err error
	record.Value.HostID, err = parseUUID(rawHostID, "metric snapshot host id")
	if err != nil {
		return host.HostMetricSnapshot{}, err
	}
	records := []snapshotRecord{record}
	if err := loadMetrics(ctx, q, records); err != nil {
		return host.HostMetricSnapshot{}, err
	}
	return records[0].Value, nil
}

func snapshotValues(records []snapshotRecord) []host.HostMetricSnapshot {
	result := make([]host.HostMetricSnapshot, len(records))
	for i := range records {
		result[i] = records[i].Value
	}
	return result
}

var _ host.HostMetricSnapshotRepository = (*HostMetricSnapshotRepositorySQLiteImpl)(nil)
