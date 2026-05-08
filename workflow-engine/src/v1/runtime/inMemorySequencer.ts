import type { Sequencer } from "./ports.ts";

export class InMemorySequencer implements Sequencer {
  private chains = new Map<string, Promise<unknown>>();

  async runExclusive<T>(workflowId: string, fn: () => Promise<T>): Promise<T> {
    const prev = this.chains.get(workflowId) ?? Promise.resolve();
    const run = prev.then(fn, fn);
    this.chains.set(
      workflowId,
      run.finally(() => {
        if (this.chains.get(workflowId) === run) this.chains.delete(workflowId);
      })
    );
    return run;
  }
}
