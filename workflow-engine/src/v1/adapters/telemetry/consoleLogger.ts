import type { Logger } from "../../runtime/ports.ts";

export class ConsoleLogger implements Logger {
  info(msg: string, meta?: Record<string, unknown>): void {
    // eslint-disable-next-line no-console
    console.log(msg, meta ?? {});
  }
  warn(msg: string, meta?: Record<string, unknown>): void {
    // eslint-disable-next-line no-console
    console.warn(msg, meta ?? {});
  }
  error(msg: string, meta?: Record<string, unknown>): void {
    // eslint-disable-next-line no-console
    console.error(msg, meta ?? {});
  }
}
