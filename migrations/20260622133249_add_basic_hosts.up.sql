create table host (
    id uuid default uuidv7() primary key,
    name varchar(255) not null,
    description varchar(512),
    ip inet not null,
    port integer not null check (port between 1 and 65535),
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    unique (ip, port)
);

create table host_note (
    id uuid primary key,
    host_id uuid references host(id) on delete cascade,
    title varchar(255) not null,
    body text not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table host_credential (
    id uuid primary key,
    host_id uuid not null references host(id) on delete cascade,

    type varchar(32) not null,

    username varchar(255),
    secret bytea not null,

    description varchar(255),

    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);