import { randomUUID } from "node:crypto";
import { join } from "node:path";
import type { InputEnvelope, Instance } from "../v1/core/spec.ts";
import { validateSpec as validateV1 } from "../v1/core/validateSpec.ts";
import { lintSpec as lintV1 } from "../v1/core/lintSpec.ts";
import { decide as decideV1 } from "../v1/core/decide.ts";
import { handle as handleV1 } from "../v1/runtime/engine.ts";
import { FileWorkflowStore as V1FileWorkflowStore } from "../v1/adapters/store-file/storeFile.ts";
import { FileIdempotencyStore as V1FileIdempotencyStore } from "../v1/adapters/idempotency-file/idempotencyFile.ts";
import { FileScheduler as V1FileScheduler } from "../v1/adapters/scheduler-file/schedulerFile.ts";
import { NoopEffectExecutor as V1NoopEffectExecutor } from "../v1/adapters/effects/noopEffects.ts";
import { AllowAllQuotaLimiter as V1AllowAllQuotaLimiter } from "../v1/adapters/quota/allowAllQuota.ts";
import { ConsoleLogger as V1ConsoleLogger } from "../v1/adapters/telemetry/consoleLogger.ts";
import { NoopMetrics as V1NoopMetrics } from "../v1/adapters/telemetry/noopMetrics.ts";
import { InMemorySequencer as V1InMemorySequencer } from "../v1/runtime/inMemorySequencer.ts";
import type { ValidationResult } from "../v1/core/validateSpec.ts";

import { validateSpec as validateV2 } from "../v2/core/validateSpec.ts";
import { lintSpec as lintV2 } from "../v2/core/lintSpec.ts";
import { decide as decideV2 } from "../v2/core/decide.ts";
import { handle as handleV2 } from "../v2/runtime/engine.ts";
import { FileWorkflowStore as V2FileWorkflowStore } from "../v2/adapters/store-file/storeFile.ts";
import { NoopEffectExecutor as V2NoopEffectExecutor } from "../v2/adapters/effects/noopEffects.ts";
import { AllowAllQuotaLimiter as V2AllowAllQuotaLimiter } from "../v2/adapters/quota/allowAllQuota.ts";
import { ConsoleLogger as V2ConsoleLogger } from "../v2/adapters/telemetry/consoleLogger.ts";
import { NoopMetrics as V2NoopMetrics } from "../v2/adapters/telemetry/noopMetrics.ts";
import { InMemorySequencer as V2InMemorySequencer } from "../v2/runtime/inMemorySequencer.ts";
import type { ValidationResult as ValidationResultV2 } from "../v2/core/validateSpec.ts";
import { logExecution } from "./execLog.ts";
import type { EngineVersion, VersionedRunRequest, VersionedSimulateRequest, VersionedSpecRequest } from "./contracts.ts";
import { currentTraceId, withSpan } from "../tracing.ts";
import { PgWorkflowStore } from "./workflowStorePg.ts";

export interface RuntimeResult {
  traceId: string;
  version: EngineVersion;
  body: unknown;
}

type ValidateResult = ValidationResult | ValidationResultV2;
type EngineResult = {
  committed?: true;
  deduped?: true;
  rejected?: true;
  reason?: string;
};

function dataRoot(baseDir: string, version: EngineVersion): string {
  return join(baseDir, `v${version}`);
}

function nowIso(): string {
  return new Date().toISOString();
}

function createInstance(spec: any, workflowId: string): Instance {
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

function buildV1Deps(spec: any, baseDir: string) {
  const dir = dataRoot(baseDir, 1);
  return {
    spec,
    store: new V1FileWorkflowStore(dir),
    idempotency: new V1FileIdempotencyStore(dir),
    effects: new V1NoopEffectExecutor(),
    quota: new V1AllowAllQuotaLimiter(),
    sequencer: new V1InMemorySequencer(),
    scheduler: new V1FileScheduler(dir),
    clock: { nowIso },
    logger: new V1ConsoleLogger(),
    metrics: new V1NoopMetrics()
  };
}

function buildV2Deps(spec: any, baseDir: string) {
  const databaseUrl = process.env.DATABASE_URL?.trim() ?? "";
  return {
    spec,
    store: databaseUrl ? new PgWorkflowStore(databaseUrl) : new V2FileWorkflowStore(dataRoot(baseDir, 2)),
    effects: new V2NoopEffectExecutor(),
    quota: new V2AllowAllQuotaLimiter(),
    sequencer: new V2InMemorySequencer(),
    clock: { nowIso },
    logger: new V2ConsoleLogger(),
    metrics: new V2NoopMetrics()
  };
}

export function validateVersionedSpec(version: EngineVersion, spec: unknown): ValidateResult {
  return version === 1 ? validateV1(spec) : validateV2(spec);
}

export function lintVersionedSpec(version: EngineVersion, spec: any) {
  return version === 1 ? lintV1(spec) : lintV2(spec);
}

export function decideVersioned(version: EngineVersion, spec: any, instance: Instance, input: InputEnvelope) {
  return version === 1 ? decideV1(spec, instance, input) : decideV2(spec, instance, input);
}

export async function simulateVersioned(
  version: EngineVersion,
  spec: any,
  inputs: InputEnvelope[],
  workflowId?: string
): Promise<{ instance: Instance; decisions: unknown[] }> {
  const wfId = workflowId ?? inputs[0]?.workflowId ?? `sim-${randomUUID()}`;
  let instance = createInstance(spec, wfId);
  const decisions: unknown[] = [];
  await withSpan("engine.simulate.loop", { version, workflowId: wfId, "input.count": inputs.length }, async () => {
    for (const input of inputs) {
      const decision = await withSpan("engine.decide", { version, workflowId: wfId, eventId: input.eventId }, async () =>
        decideVersioned(version, spec, instance, input)
      );
      decisions.push(decision);
      if (decision.rejection) break;
      instance = {
        ...instance,
        state: decision.transitionTaken!.to,
        context: applyContextPatch(instance.context, decision.contextPatch),
        version: instance.version + 1
      };
    }
  });
  return { instance, decisions };
}

export async function runVersioned(
  version: EngineVersion,
  request: VersionedRunRequest,
  baseDir: string
): Promise<RuntimeResult> {
  const traceId = currentTraceId() ?? randomUUID();
  const validated = await withSpan("spec.validate", { version, traceId }, async () => validateVersionedSpec(version, request.spec));
  if (validated.issues?.some((issue: { level: string }) => issue.level === "error")) {
    return { traceId, version, body: { rejected: true, reason: "invalid_spec", validation: validated } };
  }

  const spec = (validated as any).spec!;
  const input = request.input;
  const route = `/v${version}/run`;
  const workflowId = request.workflowId ?? input.workflowId;
  if (!workflowId) {
    return {
      traceId,
      version,
      body: { rejected: true, reason: "missing_workflow_id" }
    };
  }

  const normalizedInput = { ...input, workflowId };
  const result = (await withSpan("engine.dispatch", { version, traceId, workflowId, eventId: normalizedInput.eventId }, async () =>
    version === 1 ? handleV1(buildV1Deps(spec, baseDir), normalizedInput) : handleV2(buildV2Deps(spec, baseDir), normalizedInput)
  )) as EngineResult;

  await withSpan("audit.write", { version, traceId, workflowId, eventId: normalizedInput.eventId }, async () =>
    logExecution({
      traceId,
      serviceVersion: "v3",
      route,
      engineVersion: version,
      workflowId,
      eventId: normalizedInput.eventId,
      specId: spec.specId,
      specVersion: spec.specVersion,
      outcome: result.committed ? "committed" : result.deduped ? "deduped" : "rejected",
      ...(result.reason ? { reason: result.reason } : {}),
      requestJson: sanitizeRequestForAudit(request, normalizedInput, spec),
      responseJson: sanitizeResponseForAudit(result)
    })
  );

  return { traceId, version, body: result };
}

function applyContextPatch(context: unknown, patch: unknown): Instance["context"] {
  if (!patch || typeof patch !== "object") return context as Instance["context"];
  const base = JSON.parse(JSON.stringify(context));
  const anyPatch = patch as { set?: Record<string, unknown>; unset?: string[] };
  if (anyPatch.set) {
    for (const [k, v] of Object.entries(anyPatch.set)) setDot(base, k, v);
  }
  if (anyPatch.unset) {
    for (const k of anyPatch.unset) unsetDot(base, k);
  }
  return base as Instance["context"];
}

function setDot(obj: any, path: string, value: unknown) {
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

function sanitizeRequestForAudit(request: VersionedRunRequest, input: InputEnvelope, spec: unknown): unknown {
  return {
    version: request.version,
    workflowId: request.workflowId ?? input.workflowId ?? null,
    input: {
      eventId: input.eventId,
      workflowId: input.workflowId,
      type: input.type,
      occurredAt: input.occurredAt,
      schemaVersion: input.schemaVersion,
      tenantId: input.tenantId ?? null,
      actor: input.actor ?? null,
      payload: "[redacted]"
    },
    spec: summarizeSpec(spec)
  };
}

function sanitizeResponseForAudit(result: unknown): unknown {
  if (!result || typeof result !== "object") return result;
  const anyResult = result as { committed?: boolean; deduped?: boolean; rejected?: boolean; reason?: string; queuedEffects?: number; queuedTasks?: number };
  return {
    committed: Boolean(anyResult.committed),
    deduped: Boolean(anyResult.deduped),
    rejected: Boolean(anyResult.rejected),
    ...(anyResult.reason ? { reason: anyResult.reason } : {}),
    ...(typeof anyResult.queuedEffects === "number" ? { queuedEffects: anyResult.queuedEffects } : {}),
    ...(typeof anyResult.queuedTasks === "number" ? { queuedTasks: anyResult.queuedTasks } : {})
  };
}

function summarizeSpec(spec: unknown): unknown {
  if (!spec || typeof spec !== "object") return "[redacted]";
  const anySpec = spec as { specId?: string; specVersion?: number; schemaVersion?: number };
  return {
    specId: anySpec.specId ?? null,
    specVersion: anySpec.specVersion ?? null,
    schemaVersion: anySpec.schemaVersion ?? null
  };
}
