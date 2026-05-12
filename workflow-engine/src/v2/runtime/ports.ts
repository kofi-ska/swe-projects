import type { Decision, Effect, EventId, InputEnvelope, Instance, Json, WorkflowId } from "../core/spec.ts";

export interface PendingEffect extends Effect {
  workflowId: WorkflowId;
  commitId: string;
  recordedAt: string;
}

export interface PendingTask {
  taskId: string;
  workflowId: WorkflowId;
  commitId: string;
  sourceEventId: EventId;
  kind: "nextInput" | "timeout";
  dueAtIso: string;
  type: string;
  payload: Json;
}

export type WorkflowJournalRecord =
  | {
      kind: "commit";
      workflowId: WorkflowId;
      commitId: string;
      eventId: EventId;
      recordedAt: string;
      input: InputEnvelope;
      decision: Decision;
      nextInstance: Instance;
      effects: PendingEffect[];
      tasks: PendingTask[];
    }
  | {
      kind: "rejection";
      workflowId: WorkflowId;
      commitId: string;
      eventId: EventId;
      recordedAt: string;
      input: Pick<InputEnvelope, "workflowId" | "eventId" | "type" | "schemaVersion" | "tenantId" | "actor">;
      reason: string;
      message?: string;
      nextInstance: Instance;
    }
  | {
      kind: "effect-acked";
      workflowId: WorkflowId;
      commitId: string;
      effectId: string;
      recordedAt: string;
    }
  | {
      kind: "task-acked";
      workflowId: WorkflowId;
      commitId: string;
      taskId: string;
      recordedAt: string;
    };

export interface WorkflowProjection {
  instance: Instance | null;
  seenEventIds: Map<EventId, string>;
  pendingEffects: PendingEffect[];
  pendingTasks: PendingTask[];
}

export interface WorkflowStore {
  load(workflowId: WorkflowId): Promise<WorkflowProjection | null>;
  append(record: WorkflowJournalRecord, expectedVersion?: number): Promise<void>;
  listWorkflowIds(): Promise<WorkflowId[]>;
}

export interface IdempotencyStore {
  checkAndMark(eventId: EventId, opts?: { ttlSeconds?: number }): Promise<{ seen: boolean }>;
}

export interface EffectExecutor {
  execute(effect: Effect): Promise<{ ok: boolean; error?: string }>;
}

export interface QuotaLimiter {
  check(input: InputEnvelope): Promise<{ allowed: boolean; reason?: string }>;
}

export interface Sequencer {
  runExclusive<T>(workflowId: WorkflowId, fn: () => Promise<T>): Promise<T>;
}

export interface Scheduler {
  schedule(task: {
    eventId: EventId;
    workflowId: WorkflowId;
    dueAtIso: string;
    type: string;
    payload: unknown;
    dedupeKey?: string;
  }): Promise<void>;
}

export interface Logger {
  info(msg: string, meta?: Record<string, unknown>): void;
  warn(msg: string, meta?: Record<string, unknown>): void;
  error(msg: string, meta?: Record<string, unknown>): void;
}

export interface Metrics {
  inc(name: string, tags?: Record<string, string>): void;
  observeMs(name: string, valueMs: number, tags?: Record<string, string>): void;
}

export interface Clock {
  nowIso(): string;
}
