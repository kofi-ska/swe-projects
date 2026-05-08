import { mkdir, appendFile, readFile } from "node:fs/promises";
import { join } from "node:path";
import type { EventId } from "../../core/spec.ts";
import type { IdempotencyStore } from "../../runtime/ports.ts";

export class FileIdempotencyStore implements IdempotencyStore {
  private seen = new Set<string>();
  private loaded = false;

  private baseDir: string;

  constructor(baseDir: string) {
    this.baseDir = baseDir;
  }

  private logPath() {
    return join(this.baseDir, "idempotency", "seen.log");
  }

  private async ensureLoaded() {
    if (this.loaded) return;
    this.loaded = true;
    try {
      const raw = await readFile(this.logPath(), "utf8");
      for (const line of raw.split("\n")) {
        const id = line.trim();
        if (id) this.seen.add(id);
      }
    } catch {
      // ignore
    }
  }

  async checkAndMark(eventId: EventId, _opts?: { ttlSeconds?: number }): Promise<{ seen: boolean }> {
    await this.ensureLoaded();
    if (this.seen.has(eventId)) return { seen: true };
    this.seen.add(eventId);
    await mkdir(join(this.baseDir, "idempotency"), { recursive: true });
    await appendFile(this.logPath(), eventId + "\n", "utf8");
    return { seen: false };
  }
}
