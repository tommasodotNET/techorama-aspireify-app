import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const cli = process.env.CONTAINER_CLI || 'docker';
const composeFile = fileURLToPath(new URL('../compose.yaml', import.meta.url));
const baseUrl = process.env.BASE_URL || 'http://localhost:8080';
const password = process.env.ADMIN_PASSWORD || '4';
const compose = (...args) => execFileSync(cli, ['compose', '-f', composeFile, ...args], { encoding: 'utf8' });

async function request(path, method = 'GET', body) {
  const response = await fetch(`${baseUrl}/api${path}`, {
    method,
    headers: { 'Content-Type': 'application/json', 'X-Admin-Password': password },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  assert.ok(response.ok, `${method} ${path}: ${response.status} ${await response.clone().text()}`);
  return response.status === 204 ? null : response.json();
}

const created = [];
try {
  const fruit = { name: 'Restart-test mango', description: 'Verifies durable storage.', icon: 'orange', price: 5.25, stock: 2 };
  const product = await request('/admin/products', 'POST', fruit);
  created.push(product.id);
  const removed = await request('/admin/products', 'POST', { ...fruit, name: 'Removed restart-test mango' });
  created.push(removed.id);
  await request(`/admin/products/${removed.id}`, 'DELETE');
  created.splice(created.indexOf(removed.id), 1);
  const receipt = await request('/orders', 'POST', { productId: product.id });
  const before = await request('/products');

  console.log('Restarting only the Tall Order demo containers (preserving the volume)…');
  console.log(compose('restart'));
  console.log(compose('up', '-d', '--wait'));

  const after = await request('/products');
  assert.deepEqual(after, before, 'Catalog must be identical after restart; seeds must not reappear.');
  assert.equal(after.find(item => item.id === product.id).stock, 1);
  assert.equal(after.some(item => item.id === removed.id), false);
  // The API-generated UUID is checked before interpolation into this verification query.
  assert.match(receipt.id, /^[a-f0-9-]{36}$/);
  const persistedOrders = compose('exec', '-T', 'db', 'psql', '-U', 'giraffe', '-d', 'tallorder', '-tAc',
    `SELECT count(*) FROM orders WHERE id='${receipt.id}' AND total=5.25`);
  assert.equal(persistedOrders.trim(), '1', 'The order row must survive the database restart.');
  console.log('Persistence verified: inventory, stock, removal, and the order survived a full stack restart.');
} finally {
  for (const id of created) await request(`/admin/products/${id}`, 'DELETE');
}
