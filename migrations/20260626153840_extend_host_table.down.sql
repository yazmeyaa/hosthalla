drop index if exists host_monitoring_agent_id_idx;

alter table host
    drop constraint if exists host_monitoring_agent_id_fkey;

alter table host
    drop column if exists monitoring_agent_id;
