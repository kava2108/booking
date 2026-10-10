---
kairo:
  node_id: "imp:6-clinic-reservation-rbac-audit-openapi"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "6-clinic-reservation-rbac-audit-openapi"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:6-clinic-reservation-rbac-audit-openapi"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/internal/middleware/role.go', 'backend/internal/middleware/audit.go', 'backend/api/openapi.yaml', 'frontend/package.json', 'frontend/src/api/', 'backend/internal/middleware/rate_limiter.go']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 6
---

# IMP: ロール認可・監査ログ・OpenAPI 定義

| 項目 | 値 |
|---|---|
| imp_id | IMP-6-clinic-reservation-rbac-audit-openapi |
| imp_version | 1.0.0 |
| source_issue | #6（https://github.com/kava2108/booking/issues/6） |
| task | clinic-reservation tasks.md §1.3 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> JWT の role を検証するミドルウェア（staff / clinic_admin）と、個人情報アクセスの監査ログミドルウェアを実装する。
> 全 API の OpenAPI 定義を作成し openapi-typescript の型生成を設定する。
> 公開 API 用のレートリミットは #3 の rate_limiter を再利用する。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 スタッフ/管理者 API の保護と監査の仕組みが無い。🔵 フロント型を手書きできない（Steering）ため、OpenAPI を先に確定する必要がある。🟡 JWT 検証は user-auth P1 の jwt_auth.go に依存する。

### 1.2 目標
患者トークンで /api/v1/staff/** が 403、監査対象 API で audit_logs に記録される。OpenAPI から TS 型が生成できる。

### 1.3 非目標（Out of Scope）
各 API のハンドラー実装、JWT 発行（user-auth）、通知。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし。#3 の middleware パッケージと user-auth の jwt_auth.go（P1）を前提とする。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/internal/middleware/role.go | 追加 | RequireRole(roles ...domain.Role) echo.MiddlewareFunc | 全 staff/admin ハンドラー |
| backend/internal/middleware/audit.go | 追加 | AuditLog(action string) — AuditRepository に非同期でなく同一リクエスト内で記録 | 症状メモ/待機列参照 |
| backend/api/openapi.yaml | 追加 | 設計書 §API Design の19エンドポイントとスキーマ | フロント型生成 |
| frontend/package.json（scripts）, frontend/src/api/ | 追加/修正 | openapi-typescript による生成スクリプトと生成物の置き場 | フロント全体 |
| backend/internal/middleware/rate_limiter.go 利用設定 | 修正なし | 公開 API 用の別インスタンスを生成するのみ（#3 を変更しない） | /api/v1/wait-times |

### 3.2 API 変更

全19エンドポイントを OpenAPI として新規定義（実装は後続）

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| audit_logs | #4 で作成済みのテーブルを使用 | 不要 |

---

## 4. I/O 定義

### RequireRole(staff, clinic_admin)

**入力**
```
入力: JWT クレーム role="patient" のリクエスト /api/v1/staff/queue
```

**出力**
```
出力: HTTP 403 {"error":"forbidden"}。role="staff" なら next()
```

**エラー**
トークン無し/不正 → 401（jwt_auth 側）。role が未知の値 → 403

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| echo-jwt | v4 | JWT 検証（user-auth 側） |
| ulule/limiter | v3 | 公開 API 制限 |
| openapi-typescript | 最新安定 | 型生成 |
| kin-openapi 等（OpenAPI 検証） | 要選定 | 定義の lint |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| ハンドラー内での手書き role 判定 | 認可の抜け漏れ | RequireRole ミドルウェアに一本化 |
| 監査ログへの症状メモ本文の記録 | 要配慮情報の二次拡散 | 対象 ID とアクションのみ記録 |
| API 型の手書き（フロント） | Steering 禁止 | openapi-typescript 生成 |

---

## 6. タスク詳細

### TASK-0001: ロール認可・監査ログ・OpenAPI 定義

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-005-AC-4: スタッフ機能はロールベースで制限し、患者は利用できない（Must）
- [WHEN/IF] REQ-007-AC-2: 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する（Must）
- [WHEN/IF] REQ-007-AC-3: 通信は TLS で暗号化し、個人情報へのアクセスを監査ログに記録する（Must）

**実装手順**
1. RequireRole を実装し、role の許可リストを定数化する
2. AuditLog を実装（actor, action, target のみ）しテストする
3. OpenAPI を設計書の19本分作成し lint する
4. openapi-typescript の生成スクリプトを frontend に設定する
5. 公開 API 用レートリミットの設定値を config に追加する

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | staff トークンで /api/v1/staff/queue が通る |
| 異常系 | patient トークンで 403、トークン無しは 401 |
| 境界値 | clinic_admin は staff 用 API も許可するか（要決定 🟡: 許可を推奨） |

**受け入れ基準チェックリスト**
- [ ] 正常系: staff が staff API を呼べる
- [ ] 異常系: patient が staff/admin API で 403
- [ ] 異常系: トークン無しで 401
- [ ] 正常系: 監査対象 API 呼出しで audit_logs が 1 件増える
- [ ] セキュリティ: 監査ログに symptom_note が含まれない
- [ ] 正常系: OpenAPI から TS 型が生成されビルドが通る

**依存関係**
- 前提: #3（middleware 基盤・rate_limiter）、user-auth の jwt_auth（P1）、#4（audit_logs）
- 後続: P1 の全 staff/admin ハンドラー

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
| Unit | 90%（middleware） | go test + testify + httptest |
| Integration | ルーティングに組込んだ権限マトリクス | httptest |
| E2E | P4 5.1 で実施 | Playwright |

### 8.2 テスト観点

- [ ] 正常系: staff が staff API を呼べる
- [ ] 異常系: patient が staff/admin API で 403
- [ ] 異常系: トークン無しで 401
- [ ] 正常系: 監査対象 API 呼出しで audit_logs が 1 件増える
- [ ] セキュリティ: 監査ログに symptom_note が含まれない
- [ ] 正常系: OpenAPI から TS 型が生成されビルドが通る

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
認可の誤りで権限昇格が疑われる場合、または OpenAPI の誤りで後続が詰まる場合。

### 10.2 ロールバック手順
staff/admin ルートを一時的に無効化（ルーティングから除外）してから修正。OpenAPI は前版に戻す。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| jwt_auth（user-auth P1）が未完で #6 のテストが書けない | M | H | テストでは JWT クレームを context に直接注入するヘルパーを用意 |
| OpenAPI と実装の乖離 | H | M | ハンドラー実装時に契約テスト（oapi lint + 応答検証）を追加 |
| clinic_admin の staff API 可否が曖昧で権限穴が出る | H | M | 許可マトリクスを IMP で確定しテスト化 |
| 監査ログの同期書込みで応答遅延 | M | L | INSERT 1 本のみ。性能テストで 3 秒要件を確認 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] 認可が middleware に一本化され、ハンドラー/サービスに分散していないか

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
