import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { runVersioned, simulateVersioned, validateVersionedSpec } from "../../v3/runtime.ts";

test("v3 validates versioned specs", () => {
  const res = validateVersionedSpec(2, null);
  assert.ok(res.issues.some((i) => i.code === "SPEC_NOT_OBJECT"));
});

test("v3 simulates versioned decisions", async () => {
  const spec = {
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  };
  const input = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await simulateVersioned(2, spec, [input]);
  assert.equal(res.decisions.length, 1);
  assert.equal(res.instance.state, "B");
});

test("v3 runVersioned commits a v2 input", async () => {
  const spec = {
    specId: "S",
    specVersion: 1,
    schemaVersion: 1,
    initialState: "A",
    terminalStates: ["B"],
    states: ["A", "B"],
    permissions: { effectTypesAllowlist: [] },
    limits: { maxEffectsPerDecision: 10, maxNextInputsPerDecision: 10, maxContextBytes: 10000, maxPayloadBytes: 10000, maxGuardOps: 50 },
    transitions: [{ from: "A", on: "X", to: "B" }]
  };
  const dir = await mkdtemp(join(tmpdir(), "wfeng-v3-"));
  const input = { eventId: "e1", workflowId: "w1", type: "X", occurredAt: new Date().toISOString(), schemaVersion: 1, payload: {} };
  const res = await runVersioned(2, { version: 2, spec, input }, dir);
  assert.equal(res.body.committed, true);
});
