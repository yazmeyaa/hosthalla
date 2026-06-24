create table tag (
    id uuid default uuidv7() primary key,
    name varchar(64) not null unique,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    check (name = lower(trim(name))),
    check (length(trim(name)) > 0)
);

create table host_tag (
    host_id uuid not null references host(id) on delete cascade,
    tag_id uuid not null references tag(id) on delete cascade,
    primary key (host_id, tag_id)
);

create index host_tag_tag_id_host_id_idx
    on host_tag (tag_id, host_id);
