import http from 'k6/http';
import { check } from 'k6';
import { SharedArray } from 'k6/data';

export const options = {
  stages: [
    { duration: '10s', target: 50 },
    { duration: '30s', target: 50 },
    { duration: '10s', target: 100 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

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
