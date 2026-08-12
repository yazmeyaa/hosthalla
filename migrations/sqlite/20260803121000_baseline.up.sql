create table profile (
    id text primary key,
    username text not null unique,
    created_at timestamp not null,
    updated_at timestamp not null
);

create table password_authentication (
    id text primary key,
    profile_id text not null references profile(id) on delete cascade,
    password_hash text not null,
    created_at timestamp not null,
    updated_at timestamp not null
);

create table session (
    id text primary key,
    profile_id text not null references profile(id) on delete cascade,
    created_at timestamp not null,
    updated_at timestamp not null
);

create table api_token (
    id text primary key,
    profile_id text not null references profile(id) on delete cascade,
    name text not null,
    prefix text not null,
    hash text not null unique,
    scopes text not null default '[]',
    last_used_at timestamp,
    created_at timestamp not null,
    expires_at timestamp,
    revoked_at timestamp
);

create index api_token_profile_id_idx on api_token(profile_id);
create index api_token_prefix_idx on api_token(prefix);

create table host (
    id text primary key,
    name text not null,
    description text not null default '',
    ip text not null unique,
    monitoring_agent_id text references agent(id) on delete set null,
    created_at timestamp not null,
    updated_at timestamp not null
);

create index host_monitoring_agent_id_idx on host(monitoring_agent_id);

create table host_note (
    id text primary key,
    host_id text references host(id) on delete cascade,
    title text not null,
    body text not null,
    created_at timestamp not null,
    updated_at timestamp not null
);

create table tag (
    id text primary key,
    name text not null unique,
    created_at timestamp not null,
    updated_at timestamp not null,
    check (name = lower(trim(name))),
    check (length(trim(name)) > 0)
);

create table host_tag (
    host_id text not null references host(id) on delete cascade,
    tag_id text not null references tag(id) on delete cascade,
    primary key (host_id, tag_id)
);

create index host_tag_tag_id_host_id_idx on host_tag(tag_id, host_id);

create table host_credential (
    id text primary key,
    host_id text not null references host(id) on delete cascade,
    name text not null default '',
    type text not null,
    username text not null default '',
    port integer not null default 22 check (port between 1 and 65535),
    secret blob not null,
    description text not null default '',
    created_at timestamp not null,
    updated_at timestamp not null
);

create table agent (
    id text primary key,
    host_id text not null unique references host(id) on delete cascade,
    version text not null,
    created_at timestamp not null,
    last_seen_at timestamp not null
);

create index agent_host_id_idx on agent(host_id);
create index agent_last_seen_at_idx on agent(last_seen_at desc);

create table agent_config (
    id text primary key,
    agent_id text not null unique references agent(id) on delete cascade,
    heartbeat_interval_seconds integer not null default 5,
    metrics_interval_seconds integer not null default 30,
    version integer not null default 1
);

create index agent_config_agent_id_idx on agent_config(agent_id);

create table host_system_info (
    host_id text primary key references host(id) on delete cascade,
    hostname text not null,
    os_name text not null,
    os_version text not null,
    os_kernel text not null,
    total_memory_bytes integer not null check (total_memory_bytes >= 0),
    cpu_name text not null,
    cpu_architecture text not null,
    cpu_cores integer not null check (cpu_cores >= 0),
    cpu_frequency real not null check (cpu_frequency >= 0),
    cpu_threads integer not null check (cpu_threads >= 0),
    total_disk_bytes integer not null check (total_disk_bytes >= 0),
    created_at timestamp not null,
    updated_at timestamp not null
);

create table host_system_info_gpu (
    host_id text not null references host_system_info(host_id) on delete cascade,
    position integer not null check (position >= 0),
    name text not null,
    created_at timestamp not null,
    primary key (host_id, position)
);

create table host_metric_snapshot (
    id integer primary key autoincrement,
    host_id text not null references host(id) on delete cascade,
    timestamp timestamp not null,
    created_at timestamp not null default current_timestamp,
    unique (host_id, timestamp)
);

create index host_metric_snapshot_host_id_timestamp_idx on host_metric_snapshot(host_id, timestamp desc);

create table host_metric (
    snapshot_id integer not null references host_metric_snapshot(id) on delete cascade,
    position integer not null check (position >= 0),
    cpu_usage_percentage real not null check (cpu_usage_percentage >= 0),
    memory_usage_bytes integer not null check (memory_usage_bytes >= 0),
    disk_usage_bytes integer not null check (disk_usage_bytes >= 0),
    network_rx_bytes integer not null check (network_rx_bytes >= 0),
    network_tx_bytes integer not null check (network_tx_bytes >= 0),
    created_at timestamp not null default current_timestamp,
    primary key (snapshot_id, position)
);
