create table host_system_info (
    host_id uuid primary key references host(id) on delete cascade,
    hostname varchar(255) not null,
    os_name varchar(255) not null,
    os_version varchar(255) not null,
    os_kernel varchar(255) not null,
    total_memory_bytes bigint not null check (total_memory_bytes >= 0),
    cpu_name varchar(255) not null,
    cpu_architecture varchar(64) not null,
    cpu_cores integer not null check (cpu_cores >= 0),
    cpu_frequency double precision not null check (cpu_frequency >= 0),
    cpu_threads integer not null check (cpu_threads >= 0),
    total_disk_bytes bigint not null check (total_disk_bytes >= 0),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table host_system_info_gpu (
    host_id uuid not null references host_system_info(host_id) on delete cascade,
    position integer not null check (position >= 0),
    name varchar(255) not null,
    created_at timestamptz not null default now(),
    primary key (host_id, position)
);
