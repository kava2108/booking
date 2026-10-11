import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

// 開発時は同一オリジンで API に接続する（HttpOnly Cookie のリフレッシュトークンが SameSite=Strict のため）。
const apiTarget = process.env.VITE_API_PROXY ?? 'http://localhost:8080';

// 本番の静的配信(preview 相当)で付けるセキュリティヘッダ。配信側(nginx 等)にも同じ内容を設定すること。
// dev サーバーは HMR のインライン script が必要なため CSP を付けない。
const securityHeaders = {
  'Content-Security-Policy':
    "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'",
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'no-referrer',
  'X-Frame-Options': 'DENY',
};

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  server: { proxy: { '/api': { target: apiTarget, changeOrigin: false } } },
  preview: { headers: securityHeaders, proxy: { '/api': { target: apiTarget, changeOrigin: false } } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: false,
    restoreMocks: true,
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
