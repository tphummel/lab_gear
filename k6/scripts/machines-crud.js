/**
 * machines-crud.js - Full CRUD lifecycle test for the lab_gear machines API.
 *
 * Purpose: Each virtual user executes the complete Create → Read → Update → Delete
 * lifecycle for a machine, verifying data consistency and correctness at every step.
 * Unlike the other tests which mix operations, this script is purely workflow-oriented
 * and is useful for validating correctness under concurrent load.
 *
 * Stages:
 *   0:00 - 1:00  Ramp up to 15 VUs
 *   1:00 - 4:00  Hold at 15 VUs (concurrent CRUD workflows)
 *   4:00 - 5:00  Ramp down to 0
 *
 * Each VU iteration:
 *   1. POST   /api/v1/machines          → verify 201, capture id
 *   2. GET    /api/v1/machines/{id}     → verify 200, data matches
 *   3. GET    /api/v1/machines          → verify machine appears in list
 *   4. PUT    /api/v1/machines/{id}     → verify 200, data updated
 *   5. GET    /api/v1/machines/{id}     → verify update persisted
 *   6. DELETE /api/v1/machines/{id}     → verify 204
 *   7. GET    /api/v1/machines/{id}     → verify 404 (tombstone check)
 *
 * Usage:
 *   k6 run -e API_TOKEN=<token> k6/scripts/machines-crud.js
 *   k6 run -e BASE_URL=http://my-server:8080 -e API_TOKEN=<token> k6/scripts/machines-crud.js
 */

import http from 'k6/http';
import { check, group, sleep, fail } from 'k6';
import { BASE_URL, authHeaders } from '../lib/auth.js';
import { randomMachine, VALID_KINDS, parseJSON } from '../lib/helpers.js';

export const options = {
  stages: [
    { duration: '1m', target: 15 },  // Ramp up
    { duration: '3m', target: 15 },  // Hold
    { duration: '1m', target: 0 },   // Ramp down
  ],
  thresholds: {
    // All CRUD operations must complete successfully
    checks: ['rate>0.99'],
    http_req_failed: ['rate<0.01'],
    // End-to-end lifecycle should stay fast
    http_req_duration: ['p(95)<1000'],
  },
};

export default function () {
  const kind = VALID_KINDS[Math.floor(Math.random() * VALID_KINDS.length)];
  const original = randomMachine({ kind });
  const updated = randomMachine({ kind: 'workstation', name: `updated-${original.name}` });

  let machineID = null;
  let originalCreatedAt = null;

  // ── Step 1: Create ────────────────────────────────────────────────────────
  group('1. create', () => {
    const res = http.post(
      `${BASE_URL}/api/v1/machines`,
      JSON.stringify(original),
      { headers: authHeaders() },
    );

    const passed = check(res, {
      'create: status 201': (r) => r.status === 201,
      'create: body is JSON': (r) => parseJSON(r) !== null,
      'create: has id': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.id === 'string' && body.id.length > 0;
      },
      'create: name matches': (r) => {
        const body = parseJSON(r);
        return body !== null && body.name === original.name;
      },
      'create: kind matches': (r) => {
        const body = parseJSON(r);
        return body !== null && body.kind === original.kind;
      },
      'create: make matches': (r) => {
        const body = parseJSON(r);
        return body !== null && body.make === original.make;
      },
      'create: model matches': (r) => {
        const body = parseJSON(r);
        return body !== null && body.model === original.model;
      },
      'create: has created_at': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.created_at === 'string';
      },
      'create: has updated_at': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.updated_at === 'string';
      },
    });

    if (!passed) {
      // Cannot continue without a valid machine ID
      return;
    }

    const body = parseJSON(res);
    machineID = body.id;
    originalCreatedAt = body.created_at;
  });

  if (!machineID) {
    // Creation failed; skip remaining steps for this iteration
    return;
  }

  sleep(0.3);

  // ── Step 2: Read (verify just-created data) ───────────────────────────────
  group('2. read after create', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines/${machineID}`, { headers: authHeaders() });
    check(res, {
      'read: status 200': (r) => r.status === 200,
      'read: id matches': (r) => {
        const body = parseJSON(r);
        return body !== null && body.id === machineID;
      },
      'read: name matches original': (r) => {
        const body = parseJSON(r);
        return body !== null && body.name === original.name;
      },
      'read: kind matches original': (r) => {
        const body = parseJSON(r);
        return body !== null && body.kind === original.kind;
      },
      'read: created_at is immutable': (r) => {
        const body = parseJSON(r);
        return body !== null && body.created_at === originalCreatedAt;
      },
    });
  });

  sleep(0.3);

  // ── Step 3: Appears in list ───────────────────────────────────────────────
  group('3. appears in list', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines`, { headers: authHeaders() });
    check(res, {
      'list: status 200': (r) => r.status === 200,
      'list: machine is present': (r) => {
        const body = parseJSON(r);
        if (!body || !Array.isArray(body)) return false;
        return body.some((m) => m.id === machineID);
      },
    });
  });

  sleep(0.3);

  // ── Step 4: Update ────────────────────────────────────────────────────────
  group('4. update', () => {
    const res = http.put(
      `${BASE_URL}/api/v1/machines/${machineID}`,
      JSON.stringify(updated),
      { headers: authHeaders() },
    );
    check(res, {
      'update: status 200': (r) => r.status === 200,
      'update: id unchanged': (r) => {
        const body = parseJSON(r);
        return body !== null && body.id === machineID;
      },
      'update: name reflects update': (r) => {
        const body = parseJSON(r);
        return body !== null && body.name === updated.name;
      },
      'update: kind reflects update': (r) => {
        const body = parseJSON(r);
        return body !== null && body.kind === updated.kind;
      },
      'update: created_at is immutable': (r) => {
        const body = parseJSON(r);
        return body !== null && body.created_at === originalCreatedAt;
      },
    });
  });

  sleep(0.3);

  // ── Step 5: Read (verify update persisted) ────────────────────────────────
  group('5. read after update', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines/${machineID}`, { headers: authHeaders() });
    check(res, {
      'read updated: status 200': (r) => r.status === 200,
      'read updated: name is new value': (r) => {
        const body = parseJSON(r);
        return body !== null && body.name === updated.name;
      },
      'read updated: kind is new value': (r) => {
        const body = parseJSON(r);
        return body !== null && body.kind === updated.kind;
      },
    });
  });

  sleep(0.3);

  // ── Step 6: Delete ────────────────────────────────────────────────────────
  group('6. delete', () => {
    const res = http.del(`${BASE_URL}/api/v1/machines/${machineID}`, null, { headers: authHeaders() });
    check(res, {
      'delete: status 204': (r) => r.status === 204,
      'delete: empty body': (r) => r.body === '' || r.body === null,
    });
  });

  sleep(0.3);

  // ── Step 7: Tombstone check ───────────────────────────────────────────────
  group('7. tombstone check', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines/${machineID}`, { headers: authHeaders() });
    check(res, {
      'tombstone: status 404': (r) => r.status === 404,
      'tombstone: error field present': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.error === 'string';
      },
    });
  });

  // Brief pause before next iteration
  sleep(0.5);
}
