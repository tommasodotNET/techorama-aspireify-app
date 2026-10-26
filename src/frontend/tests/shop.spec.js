import { test, expect } from '@playwright/test';

const password = process.env.ADMIN_PASSWORD || 'alligator123';
const authorization = `Basic ${Buffer.from(`admin:${password}`).toString('base64')}`;

test('real admin add, public reload, and remove workflow', async ({ page, request }) => {
  const title = `The Purrsistence of Memory ${Date.now()}`;
  let id;
  try {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Good books.' })).toBeVisible();
    await expect(page.locator('#book-count')).toContainText('treasures');
    await page.getByRole('link', { name: 'Staff only' }).click();
    await page.getByLabel('Admin password').fill('not-the-password');
    await page.getByRole('button', { name: 'Unlock the back room' }).click();
    await expect(page.getByRole('alert').filter({ hasText: 'Incorrect admin password.' })).toBeVisible();
    await page.getByLabel('Admin password').fill(password);
    await page.getByRole('button', { name: 'Unlock the back room' }).click();
    await page.getByLabel('Book title').fill(title);
    await page.getByLabel('Author', { exact: true }).fill('Salvador Pawli');
    await page.getByLabel('Cat seller', { exact: true }).fill('Mochi');
    await page.getByLabel('Price (USD)').fill('14.25');
    await page.getByLabel('A little about the book').fill('A surreal adventure through the nap dimension.');
    await page.getByRole('button', { name: 'Add to the bookshelf' }).click();
    await expect(page.locator('#admin-message')).toContainText('now on the public shelf');
    const list = await request.get('/api/books');
    expect(list.ok()).toBeTruthy();
    const saved = (await list.json()).find((book) => book.title === title);
    expect(saved.priceCents).toBe(1425);
    id = saved.id;
    await page.getByRole('link', { name: 'The bookshelf' }).click();
    await page.reload();
    await page.getByLabel('Find a book').fill(title);
    await expect(page.locator('.book-card')).toHaveCount(1);
    await expect(page.locator('.book-card .price')).toHaveText('$14.25');
    await page.getByRole('link', { name: 'Staff only' }).click();
    await expect(page.getByLabel('Admin password')).toBeVisible();
    await page.getByLabel('Admin password').fill(password);
    await page.getByRole('button', { name: 'Unlock the back room' }).click();
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: `Remove ${title}`, exact: true }).click();
    await expect(page.locator('#admin-message')).toContainText('was removed');
    await page.getByRole('link', { name: 'The bookshelf' }).click();
    await page.reload();
    await page.getByLabel('Find a book').fill(title);
    await expect(page.locator('#catalog-status')).toContainText('No books match');
    expect((await request.delete(`/api/books/${id}`)).status()).toBe(401);
  } finally {
    if (id) await request.delete(`/api/books/${id}`, { headers: { Authorization: authorization } });
  }
});

test('loading, empty, API error and retry states', async ({ page }) => {
  let release;
  const gate = new Promise((resolve) => { release = resolve; });
  await page.route('**/api/books', async (route) => {
    await gate;
    await route.fulfill({ json: [] });
  });
  await page.goto('/');
  await expect(page.locator('#catalog-status')).toContainText('fetching');
  release();
  await expect(page.locator('#catalog-status')).toContainText('shelf is empty');
  await page.unroute('**/api/books');
  await page.route('**/api/books', (route) => route.fulfill({ status: 503, json: { error: 'Bookshelf unavailable.' } }));
  await page.reload();
  await expect(page.locator('#catalog-error')).toContainText('Bookshelf unavailable.');
  await page.unroute('**/api/books');
  await page.getByRole('button', { name: 'Try again' }).click();
  await expect(page.locator('#catalog-error')).toBeHidden();
  await expect(page.locator('#book-count')).toContainText('treasures');
});

test('book content is text, not executable markup', async ({ page }) => {
  await page.route('**/api/books', (route) => route.fulfill({
    json: [{ id: 'mock', title: '<img src=x onerror=alert(1)>', author: 'A cat', seller: 'Cleo', priceCents: 1200, description: '<script>bad()</script>' }],
  }));
  await page.goto('/');
  await expect(page.locator('.book-details h3')).toHaveText('<img src=x onerror=alert(1)>');
  await expect(page.locator('.book-card img, .book-card script')).toHaveCount(0);
});
