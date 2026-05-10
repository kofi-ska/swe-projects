import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { join } from "node:path";
import type { InputEnvelope } from "../v1/core/spec.ts";
import type { VersionedRunRequest, VersionedSimulateRequest, VersionedSpecRequest } from "./contracts.ts";
import { lintVersionedSpec, runVersioned, simulateVersioned, validateVersionedSpec } from "./runtime.ts";

const PORT = Number(process.env.PORT ?? "8080");
const DATA_DIR = process.env.DATA_DIR ?? join(process.cwd(), "data");
const MAX_BODY_BYTES = Number(process.env.MAX_BODY_BYTES ?? "1048576");

export async function startServer(port = PORT) {
  const server = createServer(async (req, res) => {
    try {
      await route(req, res);
    } catch (err) {
      writeJson(res, 500, { ok: false, error: "internal_error", message: String(err) });
    }
  });

  await new Promise<void>((resolve) => {
    server.listen(port, "0.0.0.0", resolve);
  });

  return server;
}

async function route(req: IncomingMessage, res: ServerResponse): Promise<void> {
  const url = new URL(req.url ?? "/", "http://localhost");

  if (req.method === "GET" && url.pathname === "/healthz") {
    writeJson(res, 200, { ok: true });
    return;
  }

  if (req.method === "GET" && url.pathname === "/readyz") {
    writeJson(res, 200, { ok: true, dataDir: DATA_DIR });
    return;
  }

  const match = url.pathname.match(/^\/(v1|v2)\/(validate|lint|simulate|run)$/);
  if (!match || req.method !== "POST") {
    writeJson(res, 404, { ok: false, error: "not_found" });
    return;
  }

  const version = match[1] === "v1" ? 1 : 2;
  const action = match[2];
  const body = await readJsonBody(req);
  const traceId = req.headers["x-request-id"]?.toString() ?? randomId();

  if (action === "validate") {
    const request = body as VersionedSpecRequest;
    const result = validateVersionedSpec(version, request?.spec);
    writeJson(res, result.issues.some((i) => i.level === "error") ? 400 : 200, { traceId, version, result });
    return;
  }

  if (action === "lint") {
    const request = body as VersionedSpecRequest;
    const validation = validateVersionedSpec(version, request?.spec);
    if (validation.issues.some((i) => i.level === "error")) {
      writeJson(res, 400, { traceId, version, result: validation });
      return;
    }
    const result = lintVersionedSpec(version, validation.spec);
    writeJson(res, 200, { traceId, version, result });
    return;
  }

  if (action === "simulate") {
    const request = body as VersionedSimulateRequest;
    const validation = validateVersionedSpec(version, request?.spec);
    if (validation.issues.some((i) => i.level === "error")) {
      writeJson(res, 400, { traceId, version, result: validation });
      return;
    }
    const result = await simulateVersioned(version, validation.spec!, normalizeInputs(request?.inputs ?? [], request?.workflowId));
    writeJson(res, 200, { traceId, version, result });
    return;
  }

  if (action === "run") {
    const request = body as VersionedRunRequest;
    const result = await runVersioned(version, request, DATA_DIR);
    writeJson(res, 200, { traceId, version, result });
    return;
  }

  writeJson(res, 404, { ok: false, error: "not_found" });
}

function normalizeInputs(inputs: InputEnvelope[], workflowId?: string): InputEnvelope[] {
  if (!workflowId) return inputs;
  return inputs.map((input) => ({ ...input, workflowId }));
}

async function readJsonBody(req: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  let total = 0;
  for await (const chunk of req) {
    const buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk);
    total += buf.length;
    if (total > MAX_BODY_BYTES) {
      throw new Error("request_too_large");
    }
    chunks.push(buf);
  }
  if (chunks.length === 0) return {};
  const raw = Buffer.concat(chunks).toString("utf8");
  if (!raw.trim()) return {};
  return JSON.parse(raw) as unknown;
}

function writeJson(res: ServerResponse, status: number, body: unknown): void {
  res.statusCode = status;
  res.setHeader("content-type", "application/json; charset=utf-8");
  res.end(JSON.stringify(body));
}

function randomId(): string {
  return `req_${Math.random().toString(36).slice(2, 10)}${Math.random().toString(36).slice(2, 10)}`;
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await startServer();
}
