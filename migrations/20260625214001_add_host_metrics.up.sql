create table host_metric_snapshot (
    id bigint generated always as identity primary key,
    host_id uuid not null references host(id) on delete cascade,
    timestamp timestamptz not null,
    created_at timestamptz not null default now(),
    unique (host_id, timestamp)
);

create index host_metric_snapshot_host_id_timestamp_idx
    on host_metric_snapshot (host_id, timestamp desc);

create table host_metric (
    snapshot_id bigint not null references host_metric_snapshot(id) on delete cascade,
    position integer not null check (position >= 0),
    cpu_usage_percentage double precision not null check (cpu_usage_percentage >= 0),
    memory_usage_bytes bigint not null check (memory_usage_bytes >= 0),
    disk_usage_bytes bigint not null check (disk_usage_bytes >= 0),
    network_rx_bytes bigint not null check (network_rx_bytes >= 0),
    network_tx_bytes bigint not null check (network_tx_bytes >= 0),
    created_at timestamptz not null default now(),
    primary key (snapshot_id, position)
);
