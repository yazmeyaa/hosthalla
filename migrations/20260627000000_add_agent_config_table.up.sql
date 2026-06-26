create table agent_config (
    id uuid default uuidv7() primary key,
    agent_id uuid not null unique references agent(id) on delete cascade,
    heartbeat_interval_seconds int not null default 5,
    metrics_interval_seconds int not null default 30,
    version int not null default 1
);

create index agent_config_agent_id_idx on agent_config (agent_id);
