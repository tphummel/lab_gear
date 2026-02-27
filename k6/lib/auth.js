/**
 * auth.js - Shared authentication and base URL configuration for k6 tests.
 *
 * Environment variables:
 *   BASE_URL   - The base URL of the lab_gear service (default: http://localhost:8080)
 *   API_TOKEN  - Bearer token for API authentication (required for protected endpoints)
 */

export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

/**
 * Returns HTTP headers with Bearer token authentication.
 * Reads the API_TOKEN environment variable at call time.
 */
export function authHeaders() {
  const token = __ENV.API_TOKEN;
  if (!token) {
    throw new Error('API_TOKEN environment variable is required');
  }
  return {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  };
}

/**
 * Returns HTTP headers without authentication (for public endpoints like /healthz).
 */
export function publicHeaders() {
  return {
    'Content-Type': 'application/json',
  };
}
