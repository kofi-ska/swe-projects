import type { Json, Predicate } from "./spec.ts";

function isObject(v: Json): v is Record<string, Json> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function getPath(root: Json, path: string): Json | undefined {
  // Strict dot-path: a.b.c (no brackets), to keep evaluation predictable.
  const parts = path.split(".");
  if (parts.length === 0) return undefined;
  let cur: Json = root;
  for (const p of parts) {
    if (!/^[A-Za-z0-9_]+$/.test(p)) return undefined;
    if (!isObject(cur)) return undefined;
    cur = cur[p] as Json;
    if (cur === undefined) return undefined;
  }
  return cur;
}

function jsonEquals(a: Json, b: Json): boolean {
  if (a === b) return true;
  if (typeof a !== typeof b) return false;
  if (a === null || b === null) return false;
  if (Array.isArray(a) || Array.isArray(b)) return false;
  if (typeof a !== "object") return false;
  const ao = a as Record<string, Json>;
  const bo = b as Record<string, Json>;
  const ak = Object.keys(ao);
  const bk = Object.keys(bo);
  if (ak.length !== bk.length) return false;
  for (const k of ak) {
    if (!Object.prototype.hasOwnProperty.call(bo, k)) return false;
    if (!jsonEquals(ao[k]!, bo[k]!)) return false;
  }
  return true;
}

export interface GuardEvalContext {
  context: Json;
  payload: Json;
  maxOps: number;
}

export function evaluatePredicate(pred: Predicate, ctx: GuardEvalContext): { ok: boolean; usedOps: number } {
  let usedOps = 0;

  function bump() {
    usedOps++;
    if (usedOps > ctx.maxOps) throw new Error("guard op budget exceeded");
  }

  function evalPred(p: Predicate): boolean {
    bump();
    switch (p.op) {
      case "and":
        for (const a of p.args) if (!evalPred(a)) return false;
        return true;
      case "or":
        for (const a of p.args) if (evalPred(a)) return true;
        return false;
      case "not":
        return !evalPred(p.arg);
      case "exists": {
        const v = getPath({ context: ctx.context, payload: ctx.payload } as any, p.path);
        return v !== undefined;
      }
      case "eq": {
        const v = getPath({ context: ctx.context, payload: ctx.payload } as any, p.path);
        return v !== undefined && jsonEquals(v, p.value);
      }
      case "in": {
        const v = getPath({ context: ctx.context, payload: ctx.payload } as any, p.path);
        if (v === undefined) return false;
        return p.values.some((x) => jsonEquals(v, x));
      }
      case "lt":
      case "lte":
      case "gt":
      case "gte": {
        const v = getPath({ context: ctx.context, payload: ctx.payload } as any, p.path);
        if (typeof v !== "number") return false;
        if (p.op === "lt") return v < p.value;
        if (p.op === "lte") return v <= p.value;
        if (p.op === "gt") return v > p.value;
        return v >= p.value;
      }
      default:
        return false;
    }
  }

  try {
    const ok = evalPred(pred);
    return { ok, usedOps };
  } catch {
    return { ok: false, usedOps };
  }
}
