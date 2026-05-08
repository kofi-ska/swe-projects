import type { QuotaLimiter } from "../../runtime/ports.ts";
import type { InputEnvelope } from "../../core/spec.ts";

export class AllowAllQuotaLimiter implements QuotaLimiter {
  async check(_input: InputEnvelope): Promise<{ allowed: boolean; reason?: string }> {
    return { allowed: true };
  }
}
