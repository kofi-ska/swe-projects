import { randomUUID } from "node:crypto";
import type {
  InputEnvelope,
  Instance,
  NextInputTemplate,
  Spec,
  TimeoutRule
} from "../core/spec.ts";
import { decide } from "../core/decide.ts";
import type {
  Clock,
  EffectExecutor,
  Logger,
  Metrics,
  PendingEffect,
  PendingTask,
  QuotaLimiter,
  Sequencer,
  WorkflowJournalRecord,
  WorkflowProjection,
  WorkflowStore
} from "./ports.ts";

export interface EngineDeps {
  spec: Spec;
  store: WorkflowStore;
  effects: EffectExecutor;
  quota: QuotaLimiter;
  sequencer: Sequencer;
  clock: Clock;
  logger: Logger;
  metrics: Metrics;
}

export interface HandleResult {
  committed?: true;
  deduped?: true;
  rejected?: true;
  reason?: string;
  decision?: ReturnType<typeof decide>;
  instance?: Instance;
  queuedEffects?: number;
  queuedTasks?: number;
}

export interface DrainResult {
  processedEffects: number;
  processedTasks: number;
  results: HandleResult[];
}

const IDEMPOTENCY_TTL_MS = 7 * 24 * 60 * 60 * 1000;

export async function handle(deps: EngineDeps, input: InputEnvelope): Promise<HandleResult> {
  return deps.sequencer.runExclusive(input.workflowId, () => handleLocked(deps, input));
}

export async function drainPendingWork(deps: EngineDeps, max = 100): Promise<DrainResult> {
  const workflowIds = await deps.store.listWorkflowIds();
  const results: HandleResult[] = [];
  let processedEffects = 0;
  let processedTasks = 0;

  for (const workflowId of workflowIds) {
    if (results.length >= max) break;
    await deps.sequencer.runExclusive(workflowId, async () => {
      const projection = await loadProjectionCompat(deps.store, workflowId);
      if (!projection) return;

      for (const effect of projection.pendingEffects) {
        if (processedEffects >= max) break;
        const res = await deps.effects.execute(effect);
        deps.metrics.inc("wf.effect.executed", { type: effect.type, ok: String(res.ok) });
        if (res.ok) {
          await appendRecordCompat(deps.store, {
            kind: "effect-acked",
            workflowId,
            commitId: effect.commitId,
            effectId: effect.effectId,
            recordedAt: deps.clock.nowIso()
          }, workflowId, undefined, undefined);
          processedEffects++;
        } else {
          deps.logger.error("effect failed", {
            workflowId,
            effectId: effect.effectId,
            error: res.error ?? "unknown"
          });
        }
      }

      const dueTasks = projection.pendingTasks.filter((task) => Date.parse(task.dueAtIso) <= Date.parse(deps.clock.nowIso()));
      for (const task of dueTasks) {
        if (results.length >= max || processedTasks >= max) break;
        const res = await handleLocked(
          deps,
          {
            eventId: task.taskId,
            workflowId,
            type: task.type,
            occurredAt: deps.clock.nowIso(),
            schemaVersion: deps.spec.schemaVersion,
            payload: task.payload
          }
        );
        results.push(res);
        if (res.rejected && res.reason === "store-append-failed") continue;
        await appendRecordCompat(deps.store, {
          kind: "task-acked",
          workflowId,
          commitId: task.commitId,
          taskId: task.taskId,
          recordedAt: deps.clock.nowIso()
        }, workflowId, undefined, undefined);
        processedTasks++;
      }
    });
  }

  return { processedEffects, processedTasks, results };
}

async function handleLocked(deps: EngineDeps, input: InputEnvelope): Promise<HandleResult> {
  const started = Date.now();
  deps.metrics.inc("wf.input.received");

  const projection = await loadProjectionCompat(deps.store, input.workflowId);
  const instance = projection?.instance ?? createInitialInstance(deps.spec, input.workflowId);

  if (instance.specId !== deps.spec.specId || instance.specVersion !== deps.spec.specVersion) {
    const res = await appendRejection(deps, instance, input, "spec-mismatch", "spec mismatch");
    deps.metrics.inc("wf.input.rejected", { reason: "spec_mismatch" });
    deps.logger.warn("spec mismatch", {
      workflowId: input.workflowId,
      instanceSpecId: instance.specId,
      instanceSpecVersion: instance.specVersion,
      engineSpecId: deps.spec.specId,
      engineSpecVersion: deps.spec.specVersion
    });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return res;
  }

  const seenAt = projection?.seenEventIds.get(input.eventId);
  if (seenAt && withinTtl(seenAt, deps.clock.nowIso(), IDEMPOTENCY_TTL_MS)) {
    deps.metrics.inc("wf.input.deduped");
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return { deduped: true };
  }

  if (byteLengthJson(input.payload) > deps.spec.limits.maxPayloadBytes) {
    const res = await appendRejection(deps, instance, input, "payload-too-large", "payload too large");
    deps.metrics.inc("wf.input.rejected", { reason: "payload_too_large" });
    deps.logger.warn("payload exceeds limit", { workflowId: input.workflowId });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return res;
  }

  const quota = await deps.quota.check(input);
  if (!quota.allowed) {
    const res = await appendRejection(deps, instance, input, quota.reason ?? "quota", quota.reason ?? "quota");
    deps.metrics.inc("wf.input.rejected", { reason: quota.reason ?? "quota" });
    deps.logger.warn("input rejected by quota", { workflowId: input.workflowId, reason: quota.reason });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return res;
  }

  const decision = decide(deps.spec, instance, input);
  if (decision.rejection) {
    const res = await appendRejection(deps, instance, input, decision.rejection.reason, decision.rejection.message);
    deps.metrics.inc("wf.input.rejected", { reason: decision.rejection.reason });
    deps.logger.warn("decision rejected", { workflowId: input.workflowId, reason: decision.rejection.reason });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return res;
  }

  const commitAt = deps.clock.nowIso();
  const nextContext = applyContextPatch(instance.context, decision.contextPatch);
  if (byteLengthJson(nextContext) > deps.spec.limits.maxContextBytes) {
    const res = await appendRejection(deps, instance, input, "context-too-large", "context too large");
    deps.metrics.inc("wf.input.rejected", { reason: "context_too_large" });
    deps.logger.warn("context exceeds limit", { workflowId: input.workflowId });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return res;
  }

  const updated: Instance = {
    ...instance,
    state: decision.transitionTaken!.to,
    context: nextContext,
    version: instance.version + 1
  };

  const commitId = randomUUID();
  const effects = buildPendingEffects(input, commitId, commitAt, decision.effects);
  const tasks = buildPendingTasks(deps.spec, input, commitId, commitAt, decision.nextInputs, updated.state);
  const record: WorkflowJournalRecord = {
    kind: "commit",
    workflowId: input.workflowId,
    commitId,
    eventId: input.eventId,
    recordedAt: commitAt,
    input,
    decision,
    nextInstance: updated,
    effects,
    tasks
  };

  try {
    await appendRecordCompat(deps.store, record, input.workflowId, instance, input);
  } catch (err) {
    deps.metrics.inc("wf.store.conflict");
    deps.logger.warn("store append failed", { workflowId: input.workflowId, error: String(err) });
    deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
    return { rejected: true, reason: "store-append-failed" };
  }

  deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
  return {
    committed: true,
    decision,
    instance: updated,
    queuedEffects: effects.length,
    queuedTasks: tasks.length
  };
}

async function appendRejection(
  deps: EngineDeps,
  instance: Instance,
  input: InputEnvelope,
  reason: string,
  message?: string
): Promise<HandleResult> {
  const commitId = randomUUID();
  const recordedAt = deps.clock.nowIso();
  const record: WorkflowJournalRecord = {
    kind: "rejection",
    workflowId: input.workflowId,
    commitId,
    eventId: input.eventId,
    recordedAt,
    input: {
      workflowId: input.workflowId,
      eventId: input.eventId,
      type: input.type,
      schemaVersion: input.schemaVersion,
      tenantId: input.tenantId,
      actor: input.actor
    },
    reason,
    message,
    nextInstance: instance
  };

  try {
    await appendRecordCompat(deps.store, record, input.workflowId, instance, input);
  } catch (err) {
    deps.logger.error("rejection append failed", { workflowId: input.workflowId, error: String(err) });
    return { rejected: true, reason: "store-append-failed" };
  }

  return { rejected: true, reason };
}

function createInitialInstance(spec: Spec, workflowId: string): Instance {
  return {
    workflowId,
    specId: spec.specId,
    specVersion: spec.specVersion,
    state: spec.initialState,
    context: {},
    version: 0,
    status: "RUNNING"
  };
}

function byteLengthJson(v: unknown): number {
  try {
    return Buffer.byteLength(JSON.stringify(v), "utf8");
  } catch {
    return Number.MAX_SAFE_INTEGER;
  }
}

function withinTtl(recordedAtIso: string, nowIso: string, ttlMs: number): boolean {
  const recordedAt = Date.parse(recordedAtIso);
  const now = Date.parse(nowIso);
  if (!Number.isFinite(recordedAt) || !Number.isFinite(now)) return false;
  return now - recordedAt <= ttlMs;
}

async function loadProjectionCompat(store: WorkflowStore, workflowId: string): Promise<WorkflowProjection | null> {
  const loaded = (await store.load(workflowId)) as any;
  if (!loaded) return null;
  if (loaded.instance !== undefined && loaded.seenEventIds !== undefined) return loaded as WorkflowProjection;
  return {
    instance: loaded as Instance,
    seenEventIds: new Map<string, string>(),
    pendingEffects: [],
    pendingTasks: []
  };
}

async function appendRecordCompat(
  store: WorkflowStore,
  record: WorkflowJournalRecord,
  workflowId: string,
  instance?: Instance,
  input?: InputEnvelope
): Promise<void> {
  const anyStore = store as any;
  if (typeof anyStore.append === "function") {
    await anyStore.append(record);
    return;
  }
  if (record.kind === "commit" && typeof anyStore.save === "function" && typeof anyStore.appendHistory === "function") {
    await anyStore.save(record.nextInstance, instance?.version ?? 0);
    await anyStore.appendHistory(workflowId, { input: input!, decision: record.decision });
    return;
  }
  if (record.kind === "rejection" && typeof anyStore.appendHistory === "function") {
    await anyStore.appendHistory(workflowId, {
      input: input!,
      decision: { effects: [], nextInputs: [], rejection: { reason: "INVALID_INPUT", message: record.message } }
    });
    return;
  }
  throw new Error("unsupported workflow store");
}

function applyContextPatch(context: Instance["context"], patch: any | undefined): Instance["context"] {
  if (!patch) return context;
  const anyPatch = patch as any;
  const base = deepCloneJson(context);

  if (anyPatch.set) {
    for (const k of Object.keys(anyPatch.set)) {
      setDot(base, k, anyPatch.set[k]);
    }
  }
  if (anyPatch.unset) {
    for (const k of anyPatch.unset as string[]) unsetDot(base, k);
  }
  return base;
}

function buildPendingEffects(input: InputEnvelope, commitId: string, recordedAt: string, effects: ReturnType<typeof decide>["effects"]): PendingEffect[] {
  return effects.map((effect) => ({
    ...effect,
    workflowId: input.workflowId,
    commitId,
    recordedAt
  }));
}

function buildPendingTasks(
  spec: Spec,
  input: InputEnvelope,
  commitId: string,
  recordedAt: string,
  nextInputs: NextInputTemplate[],
  state: string
): PendingTask[] {
  const tasks: PendingTask[] = [];
  for (let i = 0; i < nextInputs.length; i++) {
    const ni = nextInputs[i]!;
    const dueAtIso = ni.delayMs !== undefined ? new Date(Date.parse(recordedAt) + ni.delayMs).toISOString() : recordedAt;
    tasks.push({
      taskId: `${commitId}:next:${i}`,
      workflowId: input.workflowId,
      commitId,
      sourceEventId: input.eventId,
      kind: "nextInput",
      dueAtIso,
      type: ni.type,
      payload: ni.payload
    });
  }

  const timeouts = spec.timeouts ?? [];
  for (let i = 0; i < timeouts.length; i++) {
    const timeout = timeouts[i]!;
    if (timeout.inState !== state) continue;
    tasks.push({
      taskId: `${commitId}:timeout:${i}`,
      workflowId: input.workflowId,
      commitId,
      sourceEventId: input.eventId,
      kind: "timeout",
      dueAtIso: new Date(Date.parse(recordedAt) + timeout.afterMs).toISOString(),
      type: timeout.emit.type,
      payload: timeout.emit.payload
    });
  }

  return tasks;
}

function deepCloneJson(v: any): any {
  return JSON.parse(JSON.stringify(v));
}

function setDot(obj: any, path: string, value: any) {
  const parts = path.startsWith("context.") ? path.slice("context.".length).split(".") : path.split(".");
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i]!;
    if (typeof cur[p] !== "object" || cur[p] === null || Array.isArray(cur[p])) cur[p] = {};
    cur = cur[p];
  }
  cur[parts[parts.length - 1]!] = value;
}

function unsetDot(obj: any, path: string) {
  const parts = path.startsWith("context.") ? path.slice("context.".length).split(".") : path.split(".");
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i]!;
    if (typeof cur[p] !== "object" || cur[p] === null) return;
    cur = cur[p];
  }
  delete cur[parts[parts.length - 1]!];
}
