---
kairo:
  node_id: "imp:9-user-auth-auth-service-refresh-reset"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "9-user-auth-auth-service-refresh-reset"
  feature: "user-auth"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T17:01:04+09:00"
  updated_at: "2026-10-10T17:01:04+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:9-user-auth-auth-service-refresh-reset"
  depends_on:
    - id: "req:user-auth"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:user-auth"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/service/auth_service.go', 'backend/internal/service/auth_service_test.go']
  band: "Green"
  last_validated: "2026-10-10T17:01:04+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 9
---

# IMP: AuthService — RefreshToken / Logout / PasswordReset 実装

| 項目 | 値 |
|---|---|
| imp_id | IMP-9-user-auth-auth-service-refresh-reset |
| imp_version | 1.0.0 |
| source_issue | #9（https://github.com/kava2108/booking/issues/9） |
| task | user-auth tasks.md §3.2 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> RefreshToken（ローテーション）、Logout、RequestPasswordReset、ConfirmPasswordReset を実装する。
> 失効・使用済み化は更新件数で判定し、同時リクエストの二重使用を防ぐ。
> パスワード変更時は全リフレッシュトークンを失効させる。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 セッション継続・ログアウト・パスワード回復が無い。🔵 トークンの二重使用（競合）と、リセット後に旧セッションが残る問題を防ぐ必要がある。

### 1.2 目標
ローテーションが原子的で、未登録メールの申請も同一応答、リセット後は旧セッション無効。

### 1.3 非目標（Out of Scope）
HTTP・Cookie（#10）、再利用検知による全セッション失効（TODO）。

---

## 2. スニペット（変更対象の既存コード）

#8 の AuthService と TokenManager。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/service/auth_service.go | 修正 | RefreshToken / Logout / RequestPasswordReset / ConfirmPasswordReset を追加 | #10 |
| backend/internal/service/auth_service_test.go | 修正 | ユニットテストを追加 | — |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| — | 変更なし | 不要 |

---

## 4. I/O 定義

### RefreshToken

**入力**
```
入力: rawToken（Cookie 由来）
```

**出力**
```
出力: TokenPair（新リフレッシュトークン、旧は失効）
```

**エラー**
存在しない・期限切れ・失効済み・競合負け → ErrInvalidToken

### ConfirmPasswordReset

**入力**
```
入力: rawToken, newPassword（12 文字以上）
```

**出力**
```
出力: error のみ。成功時 password 更新、リセットトークン使用済み、全リフレッシュトークン失効
```

**エラー**
期限切れ・使用済み・不明 → ErrInvalidResetToken / 弱いパスワード → ErrWeakPassword

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| 既存のみ | — | #8 と同じ |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| Queries 構造体のハンドラーへの直接注入 | Steering 禁止。テスタビリティ低下 | repository interface 経由で DI |
| 生 SQL の文字列結合 | SQL インジェクション | sqlc のパラメータバインド |
| interface{} / any の無制限使用 | 型安全性 | 具体型・ジェネリクス |
| panic の業務ロジック使用 | Steering 禁止 | error を返す |
| トークンの存在有無で異なるエラーを返す | 列挙攻撃 | ErrInvalidToken / ErrInvalidResetToken に統一 |
| リセットトークンのログ出力 | 漏えい | ログに出さない |

---

## 6. タスク詳細

### TASK-0001: AuthService — RefreshToken / Logout / PasswordReset 実装

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-003-AC-1: WHEN ユーザーが有効なリフレッシュトークンを送信する THEN システムは新しいアクセストークンを発行する（Must）
- [WHEN/IF] REQ-003-AC-2: WHEN リフレッシュトークンが期限切れまたは無効な場合 THEN システムは HTTP 401 を返し、再ログインを要求する（Must）
- [WHEN/IF] REQ-003-AC-3: WHEN リフレッシュトークンが使用された THEN そのトークンを無効化し（ローテーション）、新しいリフレッシュトークンを発行する（Must）
- [WHEN/IF] REQ-004-AC-1: WHEN ユーザーが登録済みメールアドレスでリセット申請する THEN システムは有効期限 30 分のリセットトークンを含むメールを送信する（Must）
- [WHEN/IF] REQ-004-AC-2: WHEN 未登録のメールアドレスでリセット申請があった場合 THEN システムは登録済みと同一のレスポンスを返す（メールアドレス存在の推測防止）（Must）
- [WHEN/IF] REQ-004-AC-3: WHEN ユーザーが有効なリセットトークンと新しいパスワードを送信する THEN システムはパスワードを更新し、リセットトークンを無効化する（Must）
- [WHEN/IF] REQ-004-AC-4: WHEN リセットトークンが期限切れまたは使用済みの場合 THEN システムは HTTP 400 を返す（Must）
- [WHEN/IF] REQ-005-AC-1: WHEN ユーザーがログアウトを要求する THEN システムはリフレッシュトークンを無効化し、Cookie を削除する（Must）
- [WHEN/IF] REQ-005-AC-2: WHEN ログアウト後にリフレッシュトークンを使用しようとした場合 THEN システムは HTTP 401 を返す（Must）

**実装手順**
1. RefreshToken: ハッシュ→取得→有効性→Revoke（件数判定）→新ペア保存
2. Logout: ハッシュ→取得→失効（未存在でも成功＝冪等）
3. RequestPasswordReset: 未登録でも nil。登録済みなら 30 分トークンを保存し非同期メール
4. ConfirmPasswordReset: 検証→使用済み化（件数判定）→bcrypt→UpdatePassword→全失効
5. 各メソッドのユニットテスト

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | ローテーションで旧トークンが使えなくなる |
| 異常系 | 期限切れ・失効済み・不明・競合で 401 相当 |
| 境界値 | 期限ちょうど、30 分ちょうど、パスワード 11/12 文字 |

**受け入れ基準チェックリスト**
- [ ] 正常系: refresh で新ペア、旧は失効
- [ ] 異常系: 失効済み・期限切れ・不明は ErrInvalidToken
- [ ] 異常系: Revoke が 0 件（競合負け）は ErrInvalidToken
- [ ] 正常系: logout 後の refresh は ErrInvalidToken
- [ ] 正常系: 未登録メールの reset 申請も nil でトークンを作らない
- [ ] 異常系: 使用済み・期限切れリセットトークン → ErrInvalidResetToken
- [ ] 正常系: reset 成功で全リフレッシュトークンが失効
- [ ] セキュリティ: reset の使用済み化が競合しても片方のみ成功

**依存関係**
- 前提: #8
- 後続: #10

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
| Unit | 90% | testify + mockery |
| Integration | #10 で実施 | — |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: refresh で新ペア、旧は失効
- [ ] 異常系: 失効済み・期限切れ・不明は ErrInvalidToken
- [ ] 異常系: Revoke が 0 件（競合負け）は ErrInvalidToken
- [ ] 正常系: logout 後の refresh は ErrInvalidToken
- [ ] 正常系: 未登録メールの reset 申請も nil でトークンを作らない
- [ ] 異常系: 使用済み・期限切れリセットトークン → ErrInvalidResetToken
- [ ] 正常系: reset 成功で全リフレッシュトークンが失効
- [ ] セキュリティ: reset の使用済み化が競合しても片方のみ成功

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
リフレッシュの二重使用や旧セッション残存が見つかった場合。

### 10.2 ロールバック手順
該当メソッドを無効化（ルート停止）。DB 変更なし。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| リセット成功後に UpdatePassword が失敗するとトークンだけ消費 | M | L | ユーザーは再申請。Tx 化は将来の改善 |
| 未登録/登録済みの処理時間差で存在を推測される | M | M | メール送信は非同期。DB 書込み分の差は許容し TODO に記録 |
| 再利用検知（盗難検知）が無い | M | M | 失効済み再提示の全失効は TODO。リスク受容として記録 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] トランザクション境界が無い箇所の失敗時の整合性（部分成功）が許容できる設計か

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
| 1.0.0 | 初版作成 | Suetsugu Kawamura | 2026-10-10T17:01:04+09:00 |
