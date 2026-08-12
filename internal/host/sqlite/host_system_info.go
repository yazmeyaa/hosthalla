package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

type HostSystemInfoRepositorySQLiteImpl struct{ db *sql.DB }

func NewHostSystemInfoRepository(db *sql.DB) *HostSystemInfoRepositorySQLiteImpl {
	return &HostSystemInfoRepositorySQLiteImpl{db: db}
}

func (r *HostSystemInfoRepositorySQLiteImpl) GetHostSystemInfoByHostID(ctx context.Context, hostID uuid.UUID) (host.HostSystemInfo, error) {
	return getHostSystemInfoByHostID(ctx, r.db, hostID)
}

func (r *HostSystemInfoRepositorySQLiteImpl) ListHostSystemInfosByHostIDs(ctx context.Context, hostIDs []uuid.UUID) (map[uuid.UUID]host.HostSystemInfo, error) {
	result := make(map[uuid.UUID]host.HostSystemInfo, len(hostIDs))
	if len(hostIDs) == 0 {
		return result, nil
	}
	args := uuidStrings(hostIDs)
	rows, err := r.db.QueryContext(ctx, systemInfoSelect+` where host_id in (`+placeholders(len(hostIDs))+`)`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		value, err := scanHostSystemInfo(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		value.GPUs = make([]host.GPUSystemInfo, 0)
		result[value.HostID] = value
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	gpuRows, err := r.db.QueryContext(ctx, `select host_id, name from host_system_info_gpu where host_id in (`+placeholders(len(hostIDs))+`) order by host_id asc, position asc`, args...)
	if err != nil {
		return nil, err
	}
	defer gpuRows.Close()
	for gpuRows.Next() {
		var rawHostID, name string
		if err := gpuRows.Scan(&rawHostID, &name); err != nil {
			return nil, err
		}
		hostID, err := parseUUID(rawHostID, "gpu host id")
		if err != nil {
			return nil, err
		}
		value, ok := result[hostID]
		if ok {
			value.GPUs = append(value.GPUs, host.GPUSystemInfo{Name: name})
			result[hostID] = value
		}
	}
	return result, gpuRows.Err()
}

func (r *HostSystemInfoRepositorySQLiteImpl) UpsertHostSystemInfo(ctx context.Context, data host.HostSystemInfo) (host.HostSystemInfo, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
insert into host_system_info (
    host_id, hostname, os_name, os_version, os_kernel, total_memory_bytes,
    cpu_name, cpu_architecture, cpu_cores, cpu_frequency, cpu_threads,
    total_disk_bytes, created_at, updated_at
) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
on conflict(host_id) do update set
    hostname = excluded.hostname,
    os_name = excluded.os_name,
    os_version = excluded.os_version,
    os_kernel = excluded.os_kernel,
    total_memory_bytes = excluded.total_memory_bytes,
    cpu_name = excluded.cpu_name,
    cpu_architecture = excluded.cpu_architecture,
    cpu_cores = excluded.cpu_cores,
    cpu_frequency = excluded.cpu_frequency,
    cpu_threads = excluded.cpu_threads,
    total_disk_bytes = excluded.total_disk_bytes,
    updated_at = excluded.updated_at`,
		data.HostID.String(), data.Hostname, data.OS.Name, data.OS.Version, data.OS.Kernel, int64(data.TotalMemoryBytes),
		data.CPU.Name, data.CPU.Architecture, int64(data.CPU.Cores), data.CPU.Frequency, int64(data.CPU.Threads), int64(data.TotalDiskBytes), now, now)
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	if _, err := tx.ExecContext(ctx, `delete from host_system_info_gpu where host_id = ?`, data.HostID.String()); err != nil {
		return host.HostSystemInfo{}, err
	}
	for position, gpu := range data.GPUs {
		if _, err := tx.ExecContext(ctx, `insert into host_system_info_gpu (host_id, position, name, created_at) values (?, ?, ?, ?)`, data.HostID.String(), position, gpu.Name, now); err != nil {
			return host.HostSystemInfo{}, err
		}
	}
	value, err := getHostSystemInfoByHostID(ctx, tx, data.HostID)
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	if err := tx.Commit(); err != nil {
		return host.HostSystemInfo{}, err
	}
	return value, nil
}

const systemInfoSelect = `
select host_id, hostname, os_name, os_version, os_kernel, total_memory_bytes,
       cpu_name, cpu_architecture, cpu_cores, cpu_frequency, cpu_threads, total_disk_bytes
from host_system_info`

func getHostSystemInfoByHostID(ctx context.Context, q queryer, hostID uuid.UUID) (host.HostSystemInfo, error) {
	value, err := scanHostSystemInfo(q.QueryRowContext(ctx, systemInfoSelect+` where host_id = ?`, hostID.String()))
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	rows, err := q.QueryContext(ctx, `select name from host_system_info_gpu where host_id = ? order by position asc`, hostID.String())
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	defer rows.Close()
	value.GPUs = make([]host.GPUSystemInfo, 0)
	for rows.Next() {
		var gpu host.GPUSystemInfo
		if err := rows.Scan(&gpu.Name); err != nil {
			return host.HostSystemInfo{}, err
		}
		value.GPUs = append(value.GPUs, gpu)
	}
	return value, rows.Err()
}

func scanHostSystemInfo(row scanner) (host.HostSystemInfo, error) {
	var value host.HostSystemInfo
	var rawHostID string
	var memory, cores, threads, disk int64
	if err := row.Scan(
		&rawHostID, &value.Hostname, &value.OS.Name, &value.OS.Version, &value.OS.Kernel, &memory,
		&value.CPU.Name, &value.CPU.Architecture, &cores, &value.CPU.Frequency, &threads, &disk,
	); err != nil {
		return host.HostSystemInfo{}, err
	}
	var err error
	value.HostID, err = parseUUID(rawHostID, "host system info host id")
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	if memory < 0 || cores < 0 || threads < 0 || disk < 0 {
		return host.HostSystemInfo{}, fmt.Errorf("host system info contains a negative unsigned value")
	}
	value.TotalMemoryBytes = uint64(memory)
	value.CPU.Cores = uint(cores)
	value.CPU.Threads = uint(threads)
	value.TotalDiskBytes = uint64(disk)
	return value, nil
}

var _ host.HostSystemInfoRepository = (*HostSystemInfoRepositorySQLiteImpl)(nil)
