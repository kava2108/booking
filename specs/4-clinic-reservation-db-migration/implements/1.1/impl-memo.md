---
issue_id: 4-clinic-reservation-db-migration
task_id: 1.1
created_at: 2026-10-10T16:30:56+09:00
---

# 実装判断メモ: 4-clinic-reservation-db-migration / 1.1

## 採用したアプローチ

- **clinic_settings（設計書にない追加テーブル）**: REQ-006-AC-2 の「受付期間・キャンセル期限」と `PUT /api/admin/settings` の保存先が設計書の ER 図に無かったため、単一行テーブルとして追加。design.md の ER 図への反映が必要（TODO）。
- **主キー**: 診療科・医師などマスタは bigint IDENTITY、予約・枠・待機列・通知は uuid（設計書どおり）。
- **appointments.patient_id は RESTRICT**: 医療・予約記録を患者削除で連鎖消去しない。監査ログは `ON DELETE SET NULL` で記録を残し、通知は `CASCADE`。
- **queue_entries の整合性を CHECK で表現**: source=reservation ⇔ appointment_id あり、patient_id か display_name のどちらか必須。サービス層のバグでも不正行が入らない。
- **ticket_no は (department_id, service_date, ticket_no) で UNIQUE**: 採番ロジックは #5 / service 側（日次カウンタ）。
- **closures.doctor_id NULL = 全体休診**: 診療科単位の休診は未対応（必要になれば department_id を追加）。
- **ticket 用シーケンスは作らない**: 競合時は UNIQUE 違反でリトライする前提。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| 休診を slots.status のみで表現 | 期間・理由・通知対象の再計算ができない。closures を正とし slots は結果 |
| 設定を departments に分散 | キャンセル期限等は医療機関単位（現状単一デプロイ）。単一行が単純 |
| ticket_no を SERIAL | 日次リセットと診療科別採番ができない |

## 既存コードへの影響

users / refresh_tokens / password_reset_tokens は変更なし。#5 は `schema: db/migrations` を読むため、ファイル名の連番を変えないこと。

## TODO・未解決事項

- [x] design.md の ER 図に clinic_settings を追記した
- [ ] ticket_no の採番方式（#5 のクエリで MAX+1 + リトライ か、日次カウンタ表）を決める
- [ ] 診療科単位の休診が必要か確認する
