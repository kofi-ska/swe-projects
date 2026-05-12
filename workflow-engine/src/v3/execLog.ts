import { query } from "./postgres.ts";

export interface ExecutionLogEntry {
  traceId: string;
  serviceVersion: string;
  route: string;
  engineVersion: 1 | 2;
  workflowId?: string;
  eventId?: string;
  specId?: string;
  specVersion?: number;
  outcome: string;
  reason?: string;
  requestJson: unknown;
  responseJson: unknown;
}

export async function logExecution(entry: ExecutionLogEntry): Promise<void> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) return;

  try {
    const requestJson = redactAuditJson(entry.requestJson);
    const responseJson = redactAuditJson(entry.responseJson);
    await query(
      `insert into workflow_executions
        (trace_id, service_version, route, engine_version, workflow_id, event_id, spec_id, spec_version, outcome, reason, request_json, response_json)
       values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
       on conflict (trace_id) do update set
         service_version = excluded.service_version,
         route = excluded.route,
         engine_version = excluded.engine_version,
         workflow_id = excluded.workflow_id,
         event_id = excluded.event_id,
         spec_id = excluded.spec_id,
         spec_version = excluded.spec_version,
         outcome = excluded.outcome,
         reason = excluded.reason,
         request_json = excluded.request_json,
         response_json = excluded.response_json,
         created_at = now()`,
      [
        entry.traceId,
        entry.serviceVersion,
        entry.route,
        entry.engineVersion,
        entry.workflowId ?? null,
        entry.eventId ?? null,
        entry.specId ?? null,
        entry.specVersion ?? null,
        entry.outcome,
        entry.reason ?? null,
        requestJson,
        responseJson
      ],
      databaseUrl
    );
  } catch (err) {
    // Logging must never break the request path.
    // eslint-disable-next-line no-console
    console.warn("execution log write failed", String(err));
  }
}

function redactAuditJson(value: unknown): unknown {
  if (!value || typeof value !== "object") return value;
  if (Array.isArray(value)) return value.map(redactAuditJson);
  const out: Record<string, unknown> = {};
  for (const [key, entry] of Object.entries(value as Record<string, unknown>)) {
    if (key === "payload" || key === "requestJson" || key === "responseJson" || key === "secret" || key === "token" || key === "password") {
      out[key] = "[redacted]";
      continue;
    }
    out[key] = redactAuditJson(entry);
  }
  return out;
}
