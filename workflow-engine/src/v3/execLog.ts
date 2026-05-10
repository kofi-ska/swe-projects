import { execFile } from "node:child_process";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

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

function sqlLiteral(value: string): string {
  return `'${value.replaceAll("'", "''")}'`;
}

function jsonLiteral(value: unknown): string {
  return `${sqlLiteral(JSON.stringify(value))}::jsonb`;
}

export async function logExecution(entry: ExecutionLogEntry): Promise<void> {
  const databaseUrl = process.env.DATABASE_URL;
  if (!databaseUrl) return;

  const sql = [
    "insert into workflow_executions",
    "(trace_id, service_version, route, engine_version, workflow_id, event_id, spec_id, spec_version, outcome, reason, request_json, response_json)",
    "values",
    "(",
    [
      sqlLiteral(entry.traceId),
      sqlLiteral(entry.serviceVersion),
      sqlLiteral(entry.route),
      String(entry.engineVersion),
      entry.workflowId ? sqlLiteral(entry.workflowId) : "null",
      entry.eventId ? sqlLiteral(entry.eventId) : "null",
      entry.specId ? sqlLiteral(entry.specId) : "null",
      entry.specVersion === undefined ? "null" : String(entry.specVersion),
      sqlLiteral(entry.outcome),
      entry.reason ? sqlLiteral(entry.reason) : "null",
      jsonLiteral(entry.requestJson),
      jsonLiteral(entry.responseJson)
    ].join(", "),
    ");"
  ].join(" ");

  try {
    await execFileAsync("psql", ["-d", databaseUrl, "-v", "ON_ERROR_STOP=1", "-c", sql], {
      timeout: 5000,
      maxBuffer: 1024 * 1024
    });
  } catch (err) {
    // Logging must never break the request path.
    // eslint-disable-next-line no-console
    console.warn("execution log write failed", String(err));
  }
}
