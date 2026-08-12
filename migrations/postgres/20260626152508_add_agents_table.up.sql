create table agent (
    id uuid default uuidv7() primary key,
    host_id uuid not null references host(id) on delete cascade,
    version varchar(255) not null,
    created_at timestamptz not null default now(),
    last_seen_at timestamptz not null default now()
);

create index agent_host_id_idx on agent (host_id);
create index agent_last_seen_at_idx on agent (last_seen_at desc);
