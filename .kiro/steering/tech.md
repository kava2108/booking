# Technical Stack Steering

## 言語・ランタイム

| 対象 | 言語 | バージョン（目標） |
|------|------|----------------|
| フロントエンド | TypeScript | 5.x |
| バックエンド | Go | 1.25+ |
| Node.js（ビルドツール） | — | 20 LTS |

## フレームワーク

| 対象 | フレームワーク | 用途 |
|------|--------------|------|
| フロントエンド | React 18 | UI |
| バックエンド | Echo v4 | REST API |
| ORM/クエリビルダー | sqlc + pgx/v5 | DB アクセス |
| マイグレーション | golang-migrate | スキーマ管理 |

## 主要ライブラリ（想定）

### フロントエンド
- `react-router-dom` — ルーティング
- `tanstack/react-query` — サーバー状態管理（Hooks は `src/features/[feature]/api/` に機能単位で切り出す）
- `axios` — HTTP クライアント
- `react-hook-form` + `zod` — フォームバリデーション
- `openapi-typescript` — OpenAPI 定義から TypeScript 型を自動生成（バックエンドとの型乖離防止）
- `tailwindcss` — スタイリング
- `date-fns` — 日時操作
- `vitest` + `testing-library/react` — テスト

### バックエンド
- `echo/v4` — HTTP フレームワーク
- `echo-jwt` — JWT 認証ミドルウェア（Echo 公式）
- `pgx/v5` — PostgreSQL ドライバ（高性能）
- `sqlc` — SQL から型安全な Go コードを生成（`Queries` は interface を介して DI する）
- `golang-migrate` — DB マイグレーション（CI でチェックサム整合性を必ず検証する）
- `golang-jwt/jwt` — JWT 生成・検証
- `bcrypt`（`golang.org/x/crypto`）— パスワードハッシュ
- `go-playground/validator` — 入力バリデーション
- `zerolog` — 構造化ロギング（X-Request-ID を全レイヤーで context 経由で引き回す）
- `ulule/limiter` — レートリミット（Echo ミドルウェアとして組み込む）
- `testify` + `mockery` — テストアサーション・モック生成
- `viper` — 設定管理（環境変数・設定ファイル）

## データベース

- **RDBMS**: PostgreSQL 15+
- **クエリ**: sqlc（SQL ファイルから型安全なコードを生成、生 ORM 禁止）
- **マイグレーション**: golang-migrate（`migrations/` ディレクトリに up/down ファイルを管理、CI でチェックサム検証を必須とする）
- **接続プール**: `pgxpool`、接続数は環境変数で管理（`DB_MAX_CONNS`）
- **命名**: テーブル名 snake_case 複数形、カラム名 snake_case

## インフラ・デプロイ（想定）

- **コンテナ**: Docker + docker-compose（開発環境）
- **CI**: GitHub Actions
- **環境**: dev / staging / production の3環境
- **環境変数管理**: `.env`（ローカル）/ Secrets Manager（本番）

## テスト戦略

| レイヤー | ツール | カバレッジ目標 |
|---------|--------|-------------|
| フロントエンド ユニット | Vitest + Testing Library | 80%+ |
| フロントエンド E2E | Playwright | 主要フロー 100% |
| バックエンド ユニット | `go test` + testify | 80%+ |
| バックエンド 統合 | `go test` + httptest | API エンドポイント全件 |

## セキュリティ制約

- **認証**: JWT（Bearer トークン）+ リフレッシュトークン
- **認可**: ロールベース（一般ユーザー / 施設管理者 / システム管理者）
- **パスワード**: bcrypt ハッシュ、最低12文字
- **秘匿情報**: 環境変数のみ、ハードコード禁止、`.env` は `.gitignore` 対象
- **入力バリデーション**: go-playground/validator（バックエンド）/ zod（フロントエンド）で全インプットを検証
- **SQL インジェクション**: ORM 経由のみ、生 SQL 使用時は必ずパラメータバインド
- **CORS**: 許可オリジンを明示的に列挙（ワイルドカード禁止）
- **レートリミット**: 認証エンドポイントに適用（`ulule/limiter` + Echo ミドルウェア）

## 制約・非推奨事項

- `interface{}` / `any` の無制限使用禁止（Go）—型安全性を維持する
- `dangerouslySetInnerHTML` の無制限使用禁止（React）
- sqlc を迂回した生 SQL 文字列結合は禁止（SQLインジェクション防止）
- `any` 型の無制限使用禁止（TypeScript strict モード）
- シークレット・API キーのソースコードへの直書き禁止
- `panic` の業務ロジックへの使用禁止（recover のある middleware 外では使わない）
- sqlc の `Queries` 構造体をハンドラーに直接注入禁止 — 必ず interface 経由で DI する（テスタビリティ確保）
- React Query の Hooks をコンポーネント内にベタ書き禁止 — `src/features/[feature]/api/` に切り出す
- フロントエンドの API 型を手書き禁止 — `openapi-typescript` により OpenAPI 定義から自動生成する

## AI プロンプト用コンテキストルール

このセクションは LLM がプロジェクトに即したコードを生成するための明示的な指示です。

### Go
- エラーハンドリングは必ず `if err != nil` で行い、素通りさせない
- エラーログには `zerolog` を使い、`X-Request-ID` を必ず含める（例: `log.Error().Str("request_id", reqID).Err(err).Msg("...")`)）
- `Queries` 構造体を使う際は、対応する interface を定義してから実装する（mockery でモック生成）
- DB アクセスは repository 層のみ。service 層は repository interface のみ依存する
- context は第一引数として必ず渡す（`ctx context.Context`）

### TypeScript / React
- API 通信のカスタム Hook は `src/features/[feature]/api/use[Resource].ts` に配置する
- レスポンス型は `openapi-typescript` 生成型を使う（手書き禁止）
- フォームは `react-hook-form` + `zod` 必須、`onChange` ベタ書き禁止
- コンポーネントは props の型を必ず明示する（`React.FC<Props>` 形式）

### 共通
- マイグレーションファイルは必ず up/down ペアで作成する
- 環境変数は `viper` 経由で取得し、デフォルト値を明示する
