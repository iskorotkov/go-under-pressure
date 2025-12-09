import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';

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
    http_req_duration: ['p(50)<10', 'p(95)<50', 'p(99)<100'],
    http_req_failed: ['rate<0.001'],
  },
};

const codes = new SharedArray('codes', function () {
  const file = __ENV.CODES_FILE || 'codes.json';
  return JSON.parse(open(file));
});

export default function () {
  const code = codes[Math.floor(Math.random() * codes.length)];

  const res = http.get(`${BASE_URL}/${code}`, { redirects: 0 });

  check(res, {
    'status is 302': (r) => r.status === 302,
    'has location header': (r) => r.headers['Location'] !== undefined,
  });
}
