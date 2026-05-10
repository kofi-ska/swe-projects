create table if not exists workflow_executions (
  trace_id text primary key,
  service_version text not null,
  route text not null,
  engine_version integer not null,
  workflow_id text,
  event_id text,
  spec_id text,
  spec_version integer,
  outcome text not null,
  reason text,
  request_json jsonb not null,
  response_json jsonb not null,
  created_at timestamptz not null default now()
);

create index if not exists workflow_executions_workflow_id_idx
  on workflow_executions (workflow_id, created_at desc);

create index if not exists workflow_executions_event_id_idx
  on workflow_executions (event_id, created_at desc);
