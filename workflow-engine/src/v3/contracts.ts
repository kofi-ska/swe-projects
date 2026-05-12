import type { InputEnvelope } from "../v1/core/spec.ts";

export type EngineVersion = 1 | 2;

export interface VersionedSpecRequest {
  version: EngineVersion;
  spec: unknown;
}

export interface VersionedSimulateRequest extends VersionedSpecRequest {
  inputs: InputEnvelope[];
  workflowId?: string;
}

export interface VersionedRunRequest extends VersionedSpecRequest {
  input: InputEnvelope;
  dataDir?: string;
  workflowId?: string;
}

export interface VersionedRequestResult {
  ok: boolean;
  version: EngineVersion;
  traceId: string;
  body: unknown;
}
