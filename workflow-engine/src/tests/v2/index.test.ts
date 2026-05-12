import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  AllowAllQuotaLimiter,
  ConsoleLogger,
  FileIdempotencyStore,
  FileScheduler,
  FileWorkflowStore,
  InMemorySequencer,
  NoopEffectExecutor,
  NoopMetrics,
  decide,
  handle,
  validateSpec
} from "../../v2/index.ts";
import type { InputEnvelope } from "../../v1/core/spec.ts";
import type { WorkflowJournalRecord, WorkflowProjection, WorkflowStore } from "../../v2/runtime/ports.ts";
import type { Instance } from "../../v1/core/spec.ts";

test("validateSpec rejects non-object", () => {
  const res = validateSpec(null);
  assert.ok(res.issues.some((i) => i.code === "SPEC_NOT_OBJECT"));
});

test("decide takes transition when guard passes", () => {
  const res = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["C"],
    states: ["A", "B", "C"],
    permissions: { effectTypesAllowlist: ["log"] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [
      { from: "A", on: "X", to: "B", guard: { op: "eq", path: "payload.ok", value: true }, effects: [{ type: "log", params: { msg: "hi" } }] }
    ]
  });
  assert.equal(res.issues.some((i) => i.level === "error"), false);
  const spec = res.spec!;
  const instance = { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" as const };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: { ok: true } };
  const d = decide(spec, instance, input);
  assert.equal(d.rejection, undefined);
  assert.deepEqual(d.transitionTaken, { from: "A", to: "B", on: "X" });
  assert.equal(d.effects.length, 1);
});

test("runtime enforces payload size and idempotency", async () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: ["log"] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  });
  assert.equal(vr.issues.some((i) => i.level === "error"), false);
  const spec = vr.spec!;

  const dir = await mkdtemp(join(tmpdir(), "wfeng-"));
  const store = new FileWorkflowStore(dir);
  const scheduler = new FileScheduler(dir);
  const deps = {
    spec,
    store,
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };

  const big: InputEnvelope = {
    eventId: "e1",
    workflowId: "w1",
    type: "X",
    occurredAt: new Date().toISOString(),
    schemaVersion: 1,
    payload: { too: "large payload" }
  };
  const r1 = await handle(deps, big);
  assert.deepEqual(r1, { rejected: true, reason: "payload-too-large" });

  const ok: InputEnvelope = { ...big, eventId: "e2", payload: { a: 1 } };
  const r2 = await handle(deps, ok);
  assert.equal((r2 as any).decision?.rejection, undefined);
  const r3 = await handle(deps, ok);
  assert.equal((r3 as any).deduped, true);
});

test("commit writes durable journal record", async () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: ["log"] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B", effects: [{ type: "log", params: { msg: "done" } }] }]
  });
  const spec = vr.spec!;
  const dir = await mkdtemp(join(tmpdir(), "wfeng-"));
  const store = new FileWorkflowStore(dir);
  const deps = {
    spec,
    store,
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.equal(res.committed, true);
  const projection = await store.load("w1");
  assert.ok(projection?.instance);
  assert.equal(projection?.pendingEffects.length, 1);
});

test("file scheduler popDue returns due tasks", async () => {
  const dir = await mkdtemp(join(tmpdir(), "wfeng-"));
  const scheduler = new FileScheduler(dir);
  await scheduler.schedule({
    eventId: "e1",
    workflowId: "w1",
    dueAtIso: new Date(Date.now() - 1000).toISOString(),
    type: "X",
    payload: { a: 1 }
  });
  const due = await scheduler.popDue(new Date().toISOString(), 10);
  assert.equal(due.length, 1);
  assert.equal(due[0]!.eventId, "e1");
});

test("runtime rejects spec mismatch", async () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 2,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  });
  const spec = vr.spec!;
  const dir = await mkdtemp(join(tmpdir(), "wfeng-"));
  const store = new FileWorkflowStore(dir);
  await store.save(
    { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" },
    0
  );
  const deps = {
    spec,
    store,
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.deepEqual(res, { rejected: true, reason: "spec-mismatch" });
});

test("runtime rejects disallowed effects", () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B", effects: [{ type: "log", params: { msg: "x" } }] }]
  });
  assert.ok(vr.issues.some((i) => i.code === "EFFECT_TYPE_NOT_ALLOWED"));
});

test("runtime returns store-append-failed on append error", async () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  });
  const spec = vr.spec!;
  const dir = await mkdtemp(join(tmpdir(), "wfeng-"));

  class ThrowingStore implements WorkflowStore {
    async load(_workflowId: string): Promise<WorkflowProjection | null> {
      return {
        instance: { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" },
        seenEventIds: new Map(),
        pendingEffects: [],
        pendingTasks: []
      };
    }
    async append(_record: WorkflowJournalRecord): Promise<void> {
      throw new Error("boom");
    }
    async listWorkflowIds(): Promise<string[]> {
      return [];
    }
  }

  const deps = {
    spec,
    store: new ThrowingStore(),
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };

  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.deepEqual(res, { rejected: true, reason: "store-append-failed" });
});

test("runtime dedupes duplicate journal writes", async () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  });
  const spec = vr.spec!;

  class DuplicateStore implements WorkflowStore {
    async load(_workflowId: string): Promise<WorkflowProjection | null> {
      return {
        instance: { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" },
        seenEventIds: new Map(),
        pendingEffects: [],
        pendingTasks: []
      };
    }
    async append(_record: WorkflowJournalRecord): Promise<void> {
      throw { code: "23505" };
    }
    async listWorkflowIds(): Promise<string[]> {
      return [];
    }
  }

  const deps = {
    spec,
    store: new DuplicateStore(),
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.equal((res as { deduped?: boolean }).deduped, true);
});
