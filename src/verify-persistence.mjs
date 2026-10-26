import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const base = process.env.BASE_URL || 'http://localhost:8088';
const headers = {
  Authorization: `Basic ${Buffer.from(`admin:${process.env.ADMIN_PASSWORD || 'alligator123'}`).toString('base64')}`,
  'Content-Type': 'application/json',
};
const composeFile = fileURLToPath(new URL('./compose.yaml', import.meta.url));
async function api(path, options = {}) {
  const response = await fetch(`${base}${path}`, { ...options, signal: AbortSignal.timeout(15000) });
  assert.ok(response.ok, `${options.method || 'GET'} ${path}: ${response.status} ${await response.clone().text()}`);
  return response.status === 204 ? null : response.json();
}
const ids = [];
const before = await api('/api/books');
const title = `Restart proof ${Date.now()}`;
try {
  for (const suffix of ['keep', 'remove']) {
    const book = await api('/api/books', {
      method: 'POST', headers,
      body: JSON.stringify({ title: `${title} ${suffix}`, author: 'Miso', seller: 'Mochi', priceCents: 1123, description: 'Persistence verification.' }),
    });
    ids.push(book.id);
  }
  await api(`/api/books/${ids[1]}`, { method: 'DELETE', headers });
  const restart = spawnSync('docker', ['compose', '-f', composeFile, 'restart', 'cosmos', 'api', 'frontend'], { stdio: 'inherit' });
  if (restart.error) throw restart.error;
  assert.equal(restart.status, 0, 'Compose restart failed');
  let ready = false;
  let lastError;
  for (let attempt = 0; attempt < 90; attempt++) {
    try {
      await api('/healthz');
      ready = true;
      break;
    } catch (error) {
      lastError = error;
      await new Promise((resolve) => setTimeout(resolve, 2000));
    }
  }
  assert.ok(ready, `App not ready after restart: ${lastError}`);
  const after = await api('/api/books');
  assert.deepEqual(after.find((book) => book.id === ids[0]), {
    id: ids[0], shelf: 'books', kind: 'book', title: `${title} keep`,
    author: 'Miso', seller: 'Mochi', priceCents: 1123, description: 'Persistence verification.',
  });
  assert.ok(!after.some((book) => book.id === ids[1]), 'Deleted book returned after restart');
  assert.deepEqual(after.filter((book) => book.id !== ids[0]), before, 'Existing listings or seed data changed after restart');
  console.log('Verified: book fields survive database/API restart, deletion persists, and seeding does not duplicate or change existing listings.');
} finally {
  for (const id of ids) {
    const response = await fetch(`${base}/api/books/${id}`, { method: 'DELETE', headers, signal: AbortSignal.timeout(15000) });
    assert.ok(response.ok || response.status === 404, `Cleanup failed: ${response.status}`);
  }
}
