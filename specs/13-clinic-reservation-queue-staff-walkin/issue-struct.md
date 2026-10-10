# Issue 構造定義: 13-clinic-reservation-queue-staff-walkin

## 概要
待機列とスタッフ操作・当日受付

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#2.3
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-005-AC-1 | WHEN スタッフが患者の来院受付を記録する THEN システムはその患者を待機列に追加する | Must |
| REQ-005-AC-2 | WHEN スタッフが呼出・診察開始・診察完了を記録する THEN システムは待機列と診察実績を更新し、待ち時間表示へ反映する | Must |
| REQ-005-AC-4 | スタッフ機能はロールベースで制限し、患者は利用できない | Must |
| REQ-008-AC-1 | WHEN 患者またはスタッフが当日受付を登録する THEN システムは診療科の待機列の末尾に追加し受付番号を発行する | Must |
| REQ-008-AC-2 | 待機列は予約患者の予約時刻と当日受付患者の受付順を合わせた順序で管理し、待ち時間の算出に含める | Must |
| REQ-008-AC-3 | IF 受付終了時刻を過ぎている・定員に達した THEN システムは当日受付を拒否し理由を表示する | Must |
| REQ-007-AC-2 | 症状メモ等の要配慮個人情報は、本人と権限を持つスタッフ以外が参照できないようアクセス制御する | Must |

## タスク一覧
- QueueService: 状態遷移（waiting→called→in_consultation→done）と不正遷移の 409
- 予約患者の来院受付（sort_time = GREATEST(予約時刻, 受付時刻)）
- 当日受付（患者本人 / スタッフ代行・無アカウント可）、受付番号発行、締切・定員の判定
- GET /api/v1/staff/queue（氏名・症状メモを含む）、call / start / complete の各 API

## 依存関係
- 先行タスク: #4, #5, #6
- 並列波形: P1

## GitHub Issue
- Issue 番号: #13
- Issue URL: https://github.com/kava2108/booking/issues/13
