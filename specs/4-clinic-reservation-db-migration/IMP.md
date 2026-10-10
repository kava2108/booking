---
kairo:
  node_id: "imp:4-clinic-reservation-db-migration"
  artifact_type: "imp"
  phase: "IMP"
  issue_id: "4-clinic-reservation-db-migration"
  feature: "clinic-reservation"
  imp_version: "1.0.0"
  status: "active"
  created_at: "2026-10-10T16:13:39+09:00"
  updated_at: "2026-10-10T16:13:39+09:00"
  drift_baseline: "N/A"
coherence:
  id: "imp:4-clinic-reservation-db-migration"
  depends_on:
    - id: "req:clinic-reservation"
      relation: "implements"
      confidence: 0.95
      required: true
    - id: "design:clinic-reservation"
      relation: "derives_from"
      confidence: 0.95
      required: true
  modules: ['backend/db/migrations/000004_create_departments_doctors.{up,down}.sql', 'backend/db/migrations/000005_create_slots.{up,down}.sql', 'backend/db/migrations/000006_create_appointments.{up,down}.sql', 'backend/db/migrations/000007_create_queue_entries.{up,down}.sql', 'backend/db/migrations/000008_create_closures_notifications_audit.{up,down}.sql']
  band: "Green"
  last_validated: "2026-10-10T16:13:39+09:00"
baton:
  phase: "imp"
  auto_step: false
  pending_label: "pending:next-phase"
  issue_number: 4
---

# IMP: DB マイグレーション作成（clinic-reservation）

| 項目 | 値 |
|---|---|
| imp_id | IMP-4-clinic-reservation-db-migration |
| imp_version | 1.0.0 |
| source_issue | #4（https://github.com/kava2108/booking/issues/4） |
| task | clinic-reservation tasks.md §1.1 |
| status | draft |
| drift_baseline | N/A（リポジトリにコミットが無いため） |
| author | Suetsugu Kawamura |

> **Executive Summary**
> departments / doctors / doctor_schedules / slots / appointments / queue_entries / closures / notifications / audit_logs を 000004 以降で作成する。
> 定員超過を防ぐ CHECK・UNIQUE・部分ユニークと、待ち時間算出用インデックスを含む。
> users.role は #1 で定義済みのため変更しない。

---

## 1. 背景と動機

### 1.1 問題定義
🔵 予約・待機列・通知・監査のテーブルが無く、clinic-reservation の実装が始められない。🟡 #1 の users が先にマージされる前提（FK 依存）。

### 1.2 目標
up/down が冪等に通り、設計書 §Database Design の制約とインデックスが全て入る。

### 1.3 非目標（Out of Scope）
sqlc クエリ（#5）、アプリコード、users テーブルの変更。

---

## 2. スニペット（変更対象の既存コード）

既存コードなし。#1 の 000001（users）を FK 参照先とする。

---

## 3. 成果物・変更スコープ

### 3.1 変更ファイル一覧

| ファイルパス | 変更種別 | 変更概要 | 影響範囲 |
|---|---|---|---|
| backend/db/migrations/000004_create_departments_doctors.{up,down}.sql | 追加 | departments（standard_consult_minutes, walkin_cutoff, walkin_capacity）, doctors, doctor_schedules | #5 |
| backend/db/migrations/000005_create_slots.{up,down}.sql | 追加 | slots（CHECK booked_count<=capacity, UNIQUE(doctor_id,start_at), IDX(department_id,start_at)） | #5 |
| backend/db/migrations/000006_create_appointments.{up,down}.sql | 追加 | appointments（部分ユニーク (patient_id,slot_id) WHERE status=booked） | #5 |
| backend/db/migrations/000007_create_queue_entries.{up,down}.sql | 追加 | queue_entries（IDX(department_id,status,sort_time), 日次 ticket_no UNIQUE） | #5 |
| backend/db/migrations/000008_create_closures_notifications_audit.{up,down}.sql | 追加 | closures, notifications（IDX(status,send_after)）, audit_logs | #5, #6 |

### 3.2 API 変更

なし

### 3.3 スキーマ変更

| テーブル/型 | 変更内容 | マイグレーション必要? |
|---|---|---|
| departments, doctors, doctor_schedules, slots, appointments, queue_entries, closures, notifications, audit_logs | 新規 | 必要 |
| users | 変更なし | 不要 |

---

## 4. I/O 定義

### 000004〜000008

**入力**
```
入力: #1 適用済みの DB
  例: schema_migrations.version = 3
```

**出力**
```
出力: 9 テーブル + 制約。version = 8
```

**エラー**
booked_count > capacity の UPDATE → CHECK 違反。同一患者の同一枠 booked 二重 → UNIQUE 違反

---

## 5. 依存関係

### 5.1 使用するライブラリ・モジュール

| ライブラリ | バージョン | 用途 |
|---|---|---|
| golang-migrate | v4 | up/down |
| PostgreSQL | 15+ | 部分ユニーク・tstzrange を利用 |

### 5.2 禁止ライブラリ・禁止パターン

| 禁止対象 | 理由 | 代替 |
|---|---|---|
| users テーブルの変更 | user-auth 領域。role は #1 で確定 | 必要なら #1 を更新 |
| down.sql の省略 | Steering | 必ず up/down ペア |
| 症状メモ列の他テーブルへの複製 | 要配慮情報の拡散防止 | appointments.symptom_note のみ |

---

## 6. タスク詳細

### TASK-0001: DB マイグレーション作成（clinic-reservation）

**完了条件（EARS 記法）**
- [WHEN/IF] REQ-002-AC-2: WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す（Must）
- [WHEN/IF] REQ-005-AC-4: スタッフ機能はロールベースで制限し、患者は利用できない（Must）
- [WHEN/IF] REQ-007-AC-3: 通信は TLS で暗号化し、個人情報へのアクセスを監査ログに記録する（Must）
- [WHEN/IF] REQ-009-AC-1: システムは電子カルテ・既存受付システムに依存せず、予約・待機列・診察実績を自システム内のみで管理する（Must）

**実装手順**
1. 000004〜000008 の up/down を設計書に沿って書く
2. slots の CHECK・UNIQUE、appointments の部分ユニークを定義する
3. queue_entries に日次ユニークな ticket_no の仕組みを入れる（service_date 列 + UNIQUE）
4. closures は期間重複検索用に tstzrange + GiST を検討する
5. up→down→up と、#1 適用後の連続適用をテストする

**例外・境界条件**
| シナリオ | 期待される振る舞い |
|---|---|
| 正常系 | #1 適用後に 000004〜8 が通る |
| 異常系 | #1 未適用だと FK エラーで失敗する（順序依存の検証） |
| 境界値 | booked_count = capacity は許可、capacity+1 は拒否 |

**受け入れ基準チェックリスト**
- [ ] 正常系: 000001〜000008 の連続 up/down/up
- [ ] 異常系: booked_count > capacity で CHECK 違反
- [ ] 異常系: 同一患者・同一枠の booked 二重で UNIQUE 違反（canceled は許可）
- [ ] 境界値: 同一日・同一診療科で ticket_no 重複が拒否される
- [ ] セキュリティ: symptom_note が appointments 以外に存在しない

**依存関係**
- 前提: #1（users FK）
- 後続: #5, #6

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
| Unit | — | — |
| Integration | 制約の違反ケース（CHECK/UNIQUE/部分ユニーク）を SQL で検証 | go test + 一時 PostgreSQL |
| E2E | 対象外 | — |

### 8.2 テスト観点

- [ ] 正常系: 000001〜000008 の連続 up/down/up
- [ ] 異常系: booked_count > capacity で CHECK 違反
- [ ] 異常系: 同一患者・同一枠の booked 二重で UNIQUE 違反（canceled は許可）
- [ ] 境界値: 同一日・同一診療科で ticket_no 重複が拒否される
- [ ] セキュリティ: symptom_note が appointments 以外に存在しない

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
制約不備で二重予約が起き得る、または up が失敗する場合。

### 10.2 ロールバック手順
`migrate down 5`（000008→000004）。未デプロイのためデータ移行なし。

### 10.3 ロールバック所要時間
- 自動: 該当なし
- 手動: 15 分程度（未デプロイ段階）

---

## 11. リスク

| リスク | 影響度 | 発生確率 | 対策 |
|---|---|---|---|
| #1 が未マージのまま #4 を適用して FK 失敗 | H | M | CI で 000001 から連続適用。依存を Issue に明記 |
| 番号競合（user-auth 追加マイグレーション） | M | M | user-auth 側の追加は 000009 以降などレーン分けを合意 |
| 部分ユニークの条件ミスで再予約が不可能になる | M | M | canceled 後の再予約テストを追加 |
| closures の期間型選択（tstzrange）が sqlc で扱いづらい | L | M | #5 で pgtype.Range を確認、困難なら start/end 列に変更 |

---

## 12. レビュアーチェックリスト

### 12.1 アーキテクチャレビュー
- [ ] 変更が既存アーキテクチャパターン（Steering structure.md）と整合している
- [ ] 新たな外部依存の追加が適切に評価されている
- [ ] スケーラビリティへの影響が分析されている
- [ ] 設計の複雑度が最小限に抑えられている
- [ ] ロールバック計画が実行可能である
- [ ] slots.booked_count と appointments の整合を DB 制約だけで守れる範囲と service 層に委ねる範囲が分かれているか

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
