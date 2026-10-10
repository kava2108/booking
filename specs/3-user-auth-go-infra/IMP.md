---
kairo:
  node_id: "imp:3-user-auth-go-infra"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "3-user-auth-go-infra"
  feature: "user-auth"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:3-user-auth-go-infra"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/go.mod,', 'backend/cmd/api/main.go', 'backend/internal/config/config.go', 'backend/internal/middleware/request_id.go', 'backend/internal/middleware/logger.go', 'backend/internal/middleware/rate_limiter.go']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 3
---

# IMP: Go プロジェクト基盤 / 設定 / ミドルウェア

| 項目 | 値 |
|---|---|
| imp_id | IMP-3-user-auth-go-infra |
| imp_version | 1.0.0 |
| source_issue | #3（https://github.com/kava2108/booking/issues/3） |
| task | user-auth tasks.md §1.3 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> Go モジュール、viper 設定、request_id / logger / rate_limiter ミドルウェア、main.go（Echo 組立）を作成する。
> #4・#5・#6 が同じ Go モジュールと middleware パッケージに依存するため、go.mod とディレクトリ規約をここで確定する。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 Go プロジェクトがまだ存在せず、他の全 Issue がコードを置けない。

### 1.2 目標
`go build ./...` と `go test ./internal/middleware/...` が通る。zerolog が request_id 入りの JSON ログを出す。同一 IP から5回失敗で429。

### 1.3 非目標（Out of Scope）
JWT 認証ミドルウェア（user-auth P1）、ロール認可・監査ログ（#6）、業務ロジック。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし。パスは設計書・Steering に従い `backend/` 配下とする（issue-struct の記載は `backend/` 省略形）。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/go.mod, go.sum | 追加 | module と依存の定義 | 全体 |
| backend/cmd/api/main.go | 追加 | Echo 生成、ミドルウェアチェイン、ヘルスチェック、graceful shutdown | 全体 |
| backend/internal/config/config.go | 追加 | viper で DB_DSN, JWT_*, SMTP_*, RATE_LIMIT_*, DB_MAX_CONNS を読込（デフォルト値を明示） | 全体 |
| backend/internal/middleware/request_id.go | 追加 | X-Request-ID の生成・付与・context 格納 | 全体 |
| backend/internal/middleware/logger.go | 追加 | zerolog リクエストログ（request_id 必須） | 全体 |
| backend/internal/middleware/rate_limiter.go | 追加 | ulule/limiter による IP ベース制限（5回失敗→15分）。制限値は呼び出し側から注入 | #6 が再利用 |

### 3.2 API 変更

GET /healthz（新規・認証不要）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### RateLimiter(cfg)

**入力**
```
入力: Rate{Limit:5, Period:15m}、リクエスト IP "203.0.113.1"
```

**出力**
```
出力: echo.MiddlewareFunc。6回目で HTTP 429 + Retry-After
```

**エラー**
ストア障害時はフェイルオープンせず 503（認証系のため）。要確認

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| echo/v4 | v4 | HTTP |
| zerolog | 最新安定 | ログ |
| viper | 最新安定 | 設定 |
| ulule/limiter | v3 | レートリミット |
| pgx/v5 | v5 | pgxpool 接続 |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |

---

## 6. タスク詳細

### TASK-0001: Go プロジェクト基盤 / 設定 / ミドルウェア

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-002-AC-3: WHEN 同一 IP アドレスから5回連続でログイン失敗する THEN システムは15分間そのアドレスへの認証エンドポイントへのアクセスをレートリミットする（Must）

**実装手順**
1. go mod init し、Steering の依存を追加する
2. config.go にデフォルト値付きで設定を実装する
3. request_id → logger → recover の順でミドルウェアを実装しテストする
4. rate_limiter を汎用化（キー・レートを引数化）して実装する
5. main.go で組立て、/healthz を追加する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | X-Request-ID が無ければ生成、あれば引継ぎ |
| 異常系 | 必須環境変数 DB_DSN が無い場合は起動失敗し理由をログ |
| 境界値 | 5回目は通過、6回目で 429。15分経過後に解除 |

**受け入れ基準チェックリスト**
- [ ] 正常系: ログ JSON に request_id が含まれる
- [ ] 正常系: ヘッダの X-Request-ID を引き継ぐ
- [ ] 異常系: DB_DSN 未設定で起動エラー
- [ ] 境界値: 6 回目のリクエストで 429
- [ ] セキュリティ: ログにパスワード・トークンが出力されない

**依存関係**
- 前提: #1（pgxpool 接続。middleware 単体は #1 なしで先行可）
- 後続: #6, user-auth P1

---

## 7. 禁止事項

- [ ] 本 Issue の「3.1 変更ファイル一覧」以外のファイルを変更しないこと
- [ ] 他 Issue が所有するファイル・マイグレーション番号を変更しないこと
- [ ] 既存のテストを削除・修正しないこと
- [ ] 新たな外部依存を追加しないこと（5.1 に無いもの。必要なら IMP を更新）
- [ ] 禁止パターン（5.2）を使わないこと

---

## 8. テスト戦略

### 8.1 テスト方針

| レベル | カバレッジ目標 | 使用フレームワーク |
|---|---|---|
| Unit | 90%（middleware/config） | go test + testify + httptest |
| Integration | /healthz 起動確認 | httptest |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: ログ JSON に request_id が含まれる
- [ ] 正常系: ヘッダの X-Request-ID を引き継ぐ
- [ ] 異常系: DB_DSN 未設定で起動エラー
- [ ] 境界値: 6 回目のリクエストで 429
- [ ] セキュリティ: ログにパスワード・トークンが出力されない

### 8.3 テスト除外事項
生成物（sqlc 出力・mockery 出力）は直接テストしない。

### 8.4 テストデータ方針
個人情報を含む実データは使用しない。テスト用の固定 UUID・ダミー氏名のみ。

---

## 9. Definition of Done

- [ ] 全テストケースが通過している
- [ ] 型エラー・ビルドエラーが 0 件である
- [ ] 既存テストが破壊されていない
- [ ] 禁止事項（セクション 7）を全て遵守している
- [ ] IMP の受け入れ基準（EARS）を全て満たしている
- [ ] コードレビュー指摘事項が解消されている

---

## 10. ロールバック計画

### 10.1 ロールバックトリガー条件
ビルド不能・起動不能が他 Issue をブロックする場合。

### 10.2 ロールバック手順
直前コミットへ revert。middleware は独立パッケージのため部分 revert 可。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| rate_limiter の汎用化が不足し #6 で再実装が必要 | M | M | キーとレートを引数化し #6 の IMP でも参照 |
| go.mod の依存バージョンを #4〜#6 が各自更新して衝突 | M | M | go.mod の更新は本 Issue を先にマージしてから |
| issue-struct のパス表記（backend/ 省略）によるディレクトリ誤配置 | M | L | IMP とレビューで backend/ 配下に統一 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] ミドルウェアの依存方向が handler→middleware→config で循環していないか

---

## 13. 承認記録

| ロール | 担当者 | 承認日時 | コメント |
|---|---|---|---|
| アーキテクト | | | |
| 実装者 | | | |

---

## Appendix: 変更履歴

| バージョン | 変更内容 | 変更者 | 日時 |
|---|---|---|---|
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T16:13:39+09:00 |
