import { setupServer } from 'msw/node';

// 各テストが server.use(...) で必要なハンドラーを登録する。未登録のリクエストはエラーにする（setup.ts）。
export const server = setupServer();
