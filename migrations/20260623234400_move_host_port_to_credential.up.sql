alter table host_credential
    add column port integer;

update host_credential hc
set port = h.port
from host h
where hc.host_id = h.id and hc.port is null;

update host_credential
set port = 22
where port is null;

alter table host_credential
    alter column port set default 22,
    alter column port set not null;

alter table host_credential
    add constraint host_credential_port_check check (port between 1 and 65535);

alter table host
    drop constraint if exists host_ip_port_key;

alter table host
    add constraint host_ip_key unique (ip);

alter table host
    drop column port;
