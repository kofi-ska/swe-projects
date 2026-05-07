import type { Metrics } from "../../runtime/ports.ts";

export class NoopMetrics implements Metrics {
  inc(_name: string, _tags?: Record<string, string>): void {}
  observeMs(_name: string, _valueMs: number, _tags?: Record<string, string>): void {}
}
