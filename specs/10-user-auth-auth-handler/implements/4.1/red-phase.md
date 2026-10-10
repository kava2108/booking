# Red フェーズ: 10-user-auth-auth-handler / 4.1

jwt_auth_test.go、config の追加テスト、app/auth_integration_test.go（実 DB の統合テスト）、app/app_test.go を先に書き、`go vet` が undefined（JWTAuth, app.New, CookieSecure ほか）で失敗することを確認。

| ケース | AC |
|---|---|
| 全フロー: 登録 → ログイン → リフレッシュ（ローテーション、旧は 401）→ ログアウト（Cookie 削除）→ 401 | REQ-001〜003, 005 |
| Cookie 属性: HttpOnly・SameSite=Strict・Path=/api/v1/auth・Secure は設定どおり、Cache-Control: no-store | REQ-002-AC-4 |
| 登録エラー: 409（大文字違いも）、422（フィールド別 details）、12/11 文字、72 bytes 超、壊れた JSON 400、巨大ボディ 413 | REQ-001-AC-2/3 |
| ログイン失敗: 誤パスワードと未登録で同一エラー・Cookie なし / **5 回で 6 回目 429 + Retry-After**、別 IP は影響なし、X-Forwarded-For で迂回不可 | REQ-002-AC-2/3 |
| リフレッシュ失敗: Cookie なし・空・でたらめ → 401 / ログアウト: Bearer 必須、Cookie なしでも冪等 | REQ-003-AC-2, REQ-005 |
| パスワードリセット: 登録済み/未登録で**同一レスポンス**・メールは 1 通 / 弱いパスワード 422・トークン温存 / 確定 → 新パスワードで認証・旧セッション失効 / 再利用 400 | REQ-004-AC-1〜4 |
| 統一エラー形式（404/405 も）・request_id 一致 / panic は 500 で内部情報なし | — |
| **応答・ログにパスワード・トークン原文・ハッシュが出ない** | NFR |
| アクセストークンのロールが StaffOnly に連携 | clinic #6 |
| JWT ミドルウェア: 大文字小文字違いのスキーム、欠落/Basic/空/ゴミ/期限切れ/余分なトークン → 401 + WWW-Authenticate | — |
