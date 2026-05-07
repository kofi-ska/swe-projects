import type { Instance, Spec } from "./spec.ts";

export type MigrationDecision =
  | { ok: true }
  | { ok: false; reason: "spec_mismatch" | "unsupported" | "requires_manual" };

/**
 * v1 policy: instances are pinned to specId/specVersion.
 * Upgrades require an explicit migration step outside `handle()`.
 */
export function canRunInstanceWithSpec(instance: Instance, spec: Spec): MigrationDecision {
  if (instance.specId !== spec.specId) return { ok: false, reason: "spec_mismatch" };
  if (instance.specVersion !== spec.specVersion) return { ok: false, reason: "spec_mismatch" };
  return { ok: true };
}
