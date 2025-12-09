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
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const CREATE_RATIO = Number(__ENV.CREATE_RATIO) || 0.1; // 10% creates, 90% redirects

const codes = new SharedArray('codes', function () {
  const file = __ENV.CODES_FILE || 'codes.json';
  return JSON.parse(open(file));
});

export default function () {
  if (Math.random() < CREATE_RATIO) {
    createUrl();
  } else {
    redirect();
  }
}

function createUrl() {
  const payload = JSON.stringify({
    url: `https://example.com/path/${Date.now()}/${Math.random()}`,
  });

  const res = http.post(`${BASE_URL}/api/v1/urls`, payload, {
    headers: { 'Content-Type': 'application/json' },
    tags: { name: 'create' },
  });

  check(res, {
    'create: status is 201': (r) => r.status === 201,
  });
}

function redirect() {
  const code = codes[Math.floor(Math.random() * codes.length)];

  const res = http.get(`${BASE_URL}/${code}`, {
    redirects: 0,
    tags: { name: 'redirect' },
  });

  check(res, {
    'redirect: status is 302': (r) => r.status === 302,
  });
}
