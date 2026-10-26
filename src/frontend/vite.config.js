import { defineConfig } from 'vite';

export default defineConfig({
  server: {
    proxy: {
      '/api': process.env.API_UPSTREAM || 'http://localhost:8080',
      '/healthz': process.env.API_UPSTREAM || 'http://localhost:8080',
    },
  },
});
