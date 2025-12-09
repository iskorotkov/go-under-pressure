import http from 'k6/http';
import { check } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const VUS = Number(__ENV.VUS) || 50;
const VUS_MAX = Number(__ENV.VUS_MAX) || VUS * 2;

export const options = {
  stages: [
    { duration: '10s', target: VUS },
    { duration: '30s', target: VUS },
    { duration: '10s', target: VUS_MAX },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(50)<50', 'p(95)<200', 'p(99)<500'],
    http_req_failed: ['rate<0.001'],
  },
};

export default function () {
  const payload = JSON.stringify({
    url: `https://example.com/path/${Date.now()}/${Math.random()}`,
  });

  const res = http.post(`${BASE_URL}/api/v1/urls`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'status is 201': (r) => r.status === 201,
    'has short_code': (r) => r.json('short_code') !== undefined,
  });
}
