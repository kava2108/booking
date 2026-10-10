---
issue_id: 3-user-auth-go-infra
task_id: 1.3
created_at: 2026-10-10T16:39:54+09:00
---

# 実装判断メモ: 3-user-auth-go-infra / 1.3

## 採用したアプローチ

- **Go 1.25**: 最新の echo v4.16 が go >= 1.25 を要求するため。Steering の「1.23+」を満たす。tech.md の目標値は 1.25 に更新してよい（TODO）。
- **IP 取得は ExtractIPDirect**: Echo 既定は X-Forwarded-For を信用するため、ヘッダー偽装でレートリミットを迂回できる。直接接続の RemoteAddr を使う。リバースプロキシ配下にするときは信頼プロキシを明示して差し替える。
- **レートリミットは「リクエスト数」で数える**: ulule/limiter は試行回数のカウンタ。issue の「5 回失敗後に 429」は、認証エンドポイントへの 5 回の試行と読み替えた（成功ログインも数える）。失敗のみを数える・成功でリセットする挙動が必要なら、P1 の auth ハンドラー側で `Reset` を呼ぶ拡張が必要。
- **ストア障害は 503（フェイルクローズ）**: 認証系の制限を黙って無効化しない。
- **メモリストア**: 単一インスタンス前提。複数台にするなら Redis ストアへ差し替え（NewRateLimiter が Store を受け取る）。
- **ログ**: ボディ・ヘッダー・クエリ文字列は出さない（リセットトークンがクエリに載る可能性）。Logger は ID を context から取るので RequestID を先に置く。
- **config はエラーを全件まとめて返す**（errors.Join）。メッセージにキー名のみ、値は含めない。
- **pgxpool は未接続**: DB 接続は repository 実装（P1）で追加する。本タスクでは設定値（DB_DSN, DB_MAX_CONNS）の読込まで。
- Go は docker イメージ（golang:1.25）で実行。ホスト未導入。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| echo 旧版（Go 1.23 対応）に固定 | 古い版のまま脆弱性修正を取り逃がす |
| 失敗回数のみ数える自前実装 | 初期は limiter で十分。要件の詳細は P1 で再評価 |
| echo 標準の RequestID ミドルウェア | 外部 ID の検証（ログインジェクション対策）と context 取得を自前で制御したい |

## 既存コードへの影響

新規のみ。#6 は `NewRateLimiter` を別 Rate で再利用する。#2 の生成コードも同じ module に含まれる。

## TODO・未解決事項

- [ ] 実プロセス起動確認（DB_DSN / JWT_* を与えて /healthz）
- [ ] 失敗のみカウント・成功でリセットする必要があるか P1 で決める
- [x] tech.md の Go バージョン目標を 1.25 に更新した
- [ ] Dockerfile / docker-compose は本 Issue の範囲外
- [ ] Steering の「CI でマイグレーションのチェックサム検証」の方式（golang-migrate には無い）
