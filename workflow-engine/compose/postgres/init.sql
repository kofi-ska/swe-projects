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

create table if not exists workflow_journal (
  id bigserial primary key,
  workflow_id text not null,
  record_kind text not null,
  event_id text,
  commit_id text not null,
  recorded_at timestamptz not null,
  record_json jsonb not null
);

create index if not exists workflow_journal_workflow_idx
  on workflow_journal (workflow_id, id);

create unique index if not exists workflow_journal_event_idx
  on workflow_journal (workflow_id, event_id)
  where event_id is not null;

create index if not exists workflow_journal_recorded_at_idx
  on workflow_journal (recorded_at desc);

create table if not exists workflow_state (
  workflow_id text primary key,
  instance_version integer not null,
  projection_json jsonb not null,
  updated_at timestamptz not null
);

create table if not exists request_rate_limits (
  bucket text primary key,
  window_started_at timestamptz not null,
  count integer not null,
  updated_at timestamptz not null
);
