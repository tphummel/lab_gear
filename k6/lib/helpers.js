/**
 * helpers.js - Test data generators and utility functions for k6 tests.
 */

export const VALID_KINDS = [
  'proxmox',
  'nas',
  'sbc',
  'bare_metal',
  'workstation',
  'laptop',
];

const MAKES = ['Dell', 'HP', 'Lenovo', 'Supermicro', 'ASRock', 'Raspberry Pi Foundation', 'ASUS', 'Intel'];
const CPUS = ['Intel Core i7-13700K', 'AMD Ryzen 9 5950X', 'ARM Cortex-A72', 'Intel Xeon E-2388G', 'AMD EPYC 7302P'];
const LOCATIONS = ['rack-1', 'rack-2', 'shelf-a', 'closet', 'office-desk', 'basement'];

/**
 * Returns a random element from an array.
 */
export function randomChoice(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

/**
 * Returns a random integer between min (inclusive) and max (inclusive).
 */
export function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1)) + min;
}

/**
 * Generates a random machine payload for POST/PUT requests.
 * All required fields are always populated; optional fields are included randomly.
 *
 * @param {object} overrides - Optional field overrides.
 * @returns {object} Machine payload suitable for JSON serialization.
 */
export function randomMachine(overrides = {}) {
  const kind = overrides.kind || randomChoice(VALID_KINDS);
  const index = randomInt(1000, 9999);

  const machine = {
    name: `test-${kind}-${index}`,
    kind,
    make: randomChoice(MAKES),
    model: `Model-${index}`,
  };

  // Include optional fields ~60% of the time
  if (Math.random() > 0.4) {
    machine.cpu = randomChoice(CPUS);
  }
  if (Math.random() > 0.4) {
    machine.ram_gb = randomChoice([8, 16, 32, 64, 128, 256]);
  }
  if (Math.random() > 0.4) {
    machine.storage_tb = randomChoice([0.5, 1, 2, 4, 8, 16, 20]);
  }
  if (Math.random() > 0.4) {
    machine.location = randomChoice(LOCATIONS);
  }
  if (Math.random() > 0.6) {
    machine.serial = `SN-${randomInt(100000, 999999)}`;
  }
  if (Math.random() > 0.7) {
    machine.notes = `Created by k6 load test at ${new Date().toISOString()}`;
  }

  return Object.assign(machine, overrides);
}

/**
 * Checks that an HTTP response has the expected status code.
 * Returns a boolean suitable for use with k6 check().
 */
export function expectStatus(res, expected) {
  if (res.status !== expected) {
    console.error(`Expected status ${expected}, got ${res.status}. Body: ${res.body}`);
    return false;
  }
  return true;
}

/**
 * Parses JSON from a response body, returning null on parse failure.
 */
export function parseJSON(res) {
  try {
    return res.json();
  } catch (e) {
    console.error(`Failed to parse JSON from response: ${res.body}`);
    return null;
  }
}
