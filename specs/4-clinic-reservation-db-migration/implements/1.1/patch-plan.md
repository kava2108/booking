---
kairo:
  node_id: "impl:4-clinic-reservation-db-migration:1.1"
  artifact_type: "patch_plan"
  phase: "IMP"
  issue_id: "4-clinic-reservation-db-migration"
  task_id: "1.1"
  created_at: "2026-10-10T16:30:56+09:00"
coherence:
  id: "impl:4-clinic-reservation-db-migration:1.1"
  depends_on:
    - id: "imp:4-clinic-reservation-db-migration"
      relation: "implements"
      confidence: 0.95
      required: true
  band: "Green"
---
issue_id: 4-clinic-reservation-db-migration
task_id: 1.1
imp_version: 1.0.0
mode: tdd
dry_run: false
created_at: 2026-10-10T16:30:56+09:00

# パッチ計画: 4-clinic-reservation-db-migration / 1.1

## 変更対象ファイル

| ファイルパス | 変更種別 | 変更概要 | AC 対応 |
|---|---|---|---|
| backend/db/migrations/000004_create_departments_doctors.{up,down}.sql | 新規 | clinic_settings, departments, doctors, doctor_schedules | REQ-006-AC-1/2 |
| backend/db/migrations/000005_create_slots.{up,down}.sql | 新規 | slots（CHECK / UNIQUE / index） | REQ-002-AC-2 |
| backend/db/migrations/000006_create_appointments.{up,down}.sql | 新規 | appointments（部分ユニーク） | REQ-002-AC-2 |
| backend/db/migrations/000007_create_queue_entries.{up,down}.sql | 新規 | queue_entries | REQ-005-AC-4 相当の基盤, REQ-008 |
| backend/db/migrations/000008_create_closures_notifications_audit.{up,down}.sql | 新規 | closures, notifications, audit_logs | REQ-007-AC-3, REQ-009-AC-1 |
| backend/db/tests/clinic_migrations_test.sh | 新規 | 制約・冪等性の検証 | 全 AC |
| backend/db/tests/lib.sh | 新規 | テスト共通関数（#1 のスクリプトから抽出） | — |
| backend/db/tests/migrations_test.sh | 変更 | lib.sh を source するよう変更（挙動は同一） | — |
| .github/workflows/ci.yml | 変更 | clinic テストを追加 | — |

## テストファイル

| テストファイル | カバーする AC | 実行コマンド |
|---|---|---|
| backend/db/tests/clinic_migrations_test.sh | REQ-002-AC-2, REQ-005-AC-4, REQ-007-AC-3, REQ-009-AC-1 | TEST_DATABASE_URL=postgres://…/booking_test backend/db/tests/clinic_migrations_test.sh |

## 完了チェックリスト

- [x] 全 AC に対応する実装が完了した
- [x] TDD Red フェーズのテストが全て通過した（pass=59 fail=0、PostgreSQL 16）
- [x] 既存テストが破壊されていない（#1 のテスト pass=37 fail=0）
- [x] IMP の変更ファイル一覧と実際の変更が一致している（clinic_settings 追加、lib.sh 抽出は差分）
- [ ] PostgreSQL 15 / golang-migrate での確認（CI）
