import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': process.env.API_UPSTREAM || 'http://localhost:5080',
      '/health': process.env.API_UPSTREAM || 'http://localhost:5080',
    },
  },
});
