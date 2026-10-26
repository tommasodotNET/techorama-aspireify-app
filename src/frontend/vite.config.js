import { defineConfig } from 'vite';

export function resolveApiUpstream(environment) {
  const key = Object.hasOwn(environment, 'services__api__http__0')
    ? 'services__api__http__0'
    : Object.hasOwn(environment, 'API_UPSTREAM') ? 'API_UPSTREAM' : null;
  const target = key ? environment[key] : 'http://localhost:8080';
  let url;
  try {
    url = new URL(target);
  } catch {
    throw new Error(`${key} must be an absolute HTTP(S) URL.`);
  }
  if (!['http:', 'https:'].includes(url.protocol) || !url.hostname) {
    throw new Error(`${key} must be an absolute HTTP(S) URL.`);
  }
  return target;
}

export default defineConfig(({ command }) => {
  if (command === 'build') return {};
  const target = resolveApiUpstream(process.env);
  return {
    server: {
      proxy: {
        '/api': target,
        '/healthz': target,
      },
    },
  };
});
