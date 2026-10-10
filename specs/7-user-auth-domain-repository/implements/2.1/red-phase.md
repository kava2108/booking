# Red フェーズ: 7-user-auth-domain-repository / 2.1

domain/user_test.go（トークン有効性）、repository/user_repository_test.go、token_repository_test.go（実 DB）を先に書き、`go vet` が undefined で失敗することを確認。

| ケース | AC |
|---|---|
| User の作成/取得（role 既定 patient、email/ID 検索で同値） | REQ-001-AC-1, REQ-002-AC-1 |
| 重複 email → ErrEmailTaken、未登録 → ErrNotFound | REQ-001-AC-2 |
| パスワード更新 | REQ-004 |
| リフレッシュトークン 保存/取得/失効（失効後も行は返る） | REQ-003-AC-1 |
| 二重失効の 2 回目は false、**10 並行で成功 1 件** | REQ-003-AC-3 |
| 全失効は対象ユーザーの未失効分のみ | REQ-004 |
| リセットトークン 保存/取得/使用済み化（二重使用は false） | REQ-004-AC-1 |
| IsUsable: 期限ちょうどは無効・期限 1ns 前は有効・失効/使用済み | — |
