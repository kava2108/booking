# Red フェーズ: 8-user-auth-auth-service-register-login / 3.1

auth/token_test.go、service/mailer_test.go、service/auth_service_test.go（Register / Login）を先に書き、パッケージ未作成でビルド失敗することを確認。

| ケース | AC |
|---|---|
| Register: email 正規化・bcrypt 保存・確認メール / 重複 → ErrEmailAlreadyExists / DB 障害を重複と誤認しない | REQ-001-AC-1/2 |
| パスワード境界: 11/12 文字、日本語 11/12 文字、72/73 bytes、空 | REQ-001-AC-3 |
| メール送信失敗でも登録成功 / 既定 bcrypt cost = 12 | NFR |
| Login: 15 分のアクセストークン（ロール入り）・ハッシュのみ保存・7 日 | REQ-002-AC-1, AC-4 |
| 誤パスワードと未登録は同一エラー・同一メッセージ / **未登録でも bcrypt 比較を実行（時間で検証）** | REQ-002-AC-2 |
| JWT: ラウンドトリップ、期限境界、改ざん、**alg=none / HS512 / 別鍵**、exp なし・issuer 不一致・sub 不正・未知/旧ロール、短い鍵・TTL 不正 | NFR |
| opaque トークン: 43 文字 / SHA-256 64 文字 / 既知ベクトル | — |
| SMTP: ヘッダーインジェクション拒否、件名・本文エンコード、送信エラー伝播 | — |
