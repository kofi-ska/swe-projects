import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { validateSpec } from "../v1/core/validateSpec.ts";
import { decide } from "../v1/core/decide.ts";
import type { InputEnvelope } from "../v1/core/spec.ts";
import { handle } from "../v1/runtime/engine.ts";
import { FileWorkflowStore } from "../v1/adapters/store-file/storeFile.ts";
import { FileIdempotencyStore } from "../v1/adapters/idempotency-file/idempotencyFile.ts";
import { NoopEffectExecutor } from "../v1/adapters/effects/noopEffects.ts";
import { AllowAllQuotaLimiter } from "../v1/adapters/quota/allowAllQuota.ts";
import { ConsoleLogger } from "../v1/adapters/telemetry/consoleLogger.ts";
import { NoopMetrics } from "../v1/adapters/telemetry/noopMetrics.ts";
import { InMemorySequencer } from "../v1/runtime/inMemorySequencer.ts";
import { FileScheduler } from "../v1/adapters/scheduler-file/schedulerFile.ts";
import type { WorkflowStore } from "../v1/runtime/ports.ts";
import type { Decision, Instance } from "../v1/core/spec.ts";

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
    idempotency: new FileIdempotencyStore(dir),
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    scheduler,
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

  // Small payload should run and then dedupe on replay.
  const ok: InputEnvelope = { ...big, eventId: "e2", payload: { a: 1 } };
  const r2 = await handle(deps, ok);
  assert.equal((r2 as any).decision?.rejection, undefined);
  const r3 = await handle(deps, ok);
  assert.equal((r3 as any).deduped, true);
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
  await scheduler.schedule({
    eventId: "e2",
    workflowId: "w1",
    dueAtIso: new Date(Date.now() + 60_000).toISOString(),
    type: "Y",
    payload: { b: 2 }
  });
  const due = await scheduler.popDue(new Date().toISOString(), 10);
  assert.equal(due.length, 1);
  assert.equal(due[0]!.eventId, "e1");
});

test("validateSpec errors on duplicate transition key", () => {
  const res = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: ["log"] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [
      { from: "A", on: "X", to: "B" },
      { from: "A", on: "X", to: "B" }
    ]
  });
  assert.ok(res.issues.some((i) => i.code === "DUPLICATE_TRANSITION"));
});

test("guard op budget prevents complex guards from passing", () => {
  const vr = validateSpec({
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 1 },
    transitions: [
      { from: "A", on: "X", to: "B", guard: { op: "and", args: [{ op: "exists", path: "payload.a" }, { op: "exists", path: "payload.b" }] } }
    ]
  });
  assert.equal(vr.issues.some((i) => i.level === "error"), false);
  const spec = vr.spec!;
  const instance = { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" as const };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: { a: 1, b: 2 } };
  const d = decide(spec, instance, input);
  assert.equal(d.rejection?.reason, "GUARD_FAILED");
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
    idempotency: new FileIdempotencyStore(dir),
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    scheduler: new FileScheduler(dir),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };
  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.deepEqual(res, { rejected: true, reason: "spec-mismatch" });
});

test("runtime returns store-save-failed on save error", async () => {
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
    async load(_workflowId: string): Promise<Instance | null> {
      return { workflowId: "w1", specId: "S", specVersion: 1, state: "A", context: {}, version: 0, status: "RUNNING" };
    }
    async save(_instance: Instance, _expectedVersion: number): Promise<void> {
      throw new Error("boom");
    }
    async appendHistory(_workflowId: string, _record: { input: InputEnvelope; decision: Decision }): Promise<void> {}
  }

  const deps = {
    spec,
    store: new ThrowingStore(),
    idempotency: new FileIdempotencyStore(dir),
    effects: new NoopEffectExecutor(),
    quota: new AllowAllQuotaLimiter(),
    sequencer: new InMemorySequencer(),
    scheduler: new FileScheduler(dir),
    clock: { nowIso: () => new Date().toISOString() },
    logger: new ConsoleLogger(),
    metrics: new NoopMetrics()
  };

  const input: InputEnvelope = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await handle(deps, input);
  assert.deepEqual(res, { rejected: true, reason: "store-save-failed" });
});
