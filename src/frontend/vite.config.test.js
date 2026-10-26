import assert from 'node:assert/strict';
import { test } from 'node:test';
import config, { resolveApiUpstream } from './vite.config.js';

test('WithReference service discovery takes precedence over the standalone proxy', () => {
  assert.equal(resolveApiUpstream({
    services__api__http__0: 'http://localhost:49231',
    API_UPSTREAM: 'http://api:8080',
  }), 'http://localhost:49231');
});

test('standalone and Compose configuration remain available', () => {
  assert.equal(resolveApiUpstream({ API_UPSTREAM: 'http://api:8080' }), 'http://api:8080');
  assert.equal(resolveApiUpstream({}), 'http://localhost:8080');
});

test('malformed injected configuration fails instead of falling back to another API', () => {
  for (const value of ['', 'not-a-url', 'ftp://api:8080', 'file:///tmp/api']) {
    assert.throws(() => resolveApiUpstream({
      services__api__http__0: value,
      API_UPSTREAM: 'http://api:8080',
    }), /services__api__http__0 must be an absolute HTTP\(S\) URL/);
  }
  assert.throws(() => resolveApiUpstream({ API_UPSTREAM: '' }), /API_UPSTREAM must/);
});

test('production builds do not need a running API or discovery configuration', () => {
  assert.deepEqual(config({ command: 'build' }), {});
});

test('Vite proxies both the API and readiness check to the discovered service', () => {
  const old = process.env.services__api__http__0;
  process.env.services__api__http__0 = 'http://localhost:49231';
  try {
    assert.deepEqual(config({ command: 'serve' }).server.proxy, {
      '/api': 'http://localhost:49231',
      '/healthz': 'http://localhost:49231',
    });
  } finally {
    if (old === undefined) delete process.env.services__api__http__0;
    else process.env.services__api__http__0 = old;
  }
});
