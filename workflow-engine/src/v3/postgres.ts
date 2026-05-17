import { Pool, type PoolClient, type PoolConfig, type QueryResultRow } from "pg";

const pools = new Map<string, Pool>();

export function getDatabaseUrl(): string | null {
  return process.env.DATABASE_URL?.trim() || null;
}

export function getPool(databaseUrl = getDatabaseUrl()): Pool | null {
  if (!databaseUrl) return null;
  const existing = pools.get(databaseUrl);
  if (existing) return existing;
  const pool = new Pool({
    connectionString: databaseUrl,
    max: Number(process.env.PG_POOL_SIZE ?? "10"),
    idleTimeoutMillis: 10_000,
    connectionTimeoutMillis: 5_000
  } satisfies PoolConfig);
  pools.set(databaseUrl, pool);
  return pool;
}

export async function query<T extends QueryResultRow = QueryResultRow>(
  text: string,
  values: unknown[] = [],
  databaseUrl = getDatabaseUrl()
): Promise<T[]> {
  const pool = getPool(databaseUrl);
  if (!pool) return [];
  const result = await pool.query<T>(text, values);
  return result.rows;
}

export async function withClient<T>(fn: (client: PoolClient) => Promise<T>, databaseUrl = getDatabaseUrl()): Promise<T | null> {
  const pool = getPool(databaseUrl);
  if (!pool) return null;
  const client = await pool.connect();
  try {
    return await fn(client);
  } finally {
    client.release();
  }
}

export async function shutdownPools(): Promise<void> {
  await Promise.allSettled([...pools.values()].map((pool) => pool.end()));
  pools.clear();
}
