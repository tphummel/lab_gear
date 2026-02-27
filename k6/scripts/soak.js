/**
 * soak.js - Soak (endurance) test for the lab_gear API.
 *
 * Purpose: Run the service at moderate, realistic load for an extended period
 * to detect memory leaks, gradual performance degradation, connection pool
 * exhaustion, and other time-dependent reliability issues.
 *
 * Stages:
 *   0:00 -  5:00  Ramp up to 10 VUs
 *   5:00 - 35:00  Hold at 10 VUs (30 minutes of sustained load)
 *  35:00 - 40:00  Ramp down to 0
 *
 * Total duration: ~40 minutes
 *
 * What to watch for:
 *   - Response time trends upward over time (memory pressure, GC pauses)
 *   - Error rate gradually increases (DB connection leaks, file descriptor exhaustion)
 *   - Heap usage grows without bound (memory leak)
 *
 * Usage:
 *   k6 run -e API_TOKEN=<token> k6/scripts/soak.js
 *   k6 run -e BASE_URL=http://my-server:8080 -e API_TOKEN=<token> k6/scripts/soak.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, authHeaders } from '../lib/auth.js';
import { randomMachine, randomChoice, VALID_KINDS, parseJSON } from '../lib/helpers.js';

export const options = {
  stages: [
    { duration: '5m',  target: 10 },  // Ramp up
    { duration: '30m', target: 10 },  // Sustained load
    { duration: '5m',  target: 0 },   // Ramp down
  ],
  thresholds: {
    // Error rate must stay under 1% for the entire run
    http_req_failed: ['rate<0.01'],
    // 99th percentile must stay under 1s for the entire run
    http_req_duration: ['p(99)<1000', 'p(95)<500'],
    // At least 99% of checks must pass
    checks: ['rate>0.99'],
  },
};

// Each VU maintains a small pool of machine IDs it has created,
// so reads and writes are realistic and balanced.
const myCreatedIDs = [];
const MAX_POOL_SIZE = 20;

function maybeCleanup() {
  // Keep pool bounded to avoid unbounded memory growth in the test itself
  if (myCreatedIDs.length > MAX_POOL_SIZE) {
    const idx = 0;
    const id = myCreatedIDs[idx];
    const res = http.del(`${BASE_URL}/api/v1/machines/${id}`, null, { headers: authHeaders() });
    if (res.status === 204) {
      myCreatedIDs.splice(idx, 1);
    }
  }
}

export default function () {
  maybeCleanup();

  const roll = Math.random();

  if (roll < 0.08) {
    // 8% creates
    const payload = JSON.stringify(randomMachine());
    const res = http.post(`${BASE_URL}/api/v1/machines`, payload, { headers: authHeaders() });
    check(res, {
      'soak create: status 201': (r) => r.status === 201,
      'soak create: has id': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.id === 'string';
      },
    });
    const body = parseJSON(res);
    if (body && body.id) {
      myCreatedIDs.push(body.id);
    }
  } else if (roll < 0.12 && myCreatedIDs.length > 0) {
    // 4% updates
    const id = randomChoice(myCreatedIDs);
    const payload = JSON.stringify(randomMachine());
    const res = http.put(`${BASE_URL}/api/v1/machines/${id}`, payload, { headers: authHeaders() });
    check(res, {
      'soak update: status 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
  } else if (roll < 0.25 && myCreatedIDs.length > 0) {
    // 13% get by ID
    const id = randomChoice(myCreatedIDs);
    const res = http.get(`${BASE_URL}/api/v1/machines/${id}`, { headers: authHeaders() });
    check(res, {
      'soak get: status 200 or 404': (r) => r.status === 200 || r.status === 404,
    });
  } else {
    // 75% list
    const useFilter = Math.random() > 0.5;
    const url = useFilter
      ? `${BASE_URL}/api/v1/machines?kind=${randomChoice(VALID_KINDS)}`
      : `${BASE_URL}/api/v1/machines`;
    const res = http.get(url, { headers: authHeaders() });
    check(res, {
      'soak list: status 200': (r) => r.status === 200,
      'soak list: returns array': (r) => {
        const body = parseJSON(r);
        return body !== null && Array.isArray(body);
      },
    });
  }

  // Realistic think time: 1–3 seconds
  sleep(Math.random() * 2 + 1);
}
