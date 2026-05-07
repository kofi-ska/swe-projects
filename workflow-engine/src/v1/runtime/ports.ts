import type { Decision, Effect, EventId, InputEnvelope, Instance, WorkflowId } from "../core/spec.ts";

export interface WorkflowStore {
  load(workflowId: WorkflowId): Promise<Instance | null>;
  save(instance: Instance, expectedVersion: number): Promise<void>;
  appendHistory(workflowId: WorkflowId, record: { input: InputEnvelope; decision: Decision }): Promise<void>;
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
