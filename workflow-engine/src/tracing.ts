import { AsyncLocalStorage } from "node:async_hooks";
import { randomBytes } from "node:crypto";

export type TraceValue = string | number | boolean | null;

export interface TraceAttributes {
  [key: string]: TraceValue | undefined;
}

export interface TracingConfig {
  serviceName: string;
  endpoint: string | null;
  resourceAttributes: Record<string, TraceValue>;
}

interface TraceState {
  traceId: string;
  spans: RecordedSpan[];
  activeSpanIds: string[];
  exported: boolean;
  config: TracingConfig;
}

interface RecordedSpan {
  traceId: string;
  spanId: string;
  parentSpanId?: string;
  name: string;
  startTimeUnixNano: string;
  endTimeUnixNano: string;
  attributes: TraceAttributes;
  status?: { code: number; message?: string };
}

export interface Span {
  traceId: string;
  spanId: string;
  setAttribute(key: string, value: TraceValue): void;
  recordException(error: unknown): void;
  end(status?: "ok" | "error"): void;
}

export interface TraceRootResult<T> {
  traceId: string;
  result: T;
}

type TraceExporter = (payload: unknown, endpoint: string) => Promise<void>;

const als = new AsyncLocalStorage<TraceState>();

let config: TracingConfig = {
  serviceName: process.env.OTEL_SERVICE_NAME ?? "workflow-engine-v3",
  endpoint: process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? "http://localhost:4318/v1/traces",
  resourceAttributes: {}
};

let exporter: TraceExporter = async (payload, endpoint) => {
  if (!endpoint) return;
  try {
    const res = await fetch(endpoint, {
      method: "POST",
      headers: {
        "content-type": "application/json"
      },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      // eslint-disable-next-line no-console
      console.warn("trace export failed", res.status, await res.text());
    }
  } catch (err) {
    // eslint-disable-next-line no-console
    console.warn("trace export error", String(err));
  }
};

export function initTracing(overrides?: Partial<TracingConfig>): void {
  config = {
    serviceName: overrides?.serviceName ?? process.env.OTEL_SERVICE_NAME ?? "workflow-engine-v3",
    endpoint:
      overrides?.endpoint ??
      (process.env.OTEL_TRACES_ENABLED === "false" ? null : process.env.OTEL_EXPORTER_OTLP_ENDPOINT ?? "http://localhost:4318/v1/traces"),
    resourceAttributes: overrides?.resourceAttributes ?? {}
  };
}

export function setTraceExporter(next: TraceExporter): void {
  exporter = next;
}

export function currentTraceId(): string | undefined {
  return als.getStore()?.traceId;
}

export function currentSpan(): Span | undefined {
  const state = als.getStore();
  if (!state) return undefined;
  const spanId = state.activeSpanIds[state.activeSpanIds.length - 1];
  if (!spanId) return undefined;
  const record = state.spans.find((span) => span.spanId === spanId);
  if (!record) return undefined;
  return {
    traceId: record.traceId,
    spanId: record.spanId,
    setAttribute: () => {},
    recordException: () => {},
    end: () => {}
  };
}

export async function withRootSpan<T>(name: string, attributes: TraceAttributes, fn: () => Promise<T> | T): Promise<TraceRootResult<T>> {
  const state: TraceState = {
    traceId: randomHex(16),
    spans: [],
    activeSpanIds: [],
    exported: false,
    config
  };

  return als.run(state, async () => {
    const root = startSpan(name, attributes);
    try {
      const result = await fn();
      root.end("ok");
      await flushTrace();
      return { traceId: state.traceId, result };
    } catch (err) {
      root.recordException(err);
      root.end("error");
      await flushTrace();
      throw err;
    }
  });
}

export async function withSpan<T>(name: string, attributes: TraceAttributes, fn: () => Promise<T> | T): Promise<T> {
  const state = als.getStore();
  if (!state) return fn();

  const span = startSpan(name, attributes);
  try {
    const result = await fn();
    span.end("ok");
    return result;
  } catch (err) {
    span.recordException(err);
    span.end("error");
    throw err;
  }
}

function startSpan(name: string, attributes: TraceAttributes): Span {
  const state = als.getStore();
  if (!state) {
    return {
      traceId: randomHex(16),
      spanId: randomHex(8),
      setAttribute: () => {},
      recordException: () => {},
      end: () => {}
    };
  }

  const spanId = randomHex(8);
  const parentSpanId = state.activeSpanIds[state.activeSpanIds.length - 1];
  const span: RecordedSpan = {
    traceId: state.traceId,
    spanId,
    name,
    startTimeUnixNano: nowUnixNano(),
    endTimeUnixNano: nowUnixNano(),
    attributes: { ...attributes }
  };
  if (parentSpanId) {
    span.parentSpanId = parentSpanId;
  }
  state.spans.push(span);
  state.activeSpanIds.push(spanId);

  return {
    traceId: state.traceId,
    spanId,
    setAttribute(key: string, value: TraceValue) {
      span.attributes[key] = value;
    },
    recordException(error: unknown) {
      span.status = { code: 2, message: stringifyError(error) };
      span.attributes["exception.type"] = error instanceof Error ? error.name : typeof error;
      span.attributes["exception.message"] = stringifyError(error);
    },
    end(status?: "ok" | "error") {
      span.endTimeUnixNano = nowUnixNano();
      if (status === "error" && !span.status) {
        span.status = { code: 2, message: "error" };
      } else if (!span.status) {
        span.status = { code: 1 };
      }
      const top = state.activeSpanIds[state.activeSpanIds.length - 1];
      if (top === spanId) state.activeSpanIds.pop();
    }
  };
}

async function flushTrace(): Promise<void> {
  const state = als.getStore();
  if (!state || state.exported || state.activeSpanIds.length > 0) return;
  state.exported = true;

  const resourceSpans = [
    {
      resource: {
        attributes: [
          { key: "service.name", value: { stringValue: state.config.serviceName } },
          ...Object.entries(state.config.resourceAttributes)
            .filter(([, value]) => value !== undefined)
            .map(([key, value]) => ({ key, value: toAnyValue(value as TraceValue) }))
        ]
      },
      scopeSpans: [
        {
          scope: {
            name: state.config.serviceName,
            version: "1"
          },
          spans: state.spans.map((span) => ({
            traceId: span.traceId,
            spanId: span.spanId,
            ...(span.parentSpanId ? { parentSpanId: span.parentSpanId } : {}),
            name: span.name,
            kind: 1,
            startTimeUnixNano: span.startTimeUnixNano,
            endTimeUnixNano: span.endTimeUnixNano,
            attributes: Object.entries(span.attributes)
              .filter(([, value]) => value !== undefined)
              .map(([key, value]) => ({
                key,
                value: toAnyValue(value as TraceValue)
              })),
            ...(span.status ? { status: span.status } : {})
          }))
        }
      ]
    }
  ];

  if (state.config.endpoint) {
    void exporter({ resourceSpans }, state.config.endpoint);
  }
}

function toAnyValue(value: TraceValue) {
  if (typeof value === "string") return { stringValue: value };
  if (typeof value === "number") return Number.isInteger(value) ? { intValue: String(value) } : { doubleValue: value };
  if (typeof value === "boolean") return { boolValue: value };
  return { stringValue: "null" };
}

function stringifyError(error: unknown): string {
  if (error instanceof Error) return error.message;
  return typeof error === "string" ? error : JSON.stringify(error);
}

function randomHex(bytes: number): string {
  return randomBytes(bytes).toString("hex");
}

function nowUnixNano(): string {
  return (BigInt(Date.now()) * 1_000_000n).toString();
}
