/**
 * smoke.js - Smoke test for the lab_gear API.
 *
 * Purpose: Verify that all endpoints are reachable and return expected
 * responses with minimal load (1 VU, 1 iteration). Run this first to
 * confirm the service is up before executing heavier tests.
 *
 * Usage:
 *   k6 run -e API_TOKEN=<token> k6/scripts/smoke.js
 *   k6 run -e BASE_URL=http://localhost:8080 -e API_TOKEN=<token> k6/scripts/smoke.js
 */

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { BASE_URL, authHeaders, publicHeaders } from '../lib/auth.js';
import { randomMachine, expectStatus, parseJSON } from '../lib/helpers.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    // All checks must pass
    checks: ['rate==1.0'],
    // All requests must complete within 2 seconds
    http_req_duration: ['p(100)<2000'],
  },
};

export default function () {
  let createdID;

  group('health check', () => {
    const res = http.get(`${BASE_URL}/healthz`, { headers: publicHeaders() });
    check(res, {
      'healthz status is 200': (r) => r.status === 200,
      'healthz body has status ok': (r) => {
        const body = parseJSON(r);
        return body !== null && body.status === 'ok';
      },
    });
  });

  sleep(0.5);

  group('create machine', () => {
    const payload = JSON.stringify(randomMachine({ kind: 'proxmox' }));
    const res = http.post(`${BASE_URL}/api/v1/machines`, payload, { headers: authHeaders() });
    check(res, {
      'create status is 201': (r) => r.status === 201,
      'create returns id': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.id === 'string' && body.id.length > 0;
      },
      'create returns correct kind': (r) => {
        const body = parseJSON(r);
        return body !== null && body.kind === 'proxmox';
      },
      'create returns created_at': (r) => {
        const body = parseJSON(r);
        return body !== null && typeof body.created_at === 'string';
      },
    });

    const body = parseJSON(res);
    if (body && body.id) {
      createdID = body.id;
    }
  });

  sleep(0.5);

  group('list machines', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines`, { headers: authHeaders() });
    check(res, {
      'list status is 200': (r) => r.status === 200,
      'list returns array': (r) => {
        const body = parseJSON(r);
        return body !== null && Array.isArray(body);
      },
    });
  });

  group('list machines with kind filter', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines?kind=proxmox`, { headers: authHeaders() });
    check(res, {
      'list with filter status is 200': (r) => r.status === 200,
      'list with filter returns array': (r) => {
        const body = parseJSON(r);
        return body !== null && Array.isArray(body);
      },
      'list with filter all items match kind': (r) => {
        const body = parseJSON(r);
        if (!body || !Array.isArray(body)) return false;
        return body.every((m) => m.kind === 'proxmox');
      },
    });
  });

  sleep(0.5);

  if (createdID) {
    group('get machine by id', () => {
      const res = http.get(`${BASE_URL}/api/v1/machines/${createdID}`, { headers: authHeaders() });
      check(res, {
        'get status is 200': (r) => r.status === 200,
        'get returns correct id': (r) => {
          const body = parseJSON(r);
          return body !== null && body.id === createdID;
        },
      });
    });

    sleep(0.5);

    group('update machine', () => {
      const updated = randomMachine({ kind: 'nas', name: 'updated-nas-001' });
      const payload = JSON.stringify(updated);
      const res = http.put(`${BASE_URL}/api/v1/machines/${createdID}`, payload, { headers: authHeaders() });
      check(res, {
        'update status is 200': (r) => r.status === 200,
        'update reflects new name': (r) => {
          const body = parseJSON(r);
          return body !== null && body.name === 'updated-nas-001';
        },
        'update reflects new kind': (r) => {
          const body = parseJSON(r);
          return body !== null && body.kind === 'nas';
        },
        'update preserves id': (r) => {
          const body = parseJSON(r);
          return body !== null && body.id === createdID;
        },
      });
    });

    sleep(0.5);

    group('delete machine', () => {
      const res = http.del(`${BASE_URL}/api/v1/machines/${createdID}`, null, { headers: authHeaders() });
      check(res, {
        'delete status is 204': (r) => r.status === 204,
      });
    });

    sleep(0.5);

    group('get deleted machine returns 404', () => {
      const res = http.get(`${BASE_URL}/api/v1/machines/${createdID}`, { headers: authHeaders() });
      check(res, {
        'get after delete is 404': (r) => r.status === 404,
      });
    });
  }

  group('auth rejection', () => {
    const res = http.get(`${BASE_URL}/api/v1/machines`, {
      headers: { Authorization: 'Bearer invalid-token-xyz' },
    });
    check(res, {
      'invalid token returns 401': (r) => r.status === 401,
    });
  });

  group('invalid kind returns 400', () => {
    const payload = JSON.stringify({ name: 'bad', kind: 'invalid_kind', make: 'Dell', model: 'XPS' });
    const res = http.post(`${BASE_URL}/api/v1/machines`, payload, { headers: authHeaders() });
    check(res, {
      'invalid kind returns 400': (r) => r.status === 400,
    });
  });
}
