package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yazmeyaa/hosthalla/internal/host"
)

const hostSystemInfoSelectColumns = `
host_id,
hostname,
os_name,
os_version,
os_kernel,
total_memory_bytes,
cpu_name,
cpu_architecture,
cpu_cores,
cpu_frequency,
cpu_threads,
total_disk_bytes`

type hostSystemInfoQueryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type HostSystemInfoRepositoryPostgresImpl struct {
	pool *pgxpool.Pool
}

func (h HostSystemInfoRepositoryPostgresImpl) GetHostSystemInfoByHostID(ctx context.Context, hostID uuid.UUID) (host.HostSystemInfo, error) {
	return getHostSystemInfoByHostID(ctx, h.pool, hostID)
}

func (h HostSystemInfoRepositoryPostgresImpl) UpsertHostSystemInfo(ctx context.Context, data host.HostSystemInfo) (host.HostSystemInfo, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	defer tx.Rollback(ctx)

	const upsertHostSystemInfoQuery = `
insert into host_system_info (
    host_id,
    hostname,
    os_name,
    os_version,
    os_kernel,
    total_memory_bytes,
    cpu_name,
    cpu_architecture,
    cpu_cores,
    cpu_frequency,
    cpu_threads,
    total_disk_bytes
)
values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
on conflict (host_id) do update set
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
    updated_at = now()`

	_, err = tx.Exec(
		ctx,
		upsertHostSystemInfoQuery,
		uuid.UUID(data.HostID),
		data.Hostname,
		data.OS.Name,
		data.OS.Version,
		data.OS.Kernel,
		int64(data.TotalMemoryBytes),
		data.CPU.Name,
		data.CPU.Architecture,
		int32(data.CPU.Cores),
		data.CPU.Frequency,
		int32(data.CPU.Threads),
		int64(data.TotalDiskBytes),
	)
	if err != nil {
		return host.HostSystemInfo{}, err
	}

	if _, err := tx.Exec(ctx, "delete from host_system_info_gpu where host_id = $1", uuid.UUID(data.HostID)); err != nil {
		return host.HostSystemInfo{}, err
	}

	const insertGPUQuery = `
insert into host_system_info_gpu (host_id, position, name)
values ($1, $2, $3)`
	for idx, gpu := range data.GPUs {
		if _, err := tx.Exec(ctx, insertGPUQuery, uuid.UUID(data.HostID), idx, gpu.Name); err != nil {
			return host.HostSystemInfo{}, err
		}
	}

	result, err := getHostSystemInfoByHostID(ctx, tx, data.HostID)
	if err != nil {
		return host.HostSystemInfo{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return host.HostSystemInfo{}, err
	}
	return result, nil
}

func getHostSystemInfoByHostID(ctx context.Context, q hostSystemInfoQueryer, hostID uuid.UUID) (host.HostSystemInfo, error) {
	query := "select " + hostSystemInfoSelectColumns + " from host_system_info where host_id = $1"
	row := q.QueryRow(ctx, query, uuid.UUID(hostID))
	result, err := scanHostSystemInfo(row)
	if err != nil {
		return host.HostSystemInfo{}, err
	}

	gpuQuery := "select name from host_system_info_gpu where host_id = $1 order by position asc"
	rows, err := q.Query(ctx, gpuQuery, uuid.UUID(hostID))
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	defer rows.Close()

	result.GPUs = make([]host.GPUSystemInfo, 0)
	for rows.Next() {
		var gpu host.GPUSystemInfo
		if err := rows.Scan(&gpu.Name); err != nil {
			return host.HostSystemInfo{}, err
		}
		result.GPUs = append(result.GPUs, gpu)
	}
	if err := rows.Err(); err != nil {
		return host.HostSystemInfo{}, err
	}

	return result, nil
}

func scanHostSystemInfo(row pgx.Row) (host.HostSystemInfo, error) {
	var result host.HostSystemInfo
	var totalMemoryBytes int64
	var cpuCores int32
	var cpuThreads int32
	var totalDiskBytes int64

	if err := row.Scan(
		&result.HostID,
		&result.Hostname,
		&result.OS.Name,
		&result.OS.Version,
		&result.OS.Kernel,
		&totalMemoryBytes,
		&result.CPU.Name,
		&result.CPU.Architecture,
		&cpuCores,
		&result.CPU.Frequency,
		&cpuThreads,
		&totalDiskBytes,
	); err != nil {
		return host.HostSystemInfo{}, err
	}

	convertedMemory, err := nonNegativeInt64ToUint64(totalMemoryBytes, "total_memory_bytes")
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	convertedCores, err := nonNegativeInt32ToUint(cpuCores, "cpu_cores")
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	convertedThreads, err := nonNegativeInt32ToUint(cpuThreads, "cpu_threads")
	if err != nil {
		return host.HostSystemInfo{}, err
	}
	convertedDisk, err := nonNegativeInt64ToUint64(totalDiskBytes, "total_disk_bytes")
	if err != nil {
		return host.HostSystemInfo{}, err
	}

	result.TotalMemoryBytes = convertedMemory
	result.CPU.Cores = convertedCores
	result.CPU.Threads = convertedThreads
	result.TotalDiskBytes = convertedDisk

	return result, nil
}

func nonNegativeInt64ToUint64(value int64, fieldName string) (uint64, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s is negative: %d", fieldName, value)
	}
	return uint64(value), nil
}

func nonNegativeInt32ToUint(value int32, fieldName string) (uint, error) {
	if value < 0 {
		return 0, fmt.Errorf("%s is negative: %d", fieldName, value)
	}
	return uint(value), nil
}

func NewHostSystemInfoRepository(pool *pgxpool.Pool) *HostSystemInfoRepositoryPostgresImpl {
	return &HostSystemInfoRepositoryPostgresImpl{pool: pool}
}

var _ host.HostSystemInfoRepository = &HostSystemInfoRepositoryPostgresImpl{}
