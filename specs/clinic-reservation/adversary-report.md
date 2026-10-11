# Adversary Report (security): clinic-reservation 全体

**実行日**: 2026-10-11
**範囲**: backend（auth / middleware / handler / service の主要部）と frontend の認証まわり。patch-plan・drift・過去レビューは読んでいない。
**全体判定**: ❌ FAIL（バイナリゲートによる。重大・高の欠陥は見つかっていない。未対応の中リスク 5 件がゲートを落とす）

## 確認して問題がなかった点
- JWT: HS256 固定、exp 必須、issuer・sub(UUID)・role 検証、鍵長 32 バイト以上を強制。
- 認可: ルートごとに jwtAuth → StaffOnly / AdminOnly を適用。ロール判定は RequireRole に一本化されている。引数なしは誰も通さない。
- 他人の予約は 404（GetAppointment を patient_id で絞り込み）。順番 API も本人のみ。
- パスワード: bcrypt（cost 12 以上）、72 バイト上限、未登録ユーザーでも dummyHash で比較して時間を揃える。
- リフレッシュトークン: opaque、DB には SHA-256 のみ保存。ローテーションは先着 1 件。Cookie は HttpOnly・Secure・SameSite=Strict。
- パスワードリセット: 存在有無で同一応答、宛先単位の頻度制限、使用後に全セッション失効、トークンはハッシュ保存。
- メール: 宛先の改行・複数アドレスを拒否（ヘッダーインジェクション対策）。
- 入力検証・ボディ 1MB 上限・panic 時に内部情報を出さない・X-Forwarded-For を信用しない。
- フロント: アクセストークンはメモリのみ。localStorage / innerHTML の使用なし。`npm audit --omit=dev` は 0 件。

## 指摘（重い順）

| # | 重大度 | 内容 | 場所 |
|---|---|---|---|
| S1 | 中 | セキュリティヘッダがない（CSP、HSTS、X-Content-Type-Options、frame-ancestors、Referrer-Policy）。リセット URL のトークンはクエリ文字列にあり、Referrer で漏れうる。要配慮個人情報（症状メモ）を扱うため、配備前に設定すべき。 | backend/internal/app/app.go、frontend/index.html |
| S2 | 中 | IP 単位のレートリミットが `ExtractIPDirect` 固定。リバースプロキシ配下に置くと全利用者が 1 つの IP に見え、公開 API（60 回/分）とログイン IP 上限（100 回/15 分）を全員で共有して、すぐ 429 になる。逆に XFF を信用する設定にすると迂回される。本番構成が未整備のため未決。 | app.go:56 |
| S3 | 中 | ロックがアカウント単位（メール）なので、他人のメールで 5 回失敗させると本人を 15 分締め出せる（標的型 DoS）。患者が電話に頼れない時間帯に影響しうる。保存先もメモリで、再起動で消え、複数台で共有されない。 | service/attempt_guard.go |
| S4 | 中 | 監査ログの範囲が狭い。記録されるのはスタッフの待機列・予約一覧の閲覧と休診作成のみ。管理者の設定変更（PUT /admin/schedules, /admin/settings）、呼出・開始・完了、スタッフ代行の当日受付は記録されない。個人情報保護の安全管理措置として不足する可能性がある。 | app.go:105-106、admin_handler.go |
| S5 | 中 | 失効済みリフレッシュトークンの再利用を検知しない。盗まれたトークンが先に使われると、正規ユーザーの更新が失敗するだけで、盗用側のセッションは残る（トークンファミリー失効なし）。 | service/auth_service.go RefreshToken |
| S6 | 低 | `POST /auth/register` は 409 `email_already_registered` を返すため、メールアドレスの登録有無を推測できる（リセット側は同一応答で防いでおり、方針が不揃い）。Sensitive レートリミットはある。 | auth_handler.go Register |
| S7 | 低 | アクセストークンは失効できない。ログアウトやロール変更・アカウント無効化は最大 15 分遅れる。role をトークンに埋め込んでいる。 | auth/token.go |
| S8 | 低 | `JWT_REFRESH_SECRET` は必須だが、どこでも使われていない（リフレッシュは opaque トークン）。設定の誤解と運用負荷のもとになる。 | config.go:117 |
| S9 | 低 | リセット要求のたびに新しいトークンを作り、旧トークンを無効化しない（有効なトークンが同時に複数）。有効期限 30 分で影響は小さい。 | RequestPasswordReset |
| S10 | 情報 | `govulncheck` が未導入のため Go 依存の脆弱性スキャンは未実施。CORS は未設定で、同一オリジン前提としては妥当。 | — |

## 未実施・限界
- DB マイグレーション、SQL（sqlc クエリ）、通知本文テンプレート、SMTP の TLS（`smtp.SendMail` は STARTTLS を試みるが、強制はしない）の精査は行っていない。
- 動的な攻撃試験（実サーバーへの ZAP 等）は行っていない。
- 個人情報保護法・安全管理ガイドラインのチェックリスト照合（tasks 5.1 の残り）は未実施。

## 推奨アクション
1. S1・S2 は配備前に必須。ヘッダ追加（`echomw.SecureWithConfig` と index.html / 配信側）と、信頼するプロキシを明示した IPExtractor の設定を行う。
2. S4 は監査対象の拡張（管理者設定変更・キュー操作・代行受付）。
3. S3・S5 は仕様判断が要る（ロック方式、再利用検知の有無）。
4. S6〜S9 は低リスク。S8 は不要なら設定項目を削除する。
5. `govulncheck ./...` を CI に追加する。

## 対応状況(2026-10-11)

| # | 状態 | 対応 |
|---|---|---|
| S1 | 修正済み | API に nosniff / X-Frame-Options / CSP(default-src 'none') / Referrer-Policy / HSTS(TLS 時のみ)を付与。フロントは index.html に referrer メタ、`vite preview` に CSP 等を付与。**本番の静的配信側にも同じヘッダを設定すること**(配信構成は未整備)。 |
| S2 | 修正済み(配備時に設定が必要) | `TRUSTED_PROXIES`(CIDR のカンマ区切り)を追加。設定したプロキシが付けた X-Forwarded-For だけを信用する。未設定なら従来どおり接続元 IP。**プロキシ配下に置くときは必ず設定すること。** |
| S3 | 修正済み(方針変更) | ログインのロックキーをメール単位からメール+IP 単位へ変更。他人のメールで失敗を重ねても本人の別 IP は締め出されない。代わりに、複数 IP を使う分散的な総当たりには、メール単位の天井がなくなる(IP 単位の天井 100 回/15 分と bcrypt、12 文字以上のパスワードで補う)。パスワード再設定の成功時は、そのメールの全 IP のロックを解除する。 |
| S4 | 修正済み | スタッフの来院受付・呼出・開始・完了・代行受付と、管理者の設定変更(schedules / settings)を監査ログに記録(フェイルクローズ)。対象はリクエストパス。 |
| S5 | 修正済み | 失効から 10 秒を超えたリフレッシュトークンの再利用を盗用とみなし、そのユーザーの全セッションを失効させる。10 秒以内の再送(複数タブの同時更新)は拒否するだけ。 |
| S6〜S10 | 未対応 | 低リスク・情報。 |

検証: backend `go vet` と `go test -race ./...`(実 DB 含む)全通過、vitest 222 件、Playwright 22 件、eslint・tsc 通過。
