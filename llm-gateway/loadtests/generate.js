import http from 'k6/http';
import { check } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const failures = new Rate('gateway_failures');
const streamDuration = new Trend('stream_duration', true);

export const options = {
  scenarios: {
    target: {
      executor: 'constant-arrival-rate',
      rate: Number(__ENV.RPS || 100),
      timeUnit: '1s',
      duration: __ENV.DURATION || '10m',
      preAllocatedVUs: 100,
      maxVUs: 500,
    },
  },
  thresholds: {
    gateway_failures: ['rate<0.01'],
    http_req_duration: ['p(95)<2000'],
  },
};

export default function () {
  const started = Date.now();
  const response = http.post(
    `${__ENV.BASE_URL || 'http://localhost:8080'}/v1/generate`,
    JSON.stringify({ prompt: 'Explain distributed tracing', max_tokens: Number(__ENV.TOKENS || 50), priority: 'normal' }),
    { headers: { 'Content-Type': 'application/json' }, timeout: '15s' },
  );
  const good = check(response, {
    'status is 200': (r) => r.status === 200,
    'NDJSON completes': (r) => r.body.includes('"done":true'),
  });
  failures.add(!good);
  streamDuration.add(Date.now() - started);
}
