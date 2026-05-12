import { randomUUID } from "node:crypto";
import type { InputEnvelope, Instance, NextInputTemplate, Spec, TimeoutRule } from "../core/spec.ts";
import type { Clock, EffectExecutor, IdempotencyStore, Logger, Metrics, QuotaLimiter, Scheduler, Sequencer, WorkflowStore } from "./ports.ts";
import { decide } from "../core/decide.ts";
import { withSpan } from "../../tracing.ts";

export interface EngineDeps {
  spec: Spec;
  store: WorkflowStore;
  idempotency: IdempotencyStore;
  effects: EffectExecutor;
  quota: QuotaLimiter;
  sequencer: Sequencer;
  scheduler: Scheduler;
  clock: Clock;
  logger: Logger;
  metrics: Metrics;
}

export async function handle(deps: EngineDeps, input: InputEnvelope) {
  const started = Date.now();
  return deps.sequencer.runExclusive(input.workflowId, async () =>
    withSpan("engine.handle", { workflowId: input.workflowId, eventId: input.eventId }, async () => {
      deps.metrics.inc("wf.input.received");

    if (byteLengthJson(input.payload) > deps.spec.limits.maxPayloadBytes) {
      deps.metrics.inc("wf.input.rejected", { reason: "payload_too_large" });
      deps.logger.warn("payload exceeds limit", { workflowId: input.workflowId });
      return { rejected: true, reason: "payload-too-large" };
    }

    const quota = await deps.quota.check(input);
    if (!quota.allowed) {
      deps.metrics.inc("wf.input.rejected", { reason: quota.reason ?? "quota" });
      deps.logger.warn("input rejected by quota", { workflowId: input.workflowId, reason: quota.reason });
      return { rejected: true, reason: quota.reason ?? "quota" };
    }

    const { seen } = await deps.idempotency.checkAndMark(input.eventId, { ttlSeconds: 60 * 60 * 24 * 7 });
    if (seen) {
      deps.metrics.inc("wf.input.deduped");
      return { deduped: true };
    }

    const instance = (await deps.store.load(input.workflowId)) ?? createInitialInstance(deps.spec, input.workflowId);
    if (instance.specId !== deps.spec.specId || instance.specVersion !== deps.spec.specVersion) {
      deps.metrics.inc("wf.input.rejected", { reason: "spec_mismatch" });
      deps.logger.warn("spec mismatch", {
        workflowId: input.workflowId,
        instanceSpecId: instance.specId,
        instanceSpecVersion: instance.specVersion,
        engineSpecId: deps.spec.specId,
        engineSpecVersion: deps.spec.specVersion
      });
      return { rejected: true, reason: "spec-mismatch" };
    }
    const decision = decide(deps.spec, instance, input);

    await deps.store.appendHistory(input.workflowId, { input, decision });

    if (decision.rejection) {
      deps.metrics.inc("wf.input.rejected", { reason: decision.rejection.reason });
      deps.logger.warn("decision rejected", { workflowId: input.workflowId, reason: decision.rejection.reason });
      deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
      return { decision };
    }

    const nextVersion = instance.version + 1;
    const nextContext = applyContextPatch(instance.context, decision.contextPatch);
    if (byteLengthJson(nextContext) > deps.spec.limits.maxContextBytes) {
      deps.metrics.inc("wf.input.rejected", { reason: "context_too_large" });
      deps.logger.warn("context exceeds limit", { workflowId: input.workflowId });
      return { decision: { effects: [], nextInputs: [], rejection: { reason: "INVALID_INPUT", message: "context too large" } } };
    }
    const updated: Instance = {
      ...instance,
      state: decision.transitionTaken!.to,
      context: nextContext,
      version: nextVersion
    };

    try {
      await deps.store.save(updated, instance.version);
    } catch (err) {
      deps.metrics.inc("wf.store.conflict");
      deps.logger.warn("store save failed", { workflowId: input.workflowId, error: String(err) });
      return { rejected: true, reason: "store-save-failed" };
    }

    for (const eff of decision.effects) {
      try {
        const res = await deps.effects.execute(eff);
        deps.metrics.inc("wf.effect.executed", { type: eff.type, ok: String(res.ok) });
        if (!res.ok) deps.logger.error("effect failed", { workflowId: input.workflowId, effectId: eff.effectId, error: res.error });
      } catch (err) {
        deps.metrics.inc("wf.effect.executed", { type: eff.type, ok: "false" });
        deps.logger.error("effect threw", { workflowId: input.workflowId, effectId: eff.effectId, error: String(err) });
      }
    }

    // Schedule follow-up inputs from transition.
    try {
      await scheduleNextInputs(deps, input.workflowId, input, decision.nextInputs);
    } catch (err) {
      deps.metrics.inc("wf.scheduler.error");
      deps.logger.error("scheduling next inputs failed", { workflowId: input.workflowId, error: String(err) });
    }

    // Schedule timeouts for the new state.
    try {
      await scheduleTimeoutsForState(deps, input.workflowId, updated.state);
    } catch (err) {
      deps.metrics.inc("wf.scheduler.error");
      deps.logger.error("scheduling timeouts failed", { workflowId: input.workflowId, error: String(err) });
    }

      deps.metrics.observeMs("wf.input.latency_ms", Date.now() - started);
      return { decision, instance: updated };
    })
  );
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
  // JSON.stringify is deterministic enough for sizing here; if it throws, treat as huge.
  try {
    return Buffer.byteLength(JSON.stringify(v), "utf8");
  } catch {
    return Number.MAX_SAFE_INTEGER;
  }
}

function applyContextPatch(context: Instance["context"], patch: any): Instance["context"] {
  if (!patch) return context;
  const base = deepCloneJson(context);

  if (patch.set) {
    for (const k of Object.keys(patch.set)) {
      setDot(base, k, patch.set[k]);
    }
  }
  if (patch.unset) {
    for (const k of patch.unset) unsetDot(base, k);
  }
  return base;
}

async function scheduleNextInputs(deps: EngineDeps, workflowId: string, input: InputEnvelope, nextInputs: NextInputTemplate[]) {
  for (let i = 0; i < nextInputs.length; i++) {
    const ni = nextInputs[i]!;
    const due = ni.delayMs ? new Date(Date.now() + ni.delayMs).toISOString() : deps.clock.nowIso();
    await deps.scheduler.schedule({
      eventId: randomUUID(),
      workflowId,
      dueAtIso: due,
      type: ni.type,
      payload: ni.payload,
      dedupeKey: `${input.eventId}:next:${i}`
    });
    deps.metrics.inc("wf.next_input.scheduled", { type: ni.type });
  }
}

async function scheduleTimeoutsForState(deps: EngineDeps, workflowId: string, state: string) {
  const rules = deps.spec.timeouts ?? [];
  const now = Date.now();
  for (let i = 0; i < rules.length; i++) {
    const r: TimeoutRule = rules[i]!;
    if (r.inState !== state) continue;
    const dueAtIso = new Date(now + r.afterMs).toISOString();
    await deps.scheduler.schedule({
      eventId: randomUUID(),
      workflowId,
      dueAtIso,
      type: r.emit.type,
      payload: r.emit.payload,
      dedupeKey: `timeout:${state}:${r.afterMs}:${r.emit.type}`
    });
    deps.metrics.inc("wf.timeout.scheduled", { type: r.emit.type });
  }
}


function deepCloneJson(v: any): any {
  return JSON.parse(JSON.stringify(v));
}

function setDot(obj: any, path: string, value: any) {
  const parts = path.split(".");
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i]!;
    if (typeof cur[p] !== "object" || cur[p] === null || Array.isArray(cur[p])) cur[p] = {};
    cur = cur[p];
  }
  cur[parts[parts.length - 1]!] = value;
}

function unsetDot(obj: any, path: string) {
  const parts = path.split(".");
  let cur = obj;
  for (let i = 0; i < parts.length - 1; i++) {
    const p = parts[i]!;
    if (typeof cur[p] !== "object" || cur[p] === null) return;
    cur = cur[p];
  }
  delete cur[parts[parts.length - 1]!];
}
