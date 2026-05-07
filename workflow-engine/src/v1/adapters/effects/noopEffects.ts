import type { EffectExecutor } from "../../runtime/ports.ts";
import type { Effect } from "../../core/spec.ts";

export class NoopEffectExecutor implements EffectExecutor {
  async execute(_effect: Effect): Promise<{ ok: boolean; error?: string }> {
    return { ok: true };
  }
}
