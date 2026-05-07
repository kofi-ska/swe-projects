import type { Spec, Transition } from "./spec.ts";
import type { ValidationIssue } from "./validateSpec.ts";

export function lintSpec(spec: Spec): { issues: ValidationIssue[]; riskScore: number } {
  const issues: ValidationIssue[] = [];
  let riskScore = 0;

  const key = (t: Pick<Transition, "from" | "on">) => `${t.from}::${t.on}`;

  const transitionByKey = new Map<string, Transition>();
  for (const t of spec.transitions) {
    const k = key(t);
    if (!transitionByKey.has(k)) transitionByKey.set(k, t);
  }

  // Self loops with effects are high-risk for runaway execution.
  for (const t of spec.transitions) {
    if (t.from === t.to && (t.effects?.length ?? 0) > 0) {
      issues.push({
        level: "warning",
        code: "SELF_LOOP_EFFECTS",
        message: `transition ${t.from} --(${t.on})-> ${t.to} is a self-loop with effects`,
        path: "transitions"
      });
      riskScore += 15;
    }
  }

  // Fan-out risk.
  for (const t of spec.transitions) {
    const effects = t.effects?.length ?? 0;
    const nextInputs = t.nextInputs?.length ?? 0;
    const fanout = effects + nextInputs;
    if (fanout > spec.limits.maxEffectsPerDecision + spec.limits.maxNextInputsPerDecision) {
      issues.push({
        level: "warning",
        code: "FANOUT_EXCEEDS_LIMITS",
        message: `transition ${t.from} --(${t.on})-> ${t.to} fanout (${fanout}) exceeds declared limits`,
        path: "limits"
      });
      riskScore += 10;
    } else if (fanout > 25) {
      issues.push({
        level: "warning",
        code: "HIGH_FANOUT",
        message: `transition ${t.from} --(${t.on})-> ${t.to} emits ${fanout} actions`,
        path: "transitions"
      });
      riskScore += 5;
    }
  }

  // Timer storm risk.
  const timeouts = spec.timeouts ?? [];
  if (timeouts.length > 100) {
    issues.push({
      level: "warning",
      code: "MANY_TIMEOUTS",
      message: `spec defines ${timeouts.length} timeout rules`,
      path: "timeouts"
    });
    riskScore += 5;
  }

  return { issues, riskScore };
}
