# Issue 構造定義: 17-clinic-reservation-frontend-booking

## 概要
フロント: 検索・予約・マイページ

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#3.3
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-001-AC-1 | WHEN 患者が診療科と日付を指定する THEN システムは予約可能な枠を時刻順に表示する | Must |
| REQ-001-AC-2 | WHEN 医師を指名する THEN システムはその医師の枠のみを表示する | Must |
| REQ-001-AC-3 | WHEN 枠が満席・休診・受付締切後である THEN システムはその枠を選択不可として表示する | Must |
| REQ-002-AC-1 | WHEN ログイン済み患者が空き枠・来院区分（初診/再診）・症状メモを送信する THEN システムは予約を確定し予約番号を発行する | Must |
| REQ-003-AC-1 | WHEN 患者がマイページを開く THEN システムは自分の予約一覧（今後・過去）を表示する | Must |
| REQ-003-AC-2 | WHEN 患者がキャンセル期限内に予約をキャンセルする THEN システムは枠を解放し通知を送信する | Must |
| REQ-003-AC-3 | WHEN キャンセル期限を過ぎている THEN システムはキャンセル不可とし、医療機関への連絡方法を案内する | Should |
| REQ-003-AC-4 | WHEN 患者が日時を変更する THEN システムは新枠の確保と旧枠の解放を原子的に行う | Should |

## タスク一覧
- features/slot, features/appointment の React Query フック（api/ に集約）
- 診療科・医師・日付の検索、予約フォーム（react-hook-form + zod）、満席時の再選択導線
- マイページ（予約一覧・変更・キャンセル、期限超過時の案内）

## 依存関係
- 先行タスク: #11, #12
- 並列波形: P2

## GitHub Issue
- Issue 番号: #17
- Issue URL: https://github.com/kava2108/booking/issues/17
