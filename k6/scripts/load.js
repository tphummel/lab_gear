/**
 * load.js - Normal load test for the lab_gear API.
 *
 * Purpose: Simulate realistic steady-state traffic against the API,
 * validating that the service handles concurrent users within acceptable
 * latency and error-rate thresholds.
 *
 * Traffic mix (read-heavy, as typical of inventory services):
 *   70% - List machines (with and without kind filter)
 *   20% - Get machine by ID
 *   7%  - Create machine
 *   3%  - Update + Delete machine (CRUD lifecycle)
 *
 * Stages:
 *   0:00 - 1:00  Ramp up to 10 VUs
 *   1:00 - 6:00  Hold at 10 VUs (steady state)
 *   6:00 - 7:00  Ramp down to 0
 *
 * Usage:
 *   k6 run -e API_TOKEN=<token> k6/scripts/load.js
 *   k6 run -e BASE_URL=http://my-server:8080 -e API_TOKEN=<token> k6/scripts/load.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { BASE_URL, authHeaders } from '../lib/auth.js';
import { randomMachine, randomChoice, VALID_KINDS, parseJSON } from '../lib/helpers.js';

export const options = {
  stages: [
    { duration: '1m', target: 10 },  // Ramp up
    { duration: '5m', target: 10 },  // Steady state
    { duration: '1m', target: 0 },   // Ramp down
  ],
  thresholds: {
    // 99th percentile response time under 500ms
    http_req_duration: ['p(99)<500', 'p(95)<250'],
    // Error rate under 1%
    http_req_failed: ['rate<0.01'],
    // At least 99% of checks must pass
    checks: ['rate>0.99'],
  },
};

// Shared pool of machine IDs created during the test for read/update operations.
// In k6, module-level state is per-VU, so each VU maintains its own pool.
const createdIDs = [];

/**
 * Creates a machine and adds its ID to the local pool.
 */
function createMachine() {
  const payload = JSON.stringify(randomMachine());
  const res = http.post(`${BASE_URL}/api/v1/machines`, payload, { headers: authHeaders() });
  check(res, {
    'create: status 201': (r) => r.status === 201,
    'create: has id': (r) => {
      const body = parseJSON(r);
      return body !== null && typeof body.id === 'string';
    },
  });

  const body = parseJSON(res);
  if (body && body.id) {
    createdIDs.push(body.id);
  }
}

/**
 * Lists machines, optionally with a kind filter.
 */
function listMachines() {
  const useFilter = Math.random() > 0.5;
  const url = useFilter
    ? `${BASE_URL}/api/v1/machines?kind=${randomChoice(VALID_KINDS)}`
    : `${BASE_URL}/api/v1/machines`;

  const res = http.get(url, { headers: authHeaders() });
  check(res, {
    'list: status 200': (r) => r.status === 200,
    'list: returns array': (r) => {
      const body = parseJSON(r);
      return body !== null && Array.isArray(body);
    },
  });
}

/**
 * Gets a machine by ID from the local pool (if any are available).
 */
function getMachine() {
  if (createdIDs.length === 0) {
    // Fallback: hit list endpoint if no IDs are known yet
    listMachines();
    return;
  }
  const id = randomChoice(createdIDs);
  const res = http.get(`${BASE_URL}/api/v1/machines/${id}`, { headers: authHeaders() });
  check(res, {
    'get: status 200 or 404': (r) => r.status === 200 || r.status === 404,
  });
}

/**
 * Updates a machine from the local pool.
 */
function updateMachine() {
  if (createdIDs.length === 0) return;
  const id = randomChoice(createdIDs);
  const payload = JSON.stringify(randomMachine());
  const res = http.put(`${BASE_URL}/api/v1/machines/${id}`, payload, { headers: authHeaders() });
  check(res, {
    'update: status 200 or 404': (r) => r.status === 200 || r.status === 404,
  });
}

/**
 * Deletes and removes a machine from the local pool.
 */
function deleteMachine() {
  if (createdIDs.length === 0) return;
  const idx = Math.floor(Math.random() * createdIDs.length);
  const id = createdIDs[idx];
  const res = http.del(`${BASE_URL}/api/v1/machines/${id}`, null, { headers: authHeaders() });
  check(res, {
    'delete: status 204 or 404': (r) => r.status === 204 || r.status === 404,
  });
  if (res.status === 204) {
    createdIDs.splice(idx, 1);
  }
}

export default function () {
  const roll = Math.random();

  if (roll < 0.07) {
    // 7% - create
    group('create', createMachine);
  } else if (roll < 0.10) {
    // 3% - full CRUD lifecycle (update + delete)
    group('update', updateMachine);
    sleep(0.2);
    group('delete', deleteMachine);
  } else if (roll < 0.30) {
    // 20% - get by ID
    group('get', getMachine);
  } else {
    // 70% - list (with or without filter)
    group('list', listMachines);
  }

  // Simulate think time between 0.5s and 2s
  sleep(Math.random() * 1.5 + 0.5);
}
