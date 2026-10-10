# Issue 構造定義: 16-clinic-reservation-closures

## 概要
休診・医師不在の設定

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#3.2
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-005-AC-3 | WHEN スタッフが休診・臨時休診・医師不在を設定する THEN システムは該当枠を予約不可にし、既存予約者へ通知する | Must |

## タスク一覧
- POST /api/v1/staff/closures: 該当枠を closed にし、既存予約者へ通知を登録
- 休診中は待ち時間を unavailable(closed) にする

## 依存関係
- 先行タスク: #11, #12, #14
- 並列波形: P2

## GitHub Issue
- Issue 番号: #16
- Issue URL: https://github.com/kava2108/booking/issues/16
