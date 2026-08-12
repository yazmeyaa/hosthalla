create table api_token (
    id uuid default uuidv7() primary key,
    profile_id uuid not null references profile(id) on delete cascade,
    name varchar(255) not null,
    prefix varchar(32) not null,
    hash varchar(255) not null unique,
    scopes text[] not null default '{}',
    last_used_at timestamptz,
    created_at timestamptz not null default now(),
    expires_at timestamptz,
    revoked_at timestamptz
);

create index api_token_profile_id_idx on api_token(profile_id);
create index api_token_prefix_idx on api_token(prefix);
