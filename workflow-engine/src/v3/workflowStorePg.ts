import type { WorkflowId } from "../v1/core/spec.ts";
import { replayJournal } from "../v2/adapters/store-file/storeFile.ts";
import type { PendingEffect, PendingTask, WorkflowJournalRecord, WorkflowProjection, WorkflowStore } from "../v2/runtime/ports.ts";
import { query, withClient } from "./postgres.ts";

const SCHEMA_SQL = [
  `create table if not exists workflow_journal (
    id bigserial primary key,
    workflow_id text not null,
    record_kind text not null,
    event_id text,
    commit_id text not null,
    recorded_at timestamptz not null,
    record_json jsonb not null
  )`,
  "create index if not exists workflow_journal_workflow_idx on workflow_journal (workflow_id, id)",
  "create unique index if not exists workflow_journal_event_idx on workflow_journal (workflow_id, event_id) where event_id is not null",
  "create index if not exists workflow_journal_recorded_at_idx on workflow_journal (recorded_at desc)",
  `create table if not exists workflow_state (
    workflow_id text primary key,
    instance_version integer not null,
    projection_json jsonb not null,
    updated_at timestamptz not null
  )`
];

const schemaReadyByUrl = new Map<string, Promise<void>>();

export class PgWorkflowStore implements WorkflowStore {
  private readonly schemaReady: Promise<void>;
  private readonly databaseUrl: string;

  constructor(databaseUrl = process.env.DATABASE_URL?.trim() ?? "") {
    this.databaseUrl = databaseUrl;
    this.schemaReady = ensureSchema(this.databaseUrl);
  }

  async load(workflowId: WorkflowId): Promise<WorkflowProjection | null> {
    if (!this.databaseUrl) return null;
    await this.schemaReady;
    const rows = await query<{ projection_json: unknown }>(
      "select projection_json from workflow_state where workflow_id = $1",
      [workflowId],
      this.databaseUrl
    );
    if (rows.length > 0) {
      return projectionFromJson(rows[0]!.projection_json);
    }

    const journalRows = await query<{ record_json: unknown }>(
      "select record_json from workflow_journal where workflow_id = $1 order by id asc",
      [workflowId],
      this.databaseUrl
    );
    if (journalRows.length === 0) return null;
    return replayJournal(journalRows.map((row) => JSON.stringify(row.record_json)).join("\n"));
  }

  async append(record: WorkflowJournalRecord, expectedVersion?: number): Promise<void> {
    if (!this.databaseUrl) throw new Error("DATABASE_URL is required for PgWorkflowStore");
    await this.schemaReady;
    const meta = recordMeta(record);
    await withClient(async (client) => {
      await client.query("begin");
      try {
        const currentRows = await client.query<{ instance_version: number; projection_json: unknown }>(
          "select instance_version, projection_json from workflow_state where workflow_id = $1 for update",
          [meta.workflowId]
        );
        const currentProjection = currentRows.rows[0] ? projectionFromJson(currentRows.rows[0]!.projection_json) : null;
        const currentVersion = currentRows.rows[0]?.instance_version ?? 0;

        if ((record.kind === "commit" || record.kind === "rejection") && expectedVersion !== undefined && currentVersion !== expectedVersion) {
          throw new WorkflowVersionConflictError(meta.workflowId, expectedVersion, currentVersion);
        }

        const nextProjection = applyRecordToProjection(currentProjection, record);
        await client.query(
          `insert into workflow_journal
            (workflow_id, record_kind, event_id, commit_id, recorded_at, record_json)
           values ($1, $2, $3, $4, $5, $6)`,
          [meta.workflowId, meta.recordKind, meta.eventId, meta.commitId, meta.recordedAt, JSON.stringify(record)]
        );

        if (record.kind === "commit" || record.kind === "rejection") {
          const nextVersion = nextProjection.instance?.version ?? currentVersion;
          const upsert = await client.query(
            `insert into workflow_state (workflow_id, instance_version, projection_json, updated_at)
             values ($1, $2, $3, now())
             on conflict (workflow_id) do update set
               instance_version = excluded.instance_version,
               projection_json = excluded.projection_json,
               updated_at = excluded.updated_at
             where workflow_state.instance_version = $4`,
            [meta.workflowId, nextVersion, JSON.stringify(projectionToJson(nextProjection)), expectedVersion ?? currentVersion]
          );
          if (upsert.rowCount === 0) {
            throw new WorkflowVersionConflictError(meta.workflowId, expectedVersion ?? currentVersion, currentVersion);
          }
        } else {
          await client.query(
            `insert into workflow_state (workflow_id, instance_version, projection_json, updated_at)
             values ($1, $2, $3, now())
             on conflict (workflow_id) do update set
               instance_version = excluded.instance_version,
               projection_json = excluded.projection_json,
               updated_at = excluded.updated_at`,
            [meta.workflowId, currentVersion, JSON.stringify(projectionToJson(nextProjection))]
          );
        }

        await client.query("commit");
      } catch (err) {
        await client.query("rollback").catch(() => undefined);
        if (isDuplicateEventError(err)) {
          throw new DuplicateWorkflowEventError(meta.workflowId, meta.eventId ?? meta.commitId);
        }
        throw err;
      }
    }, this.databaseUrl);
  }

  async listWorkflowIds(): Promise<WorkflowId[]> {
    if (!this.databaseUrl) return [];
    await this.schemaReady;
    const rows = await query<{ workflow_id: string }>(
      `select workflow_id from workflow_state
       union
       select workflow_id from workflow_journal
       order by workflow_id asc`,
      [],
      this.databaseUrl
    );
    return rows.map((row) => row.workflow_id);
  }
}

function ensureSchema(databaseUrl: string): Promise<void> {
  if (!databaseUrl) return Promise.resolve();
  const existing = schemaReadyByUrl.get(databaseUrl);
  if (existing) return existing;
  const ready = withClient(async (client) => {
    for (const statement of SCHEMA_SQL) {
      await client.query(statement);
    }
  }, databaseUrl)
    .then(() => undefined)
    .catch((err) => {
      schemaReadyByUrl.delete(databaseUrl);
      throw err;
    });
  schemaReadyByUrl.set(databaseUrl, ready);
  return ready;
}

function recordMeta(record: WorkflowJournalRecord): {
  workflowId: WorkflowId;
  recordKind: string;
  eventId: string | null;
  commitId: string;
  recordedAt: string;
} {
  if (record.kind === "commit") {
    return {
      workflowId: record.workflowId,
      recordKind: record.kind,
      eventId: record.eventId,
      commitId: record.commitId,
      recordedAt: record.recordedAt
    };
  }
  if (record.kind === "rejection") {
    return {
      workflowId: record.workflowId,
      recordKind: record.kind,
      eventId: record.eventId,
      commitId: record.commitId,
      recordedAt: record.recordedAt
    };
  }
  if (record.kind === "effect-acked") {
    return {
      workflowId: record.workflowId,
      recordKind: record.kind,
      eventId: null,
      commitId: record.commitId,
      recordedAt: record.recordedAt
    };
  }
  return {
    workflowId: record.workflowId,
    recordKind: record.kind,
    eventId: null,
    commitId: record.commitId,
    recordedAt: record.recordedAt
  };
}

function projectionFromJson(value: unknown): WorkflowProjection {
  if (!value || typeof value !== "object") {
    return { instance: null, seenEventIds: new Map(), pendingEffects: [], pendingTasks: [] };
  }
  const raw = value as {
    instance?: WorkflowProjection["instance"];
    seenEventIds?: Array<[string, string]> | Record<string, string>;
    pendingEffects?: PendingEffect[];
    pendingTasks?: PendingTask[];
  };
  return {
    instance: raw.instance ?? null,
    seenEventIds: seenEventIdsFromJson(raw.seenEventIds),
    pendingEffects: Array.isArray(raw.pendingEffects) ? raw.pendingEffects : [],
    pendingTasks: Array.isArray(raw.pendingTasks) ? raw.pendingTasks : []
  };
}

function projectionToJson(projection: WorkflowProjection): {
  instance: WorkflowProjection["instance"];
  seenEventIds: Array<[string, string]>;
  pendingEffects: PendingEffect[];
  pendingTasks: PendingTask[];
} {
  return {
    instance: projection.instance,
    seenEventIds: [...projection.seenEventIds.entries()],
    pendingEffects: projection.pendingEffects,
    pendingTasks: projection.pendingTasks
  };
}

function seenEventIdsFromJson(value: unknown): Map<string, string> {
  if (!value) return new Map();
  if (Array.isArray(value)) {
    return new Map(value.filter((entry): entry is [string, string] => Array.isArray(entry) && entry.length === 2 && typeof entry[0] === "string" && typeof entry[1] === "string"));
  }
  if (typeof value === "object") {
    return new Map(
      Object.entries(value as Record<string, unknown>).flatMap(([key, v]) => (typeof v === "string" ? ([[key, v] as [string, string]]) : []))
    );
  }
  return new Map();
}

function applyRecordToProjection(current: WorkflowProjection | null, record: WorkflowJournalRecord): WorkflowProjection {
  const base = current ?? { instance: null, seenEventIds: new Map<string, string>(), pendingEffects: [], pendingTasks: [] };
  const seenEventIds = new Map(base.seenEventIds);
  const pendingEffects = new Map(base.pendingEffects.map((effect) => [effect.effectId, effect] as const));
  const pendingTasks = new Map(base.pendingTasks.map((task) => [task.taskId, task] as const));
  let instance = base.instance;

  if (record.kind === "commit") {
    instance = record.nextInstance;
    seenEventIds.set(record.eventId, record.recordedAt);
    for (const eff of record.effects) pendingEffects.set(eff.effectId, eff);
    for (const task of record.tasks) pendingTasks.set(task.taskId, task);
  } else if (record.kind === "rejection") {
    instance = record.nextInstance;
    seenEventIds.set(record.eventId, record.recordedAt);
  } else if (record.kind === "effect-acked") {
    pendingEffects.delete(record.effectId);
  } else if (record.kind === "task-acked") {
    pendingTasks.delete(record.taskId);
  }

  return {
    instance,
    seenEventIds,
    pendingEffects: [...pendingEffects.values()],
    pendingTasks: [...pendingTasks.values()]
  };
}

function isDuplicateEventError(err: unknown): boolean {
  return typeof err === "object" && err !== null && "code" in err && (err as { code?: string }).code === "23505";
}

export class DuplicateWorkflowEventError extends Error {
  workflowId: string;
  eventId: string;

  constructor(workflowId: string, eventId: string) {
    super("duplicate workflow event");
    this.workflowId = workflowId;
    this.eventId = eventId;
    this.name = "DuplicateWorkflowEventError";
  }
}

export class WorkflowVersionConflictError extends Error {
  workflowId: string;
  expectedVersion: number;
  currentVersion: number;

  constructor(workflowId: string, expectedVersion: number, currentVersion: number) {
    super("workflow version conflict");
    this.workflowId = workflowId;
    this.expectedVersion = expectedVersion;
    this.currentVersion = currentVersion;
    this.name = "WorkflowVersionConflictError";
  }
}
