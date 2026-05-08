import type { ContextPatch, EffectTemplate, Json, Limits, NextInputTemplate, Permissions, Predicate, Spec, TimeoutRule, Transition } from "./spec.ts";

export interface ValidationIssue {
  level: "error" | "warning";
  code: string;
  message: string;
  path?: string;
}

export interface ValidationResult {
  issues: ValidationIssue[];
  spec?: Spec;
}

function isObject(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function asString(v: unknown): string | null {
  return typeof v === "string" ? v : null;
}

function asNumber(v: unknown): number | null {
  return typeof v === "number" && Number.isFinite(v) ? v : null;
}

function asStringArray(v: unknown): string[] | null {
  if (!Array.isArray(v)) return null;
  const out: string[] = [];
  for (let i = 0; i < v.length; i++) {
    const s = asString(v[i]);
    if (s === null) return null;
    out.push(s);
  }
  return out;
}

function validateLimits(v: unknown, issues: ValidationIssue[]): Limits | null {
  if (!isObject(v)) {
    issues.push({ level: "error", code: "LIMITS_NOT_OBJECT", message: "limits must be an object", path: "limits" });
    return null;
  }

  const required: Array<[keyof Limits, string]> = [
    ["maxEffectsPerDecision", "limits.maxEffectsPerDecision"],
    ["maxNextInputsPerDecision", "limits.maxNextInputsPerDecision"],
    ["maxContextBytes", "limits.maxContextBytes"],
    ["maxPayloadBytes", "limits.maxPayloadBytes"],
    ["maxGuardOps", "limits.maxGuardOps"]
  ];

  const out: Partial<Limits> = {};
  for (const [k, path] of required) {
    const n = asNumber(v[k as string]);
    if (n === null || n < 0) {
      issues.push({ level: "error", code: "LIMITS_INVALID", message: `${k} must be a non-negative number`, path });
      continue;
    }
    (out as any)[k] = n;
  }

  if (Object.keys(out).length !== required.length) return null;
  return out as Limits;
}

function validatePermissions(v: unknown, issues: ValidationIssue[]): Permissions | null {
  if (!isObject(v)) {
    issues.push({ level: "error", code: "PERMS_NOT_OBJECT", message: "permissions must be an object", path: "permissions" });
    return null;
  }

  const effectTypesAllowlist = asStringArray(v.effectTypesAllowlist);
  if (!effectTypesAllowlist) {
    issues.push({
      level: "error",
      code: "PERMS_ALLOWLIST_INVALID",
      message: "permissions.effectTypesAllowlist must be a string array",
      path: "permissions.effectTypesAllowlist"
    });
    return null;
  }

  const httpAllowedHosts = v.httpAllowedHosts === undefined ? undefined : asStringArray(v.httpAllowedHosts);
  if (v.httpAllowedHosts !== undefined && httpAllowedHosts === null) {
    issues.push({
      level: "error",
      code: "PERMS_HTTP_HOSTS_INVALID",
      message: "permissions.httpAllowedHosts must be a string array if provided",
      path: "permissions.httpAllowedHosts"
    });
    return null;
  }

  return { effectTypesAllowlist, httpAllowedHosts };
}

function isJson(v: unknown): v is Json {
  if (v === null) return true;
  const t = typeof v;
  if (t === "string" || t === "number" || t === "boolean") return true;
  if (Array.isArray(v)) return v.every(isJson);
  if (t === "object") {
    for (const k of Object.keys(v as any)) {
      if (!isJson((v as any)[k])) return false;
    }
    return true;
  }
  return false;
}

function validatePredicate(v: unknown, issues: ValidationIssue[], path: string): Predicate | null {
  if (!isObject(v)) {
    issues.push({ level: "error", code: "GUARD_NOT_OBJECT", message: "guard must be an object", path });
    return null;
  }
  const op = asString(v.op);
  if (!op) {
    issues.push({ level: "error", code: "GUARD_OP_INVALID", message: "guard.op must be a string", path: `${path}.op` });
    return null;
  }

  const p = path;
  switch (op) {
    case "and":
    case "or": {
      if (!Array.isArray(v.args)) {
        issues.push({ level: "error", code: "GUARD_ARGS_INVALID", message: "guard.args must be an array", path: `${p}.args` });
        return null;
      }
      const args: Predicate[] = [];
      for (let i = 0; i < v.args.length; i++) {
        const child = validatePredicate(v.args[i], issues, `${p}.args[${i}]`);
        if (!child) return null;
        args.push(child);
      }
      return { op: op as any, args };
    }
    case "not": {
      const child = validatePredicate(v.arg, issues, `${p}.arg`);
      if (!child) return null;
      return { op: "not", arg: child };
    }
    case "eq": {
      const pathStr = asString(v.path);
      if (!pathStr) {
        issues.push({ level: "error", code: "GUARD_PATH_INVALID", message: "guard.path must be a string", path: `${p}.path` });
        return null;
      }
      if (!isJson(v.value)) {
        issues.push({ level: "error", code: "GUARD_VALUE_INVALID", message: "guard.value must be JSON", path: `${p}.value` });
        return null;
      }
      return { op: "eq", path: pathStr, value: v.value };
    }
    case "in": {
      const pathStr = asString(v.path);
      if (!pathStr) {
        issues.push({ level: "error", code: "GUARD_PATH_INVALID", message: "guard.path must be a string", path: `${p}.path` });
        return null;
      }
      if (!Array.isArray(v.values) || !v.values.every(isJson)) {
        issues.push({ level: "error", code: "GUARD_VALUES_INVALID", message: "guard.values must be JSON array", path: `${p}.values` });
        return null;
      }
      return { op: "in", path: pathStr, values: v.values };
    }
    case "exists": {
      const pathStr = asString(v.path);
      if (!pathStr) {
        issues.push({ level: "error", code: "GUARD_PATH_INVALID", message: "guard.path must be a string", path: `${p}.path` });
        return null;
      }
      return { op: "exists", path: pathStr };
    }
    case "lt":
    case "lte":
    case "gt":
    case "gte": {
      const pathStr = asString(v.path);
      const val = asNumber(v.value);
      if (!pathStr) {
        issues.push({ level: "error", code: "GUARD_PATH_INVALID", message: "guard.path must be a string", path: `${p}.path` });
        return null;
      }
      if (val === null) {
        issues.push({ level: "error", code: "GUARD_NUM_INVALID", message: "guard.value must be a number", path: `${p}.value` });
        return null;
      }
      return { op: op as any, path: pathStr, value: val };
    }
    default:
      issues.push({ level: "error", code: "GUARD_OP_UNKNOWN", message: `unknown guard.op '${op}'`, path: `${p}.op` });
      return null;
  }
}

function validateEffectTemplates(v: unknown, issues: ValidationIssue[], path: string): EffectTemplate[] | null {
  if (!Array.isArray(v)) {
    issues.push({ level: "error", code: "EFFECTS_INVALID", message: "effects must be an array", path });
    return null;
  }
  const out: EffectTemplate[] = [];
  for (let i = 0; i < v.length; i++) {
    const e = v[i];
    const ep = `${path}[${i}]`;
    if (!isObject(e)) {
      issues.push({ level: "error", code: "EFFECT_NOT_OBJECT", message: "effect must be an object", path: ep });
      return null;
    }
    const type = asString(e.type);
    if (!type) {
      issues.push({ level: "error", code: "EFFECT_TYPE_INVALID", message: "effect.type must be a string", path: `${ep}.type` });
      return null;
    }
    if (!isJson(e.params)) {
      issues.push({ level: "error", code: "EFFECT_PARAMS_INVALID", message: "effect.params must be JSON", path: `${ep}.params` });
      return null;
    }
    out.push({ type, params: e.params });
  }
  return out;
}

function validateNextInputs(v: unknown, issues: ValidationIssue[], path: string): NextInputTemplate[] | null {
  if (!Array.isArray(v)) {
    issues.push({ level: "error", code: "NEXT_INPUTS_INVALID", message: "nextInputs must be an array", path });
    return null;
  }
  const out: NextInputTemplate[] = [];
  for (let i = 0; i < v.length; i++) {
    const ni = v[i];
    const np = `${path}[${i}]`;
    if (!isObject(ni)) {
      issues.push({ level: "error", code: "NEXT_INPUT_NOT_OBJECT", message: "nextInput must be an object", path: np });
      return null;
    }
    const type = asString(ni.type);
    if (!type) {
      issues.push({ level: "error", code: "NEXT_INPUT_TYPE_INVALID", message: "nextInput.type must be a string", path: `${np}.type` });
      return null;
    }
    if (!isJson(ni.payload)) {
      issues.push({ level: "error", code: "NEXT_INPUT_PAYLOAD_INVALID", message: "nextInput.payload must be JSON", path: `${np}.payload` });
      return null;
    }
    const delayMs = ni.delayMs === undefined ? undefined : asNumber(ni.delayMs);
    if (ni.delayMs !== undefined && (delayMs === null || delayMs < 0)) {
      issues.push({ level: "error", code: "NEXT_INPUT_DELAY_INVALID", message: "nextInput.delayMs must be a non-negative number", path: `${np}.delayMs` });
      return null;
    }
    out.push({ type, payload: ni.payload, delayMs: delayMs ?? undefined });
  }
  return out;
}

function isDotPath(s: string): boolean {
  if (s.length === 0) return false;
  const parts = s.split(".");
  return parts.every((p) => /^[A-Za-z0-9_]+$/.test(p));
}

function validateContextPatch(v: unknown, issues: ValidationIssue[], path: string): ContextPatch | null {
  if (!isObject(v)) {
    issues.push({ level: "error", code: "CTX_PATCH_INVALID", message: "contextPatch must be an object", path });
    return null;
  }
  const out: ContextPatch = {};
  if (v.set !== undefined) {
    if (!isObject(v.set)) {
      issues.push({ level: "error", code: "CTX_PATCH_SET_INVALID", message: "contextPatch.set must be an object", path: `${path}.set` });
      return null;
    }
    const set: Record<string, Json> = {};
    for (const k of Object.keys(v.set)) {
      if (!isDotPath(k)) {
        issues.push({ level: "error", code: "CTX_PATCH_KEY_INVALID", message: `invalid contextPatch.set key '${k}'`, path: `${path}.set` });
        return null;
      }
      const val = (v.set as any)[k];
      if (!isJson(val)) {
        issues.push({ level: "error", code: "CTX_PATCH_VAL_INVALID", message: `contextPatch.set['${k}'] must be JSON`, path: `${path}.set.${k}` });
        return null;
      }
      set[k] = val;
    }
    out.set = set;
  }
  if (v.unset !== undefined) {
    const unset = asStringArray(v.unset);
    if (!unset) {
      issues.push({ level: "error", code: "CTX_PATCH_UNSET_INVALID", message: "contextPatch.unset must be a string array", path: `${path}.unset` });
      return null;
    }
    for (const u of unset) {
      if (!isDotPath(u)) {
        issues.push({ level: "error", code: "CTX_PATCH_UNSET_PATH_INVALID", message: `invalid unset path '${u}'`, path: `${path}.unset` });
        return null;
      }
    }
    out.unset = unset;
  }
  return out;
}

export function validateSpec(spec: unknown): ValidationResult {
  const issues: ValidationIssue[] = [];
  if (!isObject(spec)) {
    issues.push({ level: "error", code: "SPEC_NOT_OBJECT", message: "spec must be an object" });
    return { issues };
  }

  const specId = asString(spec.specId);
  if (!specId) issues.push({ level: "error", code: "SPEC_ID_INVALID", message: "specId must be a string", path: "specId" });

  const specVersion = asNumber(spec.specVersion);
  if (specVersion === null) issues.push({ level: "error", code: "SPEC_VERSION_INVALID", message: "specVersion must be a number", path: "specVersion" });

  const schemaVersion = asNumber(spec.schemaVersion);
  if (schemaVersion === null) issues.push({ level: "error", code: "SCHEMA_VERSION_INVALID", message: "schemaVersion must be a number", path: "schemaVersion" });

  const initialState = asString(spec.initialState);
  if (!initialState) issues.push({ level: "error", code: "INITIAL_STATE_INVALID", message: "initialState must be a string", path: "initialState" });

  const terminalStates = asStringArray(spec.terminalStates);
  if (!terminalStates) issues.push({ level: "error", code: "TERMINAL_STATES_INVALID", message: "terminalStates must be a string array", path: "terminalStates" });

  const states = asStringArray(spec.states);
  if (!states) issues.push({ level: "error", code: "STATES_INVALID", message: "states must be a string array", path: "states" });

  const limits = validateLimits(spec.limits, issues);
  const permissions = validatePermissions(spec.permissions, issues);

  if (!Array.isArray(spec.transitions)) {
    issues.push({ level: "error", code: "TRANSITIONS_INVALID", message: "transitions must be an array", path: "transitions" });
  }

  if (issues.some((i) => i.level === "error")) return { issues };

  // Lightweight semantic checks
  if (states && initialState && !states.includes(initialState)) {
    issues.push({ level: "error", code: "INITIAL_STATE_UNKNOWN", message: "initialState must exist in states", path: "initialState" });
  }
  if (states && terminalStates) {
    for (let i = 0; i < terminalStates.length; i++) {
      const st = terminalStates[i]!;
      if (!states.includes(st)) {
        issues.push({ level: "error", code: "TERMINAL_STATE_UNKNOWN", message: `terminalStates includes unknown state '${st}'`, path: `terminalStates[${i}]` });
      }
    }
  }

  const transitions: Transition[] = [];
  const seen = new Set<string>();
  if (Array.isArray(spec.transitions) && states) {
    for (let i = 0; i < spec.transitions.length; i++) {
      const t = spec.transitions[i];
      const path = `transitions[${i}]`;
      if (!isObject(t)) {
        issues.push({ level: "error", code: "TRANSITION_NOT_OBJECT", message: "transition must be an object", path });
        continue;
      }
      const from = asString(t.from);
      const on = asString(t.on);
      const to = asString(t.to);
      if (!from || !on || !to) {
        issues.push({ level: "error", code: "TRANSITION_FIELDS_INVALID", message: "transition must have string from/on/to", path });
        continue;
      }
      if (!states.includes(from)) issues.push({ level: "error", code: "TRANSITION_FROM_UNKNOWN", message: `unknown from state '${from}'`, path: `${path}.from` });
      if (!states.includes(to)) issues.push({ level: "error", code: "TRANSITION_TO_UNKNOWN", message: `unknown to state '${to}'`, path: `${path}.to` });

      const k = `${from}::${on}`;
      if (seen.has(k)) {
        issues.push({ level: "error", code: "DUPLICATE_TRANSITION", message: `duplicate transition for (${from}, ${on})`, path });
        continue;
      }
      seen.add(k);

      const guard = t.guard === undefined ? undefined : validatePredicate(t.guard, issues, `${path}.guard`);
      const contextPatch = t.contextPatch === undefined ? undefined : validateContextPatch(t.contextPatch, issues, `${path}.contextPatch`);
      const effects = t.effects === undefined ? undefined : validateEffectTemplates(t.effects, issues, `${path}.effects`);
      const nextInputs = t.nextInputs === undefined ? undefined : validateNextInputs(t.nextInputs, issues, `${path}.nextInputs`);

      if (t.guard !== undefined && !guard) continue;
      if (t.contextPatch !== undefined && !contextPatch) continue;
      if (t.effects !== undefined && !effects) continue;
      if (t.nextInputs !== undefined && !nextInputs) continue;

      if (effects && permissions) {
        for (let j = 0; j < effects.length; j++) {
          const et = effects[j]!.type;
          if (!permissions.effectTypesAllowlist.includes(et)) {
            issues.push({
              level: "warning",
              code: "EFFECT_TYPE_NOT_ALLOWED",
              message: `effect type '${et}' not in permissions.effectTypesAllowlist`,
              path: `${path}.effects[${j}].type`
            });
          }
        }
      }

      transitions.push({
        from,
        on,
        to,
        guard: guard ?? undefined,
        contextPatch: contextPatch ?? undefined,
        effects: effects ?? undefined,
        nextInputs: nextInputs ?? undefined
      });
    }
  }

  // Reachability warnings
  if (states && initialState && transitions.length > 0 && !issues.some((i) => i.level === "error")) {
    const adj = new Map<string, string[]>();
    for (const s of states) adj.set(s, []);
    for (const t of transitions) adj.get(t.from)!.push(t.to);
    const q: string[] = [initialState];
    const vis = new Set<string>([initialState]);
    while (q.length) {
      const cur = q.shift()!;
      for (const nxt of adj.get(cur) ?? []) {
        if (!vis.has(nxt)) {
          vis.add(nxt);
          q.push(nxt);
        }
      }
    }
    for (const s of states) {
      if (!vis.has(s)) {
        issues.push({ level: "warning", code: "UNREACHABLE_STATE", message: `state '${s}' is unreachable from initialState`, path: "states" });
      }
    }
  }

  if (issues.some((i) => i.level === "error")) return { issues };

  const typedSpec: Spec = {
    specId: specId!,
    specVersion: specVersion!,
    schemaVersion: schemaVersion!,
    initialState: initialState!,
    terminalStates: terminalStates!,
    states: states!,
    transitions,
    timeouts: parseTimeouts(spec.timeouts, issues),
    permissions: permissions!,
    limits: limits!
  };

  // Timeout semantic checks
  if (typedSpec.timeouts) {
    for (let i = 0; i < typedSpec.timeouts.length; i++) {
      const tr = typedSpec.timeouts[i]!;
      if (!typedSpec.states.includes(tr.inState)) {
        issues.push({
          level: "error",
          code: "TIMEOUT_INSTATE_UNKNOWN",
          message: `timeouts[${i}].inState '${tr.inState}' not in states`,
          path: `timeouts[${i}].inState`
        });
      }
    }
  }

  if (issues.some((i) => i.level === "error")) return { issues };
  return { issues, spec: typedSpec };
}

function parseTimeouts(v: unknown, issues: ValidationIssue[]): TimeoutRule[] | undefined {
  if (v === undefined) return undefined;
  if (!Array.isArray(v)) {
    issues.push({ level: "error", code: "TIMEOUTS_INVALID", message: "timeouts must be an array", path: "timeouts" });
    return undefined;
  }
  const out: TimeoutRule[] = [];
  for (let i = 0; i < v.length; i++) {
    const tr = v[i];
    const path = `timeouts[${i}]`;
    if (!isObject(tr)) {
      issues.push({ level: "error", code: "TIMEOUT_NOT_OBJECT", message: "timeout rule must be an object", path });
      continue;
    }
    const inState = asString(tr.inState);
    const afterMs = asNumber(tr.afterMs);
    if (!inState) issues.push({ level: "error", code: "TIMEOUT_INSTATE_INVALID", message: "inState must be a string", path: `${path}.inState` });
    if (afterMs === null || afterMs < 0) issues.push({ level: "error", code: "TIMEOUT_AFTER_INVALID", message: "afterMs must be a non-negative number", path: `${path}.afterMs` });
    const emit = tr.emit;
    if (!isObject(emit)) {
      issues.push({ level: "error", code: "TIMEOUT_EMIT_INVALID", message: "emit must be an object", path: `${path}.emit` });
      continue;
    }
    const type = asString(emit.type);
    if (!type) issues.push({ level: "error", code: "TIMEOUT_EMIT_TYPE_INVALID", message: "emit.type must be a string", path: `${path}.emit.type` });
    if (!isJson(emit.payload)) issues.push({ level: "error", code: "TIMEOUT_EMIT_PAYLOAD_INVALID", message: "emit.payload must be JSON", path: `${path}.emit.payload` });
    const delayMs = emit.delayMs === undefined ? undefined : asNumber(emit.delayMs);
    if (emit.delayMs !== undefined && (delayMs === null || delayMs < 0)) {
      issues.push({ level: "error", code: "TIMEOUT_EMIT_DELAY_INVALID", message: "emit.delayMs must be a non-negative number", path: `${path}.emit.delayMs` });
    }
    if (inState && afterMs !== null && type && isJson(emit.payload) && (emit.delayMs === undefined || (delayMs !== null && delayMs >= 0))) {
      out.push({ inState, afterMs, emit: { type, payload: emit.payload, delayMs: delayMs ?? undefined } });
    }
  }
  return issues.some((i) => i.level === "error" && i.path?.startsWith("timeouts")) ? undefined : out;
}
