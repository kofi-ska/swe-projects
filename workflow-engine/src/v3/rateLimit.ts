import type { IncomingMessage } from "node:http";
import { getDatabaseUrl, query } from "./postgres.ts";

const schemaReadyByUrl = new Map<string, Promise<void>>();
const fallbackWindows = new Map<string, { windowStartMs: number; count: number }>();

export interface RequestLimiter {
  check(req: IncomingMessage): Promise<void>;
}

export function createRequestLimiter(maxRequestsPerMinute: number): RequestLimiter {
  const databaseUrl = getDatabaseUrl();
  if (!databaseUrl) {
    return {
      async check(req: IncomingMessage): Promise<void> {
        enforceInMemory(req, maxRequestsPerMinute);
      }
    };
  }

  return {
    async check(req: IncomingMessage): Promise<void> {
      await ensureSchema(databaseUrl);
      const bucket = requestRateLimitKey(req);
      const now = new Date();
      const cutoff = new Date(now.getTime() - 60_000).toISOString();
      const rows = await query<{ count: number }>(
        `insert into request_rate_limits (bucket, window_started_at, count, updated_at)
         values ($1, $2, 1, $2)
         on conflict (bucket) do update set
           count = case
             when request_rate_limits.window_started_at < $3 then 1
             else request_rate_limits.count + 1
           end,
           window_started_at = case
             when request_rate_limits.window_started_at < $3 then $2
             else request_rate_limits.window_started_at
           end,
           updated_at = $2
         returning count`,
        [bucket, now.toISOString(), cutoff],
        databaseUrl
      );
      if (rows[0]!.count > maxRequestsPerMinute) {
        throw new RateLimitError();
      }
    }
  };
}

function enforceInMemory(req: IncomingMessage, maxRequestsPerMinute: number): void {
  const bucket = requestRateLimitKey(req);
  const now = Date.now();
  const current = fallbackWindows.get(bucket);
  if (!current || now - current.windowStartMs >= 60_000) {
    fallbackWindows.set(bucket, { windowStartMs: now, count: 1 });
    return;
  }
  current.count += 1;
  if (current.count > maxRequestsPerMinute) {
    throw new RateLimitError();
  }
}

function requestRateLimitKey(req: IncomingMessage): string {
  const forwarded = req.headers["x-forwarded-for"];
  const forwardedValue = Array.isArray(forwarded) ? forwarded[0] : forwarded;
  const ip = forwardedValue?.split(",")[0]?.trim() ?? req.socket.remoteAddress ?? "unknown";
  const apiKey = requestApiKey(req);
  return apiKey ? `key:${apiKey}` : `ip:${ip}`;
}

function requestApiKey(req: IncomingMessage): string | undefined {
  const auth = req.headers.authorization?.trim();
  if (auth?.toLowerCase().startsWith("bearer ")) {
    return auth.slice(7).trim();
  }
  const header = req.headers["x-api-key"];
  if (Array.isArray(header)) return header[0]?.trim();
  return header?.trim();
}

async function ensureSchema(databaseUrl: string): Promise<void> {
  const existing = schemaReadyByUrl.get(databaseUrl);
  if (existing) return existing;
  const ready = query(
    `create table if not exists request_rate_limits (
      bucket text primary key,
      window_started_at timestamptz not null,
      count integer not null,
      updated_at timestamptz not null
    )`,
    [],
    databaseUrl
  )
    .then(() => undefined)
    .catch((err) => {
      schemaReadyByUrl.delete(databaseUrl);
      throw err;
    });
  schemaReadyByUrl.set(databaseUrl, ready);
  return ready;
}

export class RateLimitError extends Error {
  constructor() {
    super("too many requests");
    this.name = "RateLimitError";
  }
}
