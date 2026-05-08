import { mkdir, appendFile, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import type { Scheduler } from "../../runtime/ports.ts";

export interface ScheduledTask {
  eventId: string;
  workflowId: string;
  dueAtIso: string;
  type: string;
  payload: unknown;
  dedupeKey?: string;
}

export class FileScheduler implements Scheduler {
  private baseDir: string;

  constructor(baseDir: string) {
    this.baseDir = baseDir;
  }

  private queuePath() {
    return join(this.baseDir, "queue", "scheduled.jsonl");
  }

  async schedule(task: ScheduledTask): Promise<void> {
    await mkdir(join(this.baseDir, "queue"), { recursive: true });
    await appendFile(this.queuePath(), JSON.stringify(task) + "\n", "utf8");
  }

  async popDue(nowIso: string, max: number): Promise<ScheduledTask[]> {
    const now = Date.parse(nowIso);
    if (!Number.isFinite(now)) throw new Error("invalid nowIso");
    let raw = "";
    try {
      raw = await readFile(this.queuePath(), "utf8");
    } catch {
      return [];
    }

    const due: ScheduledTask[] = [];
    const keep: string[] = [];
    for (const line of raw.split("\n")) {
      const t = line.trim();
      if (!t) continue;
      let task: ScheduledTask;
      try {
        task = JSON.parse(t) as ScheduledTask;
      } catch {
        continue;
      }
      const ts = Date.parse(task.dueAtIso);
      if (due.length < max && Number.isFinite(ts) && ts <= now) due.push(task);
      else keep.push(t);
    }

    await writeFile(this.queuePath(), keep.join("\n") + (keep.length ? "\n" : ""), "utf8");
    return due;
  }
}
