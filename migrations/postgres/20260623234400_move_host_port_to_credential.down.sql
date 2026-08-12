alter table host
    add column port integer not null default 22 check (port between 1 and 65535);

update host h
set port = hc.port
from (
    select host_id, min(port) as port
    from host_credential
    group by host_id
) hc
where h.id = hc.host_id;

alter table host
    drop constraint if exists host_ip_key;

alter table host
    add constraint host_ip_port_key unique (ip, port);

alter table host_credential
    drop constraint if exists host_credential_port_check;

alter table host_credential
    drop column if exists port;
