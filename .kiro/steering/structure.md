# Project Structure Steering

## アーキテクチャパターン

フロントエンド / バックエンド分離構成（SPA + REST API）

- **フロントエンド**: React (TypeScript) — シングルページアプリケーション
- **バックエンド**: Go + Echo v4 — REST API サーバー
- **データベース**: PostgreSQL

## ディレクトリ構成（想定）

```
booking/
├── frontend/            # React アプリケーション
│   ├── src/
│   │   ├── components/  # 共通 UI コンポーネント（ロジックなし）
│   │   ├── features/    # 機能単位モジュール
│   │   │   └── [feature]/
│   │   │       ├── api/ # React Query カスタムフック（use[Resource].ts）
│   │   │       ├── components/ # 機能固有コンポーネント
│   │   │       └── types/      # 機能固有型定義
│   │   ├── pages/       # ページコンポーネント（features を組み合わせる）
│   │   ├── hooks/       # アプリ共通カスタムフック
│   │   ├── store/       # 状態管理
│   │   ├── api/         # openapi-typescript 生成型・axios クライアント
│   │   └── types/       # グローバル型定義
│   ├── public/
│   └── package.json
├── backend/             # Go アプリケーション
│   ├── cmd/
│   │   └── api/         # main エントリーポイント
│   ├── internal/
│   │   ├── handler/     # HTTP ハンドラー（Echo ルーティング）
│   │   ├── service/     # ビジネスロジック
│   │   ├── repository/  # DB アクセス層（sqlc 生成コード）
│   │   ├── domain/      # ドメインモデル・エンティティ
│   │   ├── middleware/  # 認証・CORS ・ログ等
│   │   └── config/      # 設定・環境変数
│   ├── db/
│   │   ├── query/       # sqlc 用 SQL ファイル
│   │   └── migrations/  # golang-migrate マイグレーションファイル
│   ├── pkg/         # 外部共有可能なユーティリティ
│   ├── go.mod
│   └── go.sum
├── specs/               # kairo 仕様書
├── .kiro/               # Steering・仕様
└── .kairo/              # kairo 設定
```

## 命名規則

- **フロントエンド ファイル名**: PascalCase（コンポーネント）/ camelCase（ユーティリティ）
- **バックエンド ファイル名**: snake_case（Go 標準）
- **変数名 (TS)**: camelCase
- **変数名 (Go)**: camelCase（ローカル）/ PascalCase（エクスポート）
- **クラス名 (TS)**: PascalCase / **struct 名 (Go)**: PascalCase（エクスポート）
- **API エンドポイント**: kebab-case（例: `/api/facilities/available-slots`）
- **DB テーブル名**: snake_case 複数形（例: `reservations`, `facility_slots`）
- **テストファイル (TS)**: `*.test.ts` / `*.spec.ts`
- **テストファイル (Go)**: `*_test.go`（同一パッケージに配置）

## モジュール境界

| モジュール | 責任範囲 |
|-----------|---------|
| `frontend/src/components` | 再利用可能な UI コンポーネント（ロジックなし） |
| `frontend/src/pages` | ページ単位のコンポーネント・ルーティング |
| `frontend/src/features/[feature]/api` | 機能単位の React Query カスタムフック（コンポーネントへの直接 Hook 埋め込み禁止） |
| `frontend/src/api` | openapi-typescript 生成型・axios クライアントベース |
| `backend/internal/handler` | HTTP リクエスト受付・レスポンス返却のみ |
| `backend/internal/service` | ビジネスロジック（予約・空き確認・通知） |
| `backend/internal/repository` | DB アクセスの抽象化（sqlc 生成コード） |
| `backend/internal/domain` | ドメインモデル・エンティティ定義 |
| `backend/db/query` | sqlc 用 SQL ファイル（読み取り専用・書き込み専用を分鸢） |

## コーディング規約

- **TypeScript**: ESLint + Prettier、strict モード有効
- **Go**: `gofmt` / `goimports`（標準フォーマッターを使用）、`golangci-lint`、型ヒント必須
- **インポート順序 (TS)**: external → internal → relative
- **インポート順序 (Go)**: stdlib → third-party → internal（`goimports` 準拠）
- **コミットメッセージ**: Conventional Commits（`feat:`, `fix:`, `docs:` など）
