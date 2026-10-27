import { test, expect } from '@playwright/test';

const headers = { 'X-Admin-Password': process.env.ADMIN_PASSWORD || '4' };
const sample = { name: 'Test papaya', description: 'Fresh fruit for a test giraffe.', icon: 'melon', price: 4.25, stock: 1 };

test('health and catalog are available through the frontend origin', async ({ request }) => {
  expect((await request.get('/health')).ok()).toBeTruthy();
  const response = await request.get('/api/products');
  expect(response.ok()).toBeTruthy();
  expect(Array.isArray(await response.json())).toBeTruthy();
});

test('admin writes are protected and invalid fruit is rejected', async ({ request }) => {
  expect((await request.post('/api/admin/products', { data: sample })).status()).toBe(401);
  expect((await request.post('/api/admin/unlock', { headers: { 'X-Admin-Password': 'wrong' } })).status()).toBe(401);
  expect((await request.delete('/api/admin/products/10000000-0000-0000-0000-000000000001')).status()).toBe(401);
  for (const invalid of [{ price: -1 }, { stock: -1 }, { price: 1.001 }, { name: ' ' }, { icon: 'not-fruit' }]) {
    expect((await request.post('/api/admin/products', { headers, data: { ...sample, ...invalid } })).status()).toBe(400);
  }
});

test('a purchase is persisted, cannot oversell, and removed fruit cannot be bought', async ({ request }) => {
  const created = await request.post('/api/admin/products', { headers, data: sample });
  expect(created.status()).toBe(201);
  const product = await created.json();
  try {
    const purchases = await Promise.all([1, 2].map(() => request.post('/api/orders', { data: { productId: product.id } })));
    expect(purchases.map(p => p.status()).sort()).toEqual([200, 409]);
    const receipt = await purchases.find(p => p.status() === 200).json();
    expect(receipt.productName).toBe(sample.name);
    expect(receipt.total).toBe(4.25);
    expect(receipt.id).toMatch(/^[a-f0-9-]{36}$/);
    let products = await (await request.get('/api/products')).json();
    expect(products.find(p => p.id === product.id).stock).toBe(0);
    expect((await request.delete(`/api/admin/products/${product.id}`, { headers })).status()).toBe(204);
    products = await (await request.get('/api/products')).json();
    expect(products.some(p => p.id === product.id)).toBeFalsy();
    expect((await request.post('/api/orders', { data: { productId: product.id } })).status()).toBe(409);
  } finally {
    await request.delete(`/api/admin/products/${product.id}`, { headers });
  }
});

test('branch manager adds fruit, shopper buys it, branch manager removes it', async ({ page, request }) => {
  const name = `Stage pears ${Date.now()}`;
  let productId;
  try {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'A little fruit. A tall order.' })).toBeVisible();
    await page.getByRole('button', { name: 'Staff room' }).click();
    await page.getByLabel('Demo password').fill(headers['X-Admin-Password']);
    await page.getByRole('button', { name: 'Unlock staff room' }).click();
    await page.getByLabel('Fruit name').fill(name);
    await page.getByLabel('Price (USD)').fill('3.25');
    await page.getByLabel('Stock', { exact: true }).fill('2');
    await page.getByLabel('Description', { exact: true }).fill('A stage-ready snack.');
    await page.getByRole('button', { name: '+ Add fruit' }).click();
    const card = page.getByRole('article').filter({ has: page.getByRole('heading', { name, exact: true }) });
    await expect(card).toBeVisible();
    const products = await (await request.get('/api/products')).json();
    productId = products.find(p => p.name === name).id;
    await page.getByRole('button', { name: 'Fruit stand', exact: true }).click();
    await card.getByRole('button', { name: 'Buy one' }).click();
    await expect(page.getByText('ORDER PLANTED!')).toBeVisible();
    await expect(card.getByText('1 on the branch')).toBeVisible();
    await page.reload();
    await expect(card.getByText('1 on the branch')).toBeVisible();
    await page.getByRole('button', { name: 'Staff room' }).click();
    await page.getByLabel('Demo password').fill(headers['X-Admin-Password']);
    await page.getByRole('button', { name: 'Unlock staff room' }).click();
    page.once('dialog', dialog => dialog.accept());
    await card.getByRole('button', { name: 'Remove', exact: true }).click();
    await expect(card).toHaveCount(0);
  } finally {
    if (!productId) {
      const products = await (await request.get('/api/products')).json();
      productId = products.find(p => p.name === name)?.id;
    }
    if (productId) await request.delete(`/api/admin/products/${productId}`, { headers });
  }
});

test('loading, empty, and failure states are presented honestly', async ({ page }) => {
  let release;
  const wait = new Promise(resolve => { release = resolve; });
  await page.route('**/api/products', async route => {
    await wait;
    await route.fulfill({ json: [] });
  });
  await page.goto('/');
  await expect(page.getByText('Picking the freshest fruit…')).toBeVisible();
  release();
  await expect(page.getByRole('heading', { name: 'The branches are bare.' })).toBeVisible();
  await page.unroute('**/api/products');
  await page.route('**/api/products', route => route.fulfill({ status: 503, json: { detail: 'Inventory is temporarily unavailable.' } }));
  await page.reload();
  await expect(page.getByRole('alert')).toContainText('Inventory is temporarily unavailable.');
});
