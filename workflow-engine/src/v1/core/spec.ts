export type SpecVersion = number;

export type WorkflowStatus = "RUNNING" | "PAUSED" | "QUARANTINED" | "FAILED";

export type WorkflowId = string;
export type EventId = string;
export type SpecId = string;

export type Json =
  | null
  | boolean
  | number
  | string
  | Json[]
  | { [k: string]: Json };

export interface Limits {
  maxEffectsPerDecision: number;
  maxNextInputsPerDecision: number;
  maxContextBytes: number;
  maxPayloadBytes: number;
  maxGuardOps: number;
}

export interface Permissions {
  effectTypesAllowlist: string[];
  httpAllowedHosts?: string[];
}

export type Predicate =
  | { op: "and"; args: Predicate[] }
  | { op: "or"; args: Predicate[] }
  | { op: "not"; arg: Predicate }
  | { op: "eq"; path: string; value: Json }
  | { op: "in"; path: string; values: Json[] }
  | { op: "exists"; path: string }
  | { op: "lt" | "lte" | "gt" | "gte"; path: string; value: number };

export interface Transition {
  from: string;
  on: string;
  to: string;
  guard?: Predicate;
  contextPatch?: ContextPatch;
  effects?: EffectTemplate[];
  nextInputs?: NextInputTemplate[];
}

export interface ContextPatch {
  set?: Record<string, Json>;
  unset?: string[];
}

export interface EffectTemplate {
  type: string;
  params: Json;
}

export interface NextInputTemplate {
  type: string;
  payload: Json;
  delayMs?: number;
}

export interface TimeoutRule {
  inState: string;
  afterMs: number;
  emit: NextInputTemplate;
}

export interface Spec {
  specId: SpecId;
  specVersion: SpecVersion;
  schemaVersion: number;
  initialState: string;
  terminalStates: string[];
  states: string[];
  transitions: Transition[];
  timeouts?: TimeoutRule[];
  permissions: Permissions;
  limits: Limits;
}

export interface Instance {
  workflowId: WorkflowId;
  specId: SpecId;
  specVersion: SpecVersion;
  state: string;
  context: Json;
  version: number;
  status: WorkflowStatus;
}

export interface InputEnvelope {
  eventId: EventId;
  workflowId: WorkflowId;
  type: string;
  occurredAt: string;
  schemaVersion: number;
  payload: Json;
  tenantId?: string;
  actor?: string;
}

export interface Effect {
  effectId: string;
  type: string;
  params: Json;
  idempotencyKey?: string;
}

export type RejectionReason =
  | "INVALID_INPUT"
  | "NO_TRANSITION"
  | "GUARD_FAILED"
  | "INSTANCE_NOT_RUNNABLE";

export interface Decision {
  transitionTaken?: { from: string; to: string; on: string };
  contextPatch?: ContextPatch;
  effects: Effect[];
  nextInputs: NextInputTemplate[];
  rejection?: { reason: RejectionReason; message?: string };
}
