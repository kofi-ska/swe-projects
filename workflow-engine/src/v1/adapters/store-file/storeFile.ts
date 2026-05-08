import { mkdir, readFile, writeFile, appendFile } from "node:fs/promises";
import { join } from "node:path";
import type { Decision, InputEnvelope, Instance, WorkflowId } from "../../core/spec.ts";
import type { WorkflowStore } from "../../runtime/ports.ts";

export class FileWorkflowStore implements WorkflowStore {
  private baseDir: string;

  constructor(baseDir: string) {
    this.baseDir = baseDir;
  }

  private instancePath(workflowId: WorkflowId) {
    return join(this.baseDir, "instances", `${workflowId}.json`);
  }

  private historyPath(workflowId: WorkflowId) {
    return join(this.baseDir, "history", `${workflowId}.jsonl`);
  }

  async load(workflowId: WorkflowId): Promise<Instance | null> {
    try {
      const raw = await readFile(this.instancePath(workflowId), "utf8");
      return JSON.parse(raw) as Instance;
    } catch {
      return null;
    }
  }

  async save(instance: Instance, expectedVersion: number): Promise<void> {
    await mkdir(join(this.baseDir, "instances"), { recursive: true });
    const cur = await this.load(instance.workflowId);
    if (cur && cur.version !== expectedVersion) throw new Error("version conflict");
    await writeFile(this.instancePath(instance.workflowId), JSON.stringify(instance, null, 2), "utf8");
  }

  async appendHistory(workflowId: WorkflowId, record: { input: InputEnvelope; decision: Decision }): Promise<void> {
    await mkdir(join(this.baseDir, "history"), { recursive: true });
    await appendFile(this.historyPath(workflowId), JSON.stringify(record) + "\n", "utf8");
  }
}
