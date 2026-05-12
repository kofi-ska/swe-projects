import type { Decision, Effect, InputEnvelope, Instance, NextInputTemplate, Spec, Transition } from "./spec.ts";
import { evaluatePredicate } from "./guard.ts";

export function decide(spec: Spec, instance: Instance, input: InputEnvelope): Decision {
  if (instance.status !== "RUNNING") {
    return { effects: [], nextInputs: [], rejection: { reason: "INSTANCE_NOT_RUNNABLE", message: `status=${instance.status}` } };
  }

  const transition = findTransition(spec.transitions, instance.state, input.type);
  if (!transition) return { effects: [], nextInputs: [], rejection: { reason: "NO_TRANSITION" } };

  if (transition.guard) {
    const res = evaluatePredicate(transition.guard, {
      context: instance.context,
      payload: input.payload,
      maxOps: spec.limits.maxGuardOps
    });
    if (!res.ok) return { effects: [], nextInputs: [], rejection: { reason: "GUARD_FAILED" } };
  }

  const effectsResult = buildEffects(spec, transition, input);
  if (effectsResult.skipped > 0) {
    return {
      effects: [],
      nextInputs: [],
      rejection: { reason: "INVALID_INPUT", message: "transition includes disallowed effect type" }
    };
  }
  const effects = effectsResult.effects;
  const nextInputs = buildNextInputs(spec, transition);

  if (effects.length > spec.limits.maxEffectsPerDecision) {
    return {
      effects: [],
      nextInputs: [],
      rejection: { reason: "INVALID_INPUT", message: "effects exceed maxEffectsPerDecision" }
    };
  }
  if (nextInputs.length > spec.limits.maxNextInputsPerDecision) {
    return {
      effects: [],
      nextInputs: [],
      rejection: { reason: "INVALID_INPUT", message: "nextInputs exceed maxNextInputsPerDecision" }
    };
  }

  const decision: Decision = {
    transitionTaken: { from: transition.from, to: transition.to, on: transition.on },
    effects,
    nextInputs
  };
  if (transition.contextPatch) decision.contextPatch = transition.contextPatch;
  return decision;
}

function findTransition(transitions: Transition[], from: string, on: string): Transition | null {
  for (const t of transitions) {
    if (t.from === from && t.on === on) return t;
  }
  return null;
}

function buildEffects(spec: Spec, transition: Transition, input: InputEnvelope): { effects: Effect[]; skipped: number } {
  const out: Effect[] = [];
  let skipped = 0;
  for (let i = 0; i < (transition.effects?.length ?? 0); i++) {
    const tpl = transition.effects![i]!;
    if (!spec.permissions.effectTypesAllowlist.includes(tpl.type)) {
      skipped++;
      continue;
    }
    out.push({
      effectId: `${input.eventId}:effect:${i}`,
      type: tpl.type,
      params: tpl.params,
      idempotencyKey: `${input.eventId}:effect:${i}`
    });
  }
  return { effects: out, skipped };
}

function buildNextInputs(_spec: Spec, transition: Transition): NextInputTemplate[] {
  return transition.nextInputs?.slice() ?? [];
}
