# Red フェーズ: 9-user-auth-auth-service-refresh-reset / 3.2

RefreshToken / Logout / RequestPasswordReset / ConfirmPasswordReset をスタブ（not implemented）にした状態で、追加テスト 17 件が全て失敗することを確認。

| ケース | AC |
|---|---|
| ローテーション: 新ペア・旧は失効・ロールは最新のユーザー情報 | REQ-003-AC-1/3 |
| 無効: 不明/失効済み/期限切れ/期限ちょうど/ユーザー削除済み/**競合負け（失効 0 件）** → ErrInvalidToken、DB 障害は別エラー | REQ-003-AC-2 |
| Logout: 失効、不明・失効済み・空でも成功（冪等）、DB 障害は返す | REQ-005-AC-1/2 |
| Reset 申請: 登録済み（30 分・ハッシュのみ保存・URL にトークン）/ **未登録は nil・メール無し** / 内部エラーも nil / メール失敗も nil | REQ-004-AC-1/2 |
| Reset 確定: 成功（bcrypt・使用済み化・全失効）/ 不明・使用済み・期限切れ・期限ちょうど・競合負け → ErrInvalidResetToken / 弱いパスワードはトークンを消費しない / **セッション失効に失敗したらパスワードを変えない** | REQ-004-AC-3/4 |
