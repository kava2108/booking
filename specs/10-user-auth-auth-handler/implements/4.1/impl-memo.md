---
issue_id: 10-user-auth-auth-handler
task_id: 4.1
created_at: 2026-10-10T17:14:20+09:00
---

# 実装判断メモ: 10-user-auth-auth-handler / 4.1

## 採用したアプローチ

- **echo-jwt は使わず自前の JWTAuth**: 検証を auth.TokenManager に一本化（HS256 固定・exp 必須・issuer/sub/role 検証）。Steering の「echo-jwt」から外れるが、検証経路が 2 つになるのを避けた。
- **Cookie の Path は /api/v1/auth**（設計書は /api/v1/auth/refresh）: logout も Cookie を読むため。refresh 以外の auth エンドポイントにも送られるが、HttpOnly + SameSite=Strict で露出は限定的。
- **Secure は設定可能（既定 true）**: http の開発環境のみ無効化。HttpOnly / SameSite=Strict は固定。
- **レートリミット**: login は厳格（5/15 分）、register・リセット申請/確定も別カウンタで同じ厳格値（メール濫用防止）、refresh は公開 API 用の緩い値（60/分）。カウンタは別インスタンスで、refresh が login の枠を消費しない。
- **統一エラー形式 {error, message, request_id, details}**: HTTPErrorHandler で 404/405/429/panic まで統一。5xx は汎用メッセージ、原因は Logger が `error` フィールドに記録。panic は stack を含め zerolog に出す（Echo 標準ロガーを使わない）。
- **ルートごとに noStore を付与**: `g.Use` は全メソッドを受ける 404 ルートを作り、405 にならないため。
- **app パッケージへ分離**: 統合テストが main に依存しないよう newEcho を internal/app に移した（#3 の main_test は app_test に移動）。
- **logout は Bearer 必須、Cookie は任意**（期限切れのアクセストークンでは logout できない点は仕様どおり）。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| Cookie Path を /api/v1/auth/refresh のまま、logout は Cookie なしで Bearer のみ | logout でリフレッシュトークンを失効できない（REQ-005-AC-1 を満たせない） |
| 全 auth エンドポイントを 1 つの limiter で制限 | refresh（15 分ごと）が login の枠を消費する |
| ハンドラー単体テスト（service モック） | 統合テスト（実 DB）で Cookie・レートリミット・エラー形式まで一括検証できるため優先 |

## 既存コードへの影響

#3 の main.go / main_test.go を再構成。#6 の role/audit のエラー応答を統一形式に変更（本文は request_id が付く以外同じ）。

## TODO・未解決事項

- [x] ~~共有 IP の問題~~ → アカウント単位に変更して解消（下記「変更」参照）
- [ ] API パスの不一致: user-auth は /api/v1/auth/*、clinic-reservation（OpenAPI）は /api/*。バージョン接頭辞を揃える
- [ ] JWT_REFRESH_SECRET が必須設定のまま未使用
- [ ] リバースプロキシ配下でのクライアント IP 取得（ExtractIPDirect のままだとプロキシの IP で制限される）
- [ ] Dockerfile / docker-compose / .env.example は未作成



## 変更 (2026-10-10): ログイン制限をアカウント単位に変更

- **要件 REQ-002-AC-3 を変更**: 「同一 IP から 5 回」→「同一アカウントで連続 5 回失敗で 15 分ロック」。共有 Wi-Fi（病院の待合室など）の患者を巻き込む問題の解消が目的。
- **実装**: service の `AttemptGuard`（メモリ実装 `MemoryGuard`）。キーは正規化したメール。連続失敗のみ数え、成功と本人確認済みのパスワード再設定で解除。ロック中は正しいパスワードでも拒否し DB にも触れない。未登録メールも同じ回数でロック（存在の推測防止）。インフラ障害は数えない。キー数に上限（既定 10 万）。
- **IP 単位は緩い天井に格下げ**: `RATE_LIMIT_LOGIN_IP_MAX`（既定 100 / 15 分、全アカウントをまたぐ試行）。パスワードスプレー対策。登録・リセットは `RATE_LIMIT_SENSITIVE_MAX`（既定 20）。
- **リセット申請を宛先単位で制限**: 1 時間に 3 通まで（メール爆撃対策）。超過しても応答は同じ。
- **429 に Retry-After**（残り秒数）。
- **新しいリスク**: 他人のアカウントを意図的にロックできる（攻撃者が誤ったパスワードを 5 回送る）。緩和: 15 分で自動解除／パスワード再設定で即解除／Retry-After で利用者に通知。さらに強くするなら (account, IP) の組での制限や CAPTCHA を検討。
- **制約**: ロック状態はプロセスのメモリ上。再起動で消え、複数台では共有されない。複数台構成にする前に共有ストア実装（Redis 等）へ差し替える（`AttemptGuard` interface 越しなので差し替え可能）。
