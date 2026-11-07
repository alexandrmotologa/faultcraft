import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/proxies': 'http://localhost:8474',
      '/reset': 'http://localhost:8474',
      '/version': 'http://localhost:8474',
      '/api': 'http://localhost:8474',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
