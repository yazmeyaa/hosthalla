alter table host
    add column monitoring_agent_id uuid;

alter table host
    add constraint host_monitoring_agent_id_fkey
    foreign key (monitoring_agent_id) references agent(id) on delete set null;

create index host_monitoring_agent_id_idx on host (monitoring_agent_id);
