#!/usr/bin/env node
import { readFile } from "node:fs/promises";
import {
  AllowAllQuotaLimiter,
  ConsoleLogger,
  FileIdempotencyStore,
  FileWorkflowStore,
  InMemorySequencer,
  NoopEffectExecutor,
  NoopMetrics,
  decide,
  drainPendingWork,
  handle,
  lintSpec,
  validateSpec
} from "../v2/index.ts";
import type { InputEnvelope, Instance } from "../v1/core/spec.ts";

async function main() {
  const [cmd, ...args] = process.argv.slice(2);
  if (!cmd || cmd === "help" || cmd === "--help" || cmd === "-h") {
    console.error("Usage:");
    console.error("  wf validate --spec <path>");
    console.error("  wf lint --spec <path>");
    console.error("  wf simulate --spec <path> --inputs <jsonl>");
    console.error("  wf run --spec <path> --input <json> --store <dir>");
    console.error("  wf inspect --workflow-id <id> --store <dir>");
    console.error("  wf tick --spec <path> --store <dir> [--max N]");
    process.exit(cmd ? 0 : 2);
  }

  if (cmd === "validate" || cmd === "lint" || cmd === "simulate" || cmd === "run") {
    const specPath = readArg(args, "--spec");
    if (!specPath) die("Missing --spec <path>");
    const spec = await loadSpec(specPath);
    const vr = validateSpec(spec);
    if (vr.issues.length > 0) printIssues(vr.issues);
    if (vr.issues.some((i) => i.level === "error")) process.exit(1);
    const typedSpec = vr.spec!;

    if (cmd === "validate") {
      console.log("OK");
      return;
    }

    if (cmd === "lint") {
      const lr = lintSpec(typedSpec);
      if (lr.issues.length > 0) printIssues(lr.issues);
      console.log(`riskScore=${lr.riskScore}`);
      process.exit(lr.issues.some((i) => i.level === "error") ? 1 : 0);
      return;
    }

    if (cmd === "simulate") {
      const inputsPath = readArg(args, "--inputs");
      if (!inputsPath) die("Missing --inputs <jsonl>");
      const inputs = await loadInputs(inputsPath);
      let instance: Instance = {
        workflowId: inputs[0]?.workflowId ?? "SIM",
        specId: typedSpec.specId,
        specVersion: typedSpec.specVersion,
        state: typedSpec.initialState,
        context: {},
        version: 0,
        status: "RUNNING"
      };
      for (const input of inputs) {
        const decision = decide(typedSpec, instance, input);
        console.log(JSON.stringify({ state: instance.state, input: input.type, decision }, null, 2));
        if (decision.rejection) break;
        instance = {
          ...instance,
          state: decision.transitionTaken!.to,
          context: applyContextPatch(instance.context, decision.contextPatch),
          version: instance.version + 1
        };
      }
      return;
    }

    if (cmd === "run") {
      const inputPath = readArg(args, "--input");
      const storeDir = readArg(args, "--store");
      if (!inputPath) die("Missing --input <path>");
      if (!storeDir) die("Missing --store <dir>");
      const input = JSON.parse(await readFile(inputPath, "utf8")) as InputEnvelope;
      const res = await handle(
        {
          spec: typedSpec,
          store: new FileWorkflowStore(storeDir),
          idempotency: new FileIdempotencyStore(storeDir),
          effects: new NoopEffectExecutor(),
          quota: new AllowAllQuotaLimiter(),
          sequencer: new InMemorySequencer(),
          scheduler: new FileScheduler(storeDir),
          clock: { nowIso: () => new Date().toISOString() },
          logger: new ConsoleLogger(),
          metrics: new NoopMetrics()
        },
        input
      );
      console.log(JSON.stringify(res, null, 2));
      return;
    }
  }

  if (cmd === "inspect") {
    const workflowId = readArg(args, "--workflow-id");
    const storeDir = readArg(args, "--store");
    if (!workflowId) die("Missing --workflow-id <id>");
    if (!storeDir) die("Missing --store <dir>");
    const store = new FileWorkflowStore(storeDir);
    const inst = await store.load(workflowId);
    console.log(JSON.stringify(inst, null, 2));
    process.exit(inst ? 0 : 1);
    return;
  }

  if (cmd === "tick") {
    const specPath = readArg(args, "--spec");
    const storeDir = readArg(args, "--store");
    const maxStr = readArg(args, "--max");
    const max = maxStr ? Number(maxStr) : 100;
    if (!specPath) die("Missing --spec <path>");
    if (!storeDir) die("Missing --store <dir>");
    if (!Number.isFinite(max) || max <= 0) die("--max must be a positive number");

    const vr = validateSpec(await loadSpec(specPath));
    if (vr.issues.length > 0) printIssues(vr.issues);
    if (vr.issues.some((i) => i.level === "error")) process.exit(1);
    const typedSpec = vr.spec!;

      const store = new FileWorkflowStore(storeDir);
      const deps = {
        spec: typedSpec,
        store,
        effects: new NoopEffectExecutor(),
        quota: new AllowAllQuotaLimiter(),
        sequencer: new InMemorySequencer(),
        clock: { nowIso: () => new Date().toISOString() },
        logger: new ConsoleLogger(),
        metrics: new NoopMetrics()
      };

      const results = await drainPendingWork(deps, max);
      console.log(JSON.stringify(results, null, 2));
      return;
    }

  console.error(`Unknown command: ${cmd}`);
  process.exit(2);
}

function readArg(args: string[], name: string): string | undefined {
  const idx = args.indexOf(name);
  return idx >= 0 ? args[idx + 1] : undefined;
}

function die(msg: string): never {
  console.error(msg);
  process.exit(2);
}

async function loadSpec(specPath: string): Promise<unknown> {
  const raw = await readFile(specPath, "utf8");
  return JSON.parse(raw) as unknown;
}

async function loadInputs(path: string): Promise<InputEnvelope[]> {
  const raw = await readFile(path, "utf8");
  const out: InputEnvelope[] = [];
  for (const line of raw.split("\n")) {
    const t = line.trim();
    if (!t) continue;
    out.push(JSON.parse(t) as InputEnvelope);
  }
  return out;
}

function printIssues(issues: Array<{ level: string; code: string; message: string; path?: string }>) {
  for (const issue of issues) {
    console.error(`${issue.level.toUpperCase()} ${issue.code}${issue.path ? ` (${issue.path})` : ""}: ${issue.message}`);
  }
}

function applyContextPatch(context: unknown, patch: unknown): unknown {
  if (!patch || typeof patch !== "object") return context;
  const base = JSON.parse(JSON.stringify(context));
  const anyPatch = patch as { set?: Record<string, unknown>; unset?: string[] };
  if (anyPatch.set) {
    for (const [k, v] of Object.entries(anyPatch.set)) setDot(base, k, v);
  }
  if (anyPatch.unset) {
    for (const k of anyPatch.unset) unsetDot(base, k);
  }
  return base;
}

function setDot(obj: any, path: string, value: unknown) {
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

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
