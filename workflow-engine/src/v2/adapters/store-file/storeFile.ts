import { appendFile, mkdir, readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import type { Instance, WorkflowId } from "../../core/spec.ts";
import type { PendingEffect, PendingTask, WorkflowJournalRecord, WorkflowProjection, WorkflowStore } from "../../runtime/ports.ts";

function workflowKey(workflowId: WorkflowId): string {
  return Buffer.from(workflowId, "utf8").toString("base64url");
}

function workflowIdFromKey(key: string): string {
  return Buffer.from(key, "base64url").toString("utf8");
}

export class FileWorkflowStore implements WorkflowStore {
  private readonly baseDir: string;

  constructor(baseDir: string) {
    this.baseDir = baseDir;
  }

  private journalDir() {
    return join(this.baseDir, "journals");
  }

  private journalPath(workflowId: WorkflowId) {
    return join(this.journalDir(), `${workflowKey(workflowId)}.jsonl`);
  }

  private snapshotPath(workflowId: WorkflowId) {
    return join(this.journalDir(), `${workflowKey(workflowId)}.snapshot.json`);
  }

  async load(workflowId: WorkflowId): Promise<WorkflowProjection | null> {
    const path = this.journalPath(workflowId);
    let raw = "";
    try {
      raw = await readFile(path, "utf8");
    } catch {
      try {
        const snapshotRaw = await readFile(this.snapshotPath(workflowId), "utf8");
        const instance = JSON.parse(snapshotRaw) as Instance;
        return {
          instance,
          seenEventIds: new Map(),
          pendingEffects: [],
          pendingTasks: []
        };
      } catch {
        return null;
      }
    }

    return replayJournal(raw);
  }

  async append(record: WorkflowJournalRecord): Promise<void> {
    await mkdir(this.journalDir(), { recursive: true });
    await appendFile(this.journalPath(record.workflowId), JSON.stringify(record) + "\n", "utf8");
  }

  async listWorkflowIds(): Promise<WorkflowId[]> {
    try {
      const entries = await readdir(this.journalDir(), { withFileTypes: true });
      return entries
        .filter((entry) => entry.isFile() && entry.name.endsWith(".jsonl"))
        .map((entry) => workflowIdFromKey(entry.name.slice(0, -".jsonl".length)));
    } catch {
      return [];
    }
  }

  async save(instance: Instance, expectedVersion: number): Promise<void> {
    const current = await this.load(instance.workflowId);
    if (current?.instance && current.instance.version !== expectedVersion) {
      throw new Error("version conflict");
    }
    await mkdir(this.journalDir(), { recursive: true });
    await appendFile(this.snapshotPath(instance.workflowId), JSON.stringify(instance, null, 2), "utf8");
  }

  async appendHistory(_workflowId: WorkflowId, _record: { input: unknown; decision: unknown }): Promise<void> {
    // Compatibility no-op for legacy test scaffolding.
  }
}

export function replayJournal(raw: string): WorkflowProjection {
  let instance: WorkflowProjection["instance"] = null;
  const seenEventIds = new Map<string, string>();
  const pendingEffects = new Map<string, PendingEffect>();
  const pendingTasks = new Map<string, PendingTask>();

  for (const line of raw.split("\n")) {
    const t = line.trim();
    if (!t) continue;
    const rec = JSON.parse(t) as WorkflowJournalRecord;
    if (rec.kind === "commit") {
      instance = rec.nextInstance;
      seenEventIds.set(rec.eventId, rec.recordedAt);
      for (const eff of rec.effects) pendingEffects.set(eff.effectId, eff);
      for (const task of rec.tasks) pendingTasks.set(task.taskId, task);
      continue;
    }
    if (rec.kind === "rejection") {
      instance = rec.nextInstance;
      seenEventIds.set(rec.eventId, rec.recordedAt);
      continue;
    }
    if (rec.kind === "effect-acked") {
      pendingEffects.delete(rec.effectId);
      continue;
    }
    if (rec.kind === "task-acked") {
      pendingTasks.delete(rec.taskId);
    }
  }

  return {
    instance,
    seenEventIds,
    pendingEffects: [...pendingEffects.values()],
    pendingTasks: [...pendingTasks.values()]
  };
}
