create table profile (
    id uuid default uuidv7() primary key,
    username varchar(255) not null unique,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table password_authentication (
    id uuid default uuidv7() primary key,
    profile_id uuid not null references profile(id) on delete cascade,
    password_hash varchar(255) not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table session (
    id uuid default uuidv7() primary key,
    profile_id uuid not null references profile(id) on delete cascade,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);
