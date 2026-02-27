/**
 * stress.js - Stress test for the lab_gear API.
 *
 * Purpose: Push the service beyond normal capacity to find its breaking point,
 * observe degradation behavior, and confirm the service recovers after load drops.
 *
 * Stages:
 *   0:00 -  2:00  Ramp up to 20 VUs (normal load)
 *   2:00 -  4:00  Hold at 20 VUs
 *   4:00 -  6:00  Ramp up to 50 VUs (moderate stress)
 *   6:00 -  8:00  Hold at 50 VUs
 *   8:00 - 10:00  Ramp up to 100 VUs (high stress)
 *  10:00 - 12:00  Hold at 100 VUs
 *  12:00 - 14:00  Ramp down to 0 (recovery)
 *
 * The thresholds here are more relaxed than load.js — we're looking for
 * hard failures (5xx responses), not SLA compliance.
 *
 * Usage:
 *   k6 run -e API_TOKEN=<token> k6/scripts/stress.js
 *   k6 run -e BASE_URL=http://my-server:8080 -e API_TOKEN=<token> k6/scripts/stress.js
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { BASE_URL, authHeaders } from '../lib/auth.js';
import { randomMachine, randomChoice, VALID_KINDS, parseJSON } from '../lib/helpers.js';

export const options = {
  stages: [
    { duration: '2m', target: 20 },   // Warm up
    { duration: '2m', target: 20 },   // Hold
    { duration: '2m', target: 50 },   // Moderate stress
    { duration: '2m', target: 50 },   // Hold
    { duration: '2m', target: 100 },  // High stress
    { duration: '2m', target: 100 },  // Hold
    { duration: '2m', target: 0 },    // Recovery
  ],
  thresholds: {
    // Server errors (5xx) must stay under 5%
    http_req_failed: ['rate<0.05'],
    // 95th percentile under 2s even under stress
    http_req_duration: ['p(95)<2000'],
    // At least 95% of checks must pass
    checks: ['rate>0.95'],
  },
};

const createdIDs = [];

export default function () {
  const roll = Math.random();

  if (roll < 0.10) {
    // 10% writes (higher than load.js to stress the DB more)
    const payload = JSON.stringify(randomMachine());
    const res = http.post(`${BASE_URL}/api/v1/machines`, payload, { headers: authHeaders() });
    check(res, {
      'create: not 5xx': (r) => r.status < 500,
      'create: status 201 or 400': (r) => r.status === 201 || r.status === 400,
    });
    const body = parseJSON(res);
    if (body && body.id) {
      createdIDs.push(body.id);
    }
  } else if (roll < 0.15 && createdIDs.length > 0) {
    // 5% updates
    const id = randomChoice(createdIDs);
    const payload = JSON.stringify(randomMachine());
    const res = http.put(`${BASE_URL}/api/v1/machines/${id}`, payload, { headers: authHeaders() });
    check(res, {
      'update: not 5xx': (r) => r.status < 500,
    });
  } else if (roll < 0.17 && createdIDs.length > 0) {
    // 2% deletes
    const idx = Math.floor(Math.random() * createdIDs.length);
    const id = createdIDs[idx];
    const res = http.del(`${BASE_URL}/api/v1/machines/${id}`, null, { headers: authHeaders() });
    check(res, {
      'delete: not 5xx': (r) => r.status < 500,
    });
    if (res.status === 204) {
      createdIDs.splice(idx, 1);
    }
  } else if (roll < 0.30 && createdIDs.length > 0) {
    // 13% get by ID
    const id = randomChoice(createdIDs);
    const res = http.get(`${BASE_URL}/api/v1/machines/${id}`, { headers: authHeaders() });
    check(res, {
      'get: not 5xx': (r) => r.status < 500,
    });
  } else {
    // 70% list (the dominant read path)
    const useFilter = Math.random() > 0.6;
    const url = useFilter
      ? `${BASE_URL}/api/v1/machines?kind=${randomChoice(VALID_KINDS)}`
      : `${BASE_URL}/api/v1/machines`;
    const res = http.get(url, { headers: authHeaders() });
    check(res, {
      'list: not 5xx': (r) => r.status < 500,
      'list: status 200': (r) => r.status === 200,
    });
  }

  // Shorter think time than load.js to apply more pressure
  sleep(Math.random() * 0.5 + 0.1);
}
