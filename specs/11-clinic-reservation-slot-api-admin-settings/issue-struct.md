# Issue 構造定義: 11-clinic-reservation-slot-api-admin-settings

## 概要
診療科・医師・枠 API と管理者設定

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#2.1
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-001-AC-1 | WHEN 患者が診療科と日付を指定する THEN システムは予約可能な枠を時刻順に表示する | Must |
| REQ-001-AC-2 | WHEN 医師を指名する THEN システムはその医師の枠のみを表示する | Must |
| REQ-001-AC-3 | WHEN 枠が満席・休診・受付締切後である THEN システムはその枠を選択不可として表示する | Must |
| REQ-006-AC-1 | WHEN 管理者が診療時間・枠の長さ・定員を設定する THEN システムは該当日の予約枠を生成する | Must |
| REQ-006-AC-2 | WHEN 管理者が予約受付期間・キャンセル期限を設定する THEN システムは以降の予約に適用する | Should |

## タスク一覧
- GET /api/v1/departments, /api/v1/doctors, /api/v1/slots（満席・休診・締切を選択不可として返す）
- SlotService: doctor_schedules からの枠生成と日次ジョブ
- PUT /api/v1/admin/schedules, /api/v1/admin/settings（診療時間・枠長・定員・受付期間・キャンセル期限・標準診察時間）

## 依存関係
- 先行タスク: #4, #5, #6
- 並列波形: P1

## GitHub Issue
- Issue 番号: #11
- Issue URL: https://github.com/kava2108/booking/issues/11
