import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

// 開発時は同一オリジンで API に接続する（HttpOnly Cookie のリフレッシュトークンが SameSite=Strict のため）。
const apiTarget = process.env.VITE_API_PROXY ?? 'http://localhost:8080';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy: { '/api': { target: apiTarget, changeOrigin: false } } },
  preview: { proxy: { '/api': { target: apiTarget, changeOrigin: false } } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    restoreMocks: true,
  },
});
