# Issue 構造定義: 15-clinic-reservation-wait-time-api

## 概要
待ち時間算出と公開 API

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#3.1
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-004-AC-1 | WHEN 誰かが待ち時間ページを開く THEN システムは診療科ごとの現在の待ち人数と推定待ち時間（分）を、未ログインでも表示する | Must |
| REQ-004-AC-2 | システムは待ち時間を直近の診察実績（平均診察時間）と待ち人数から算出する | Must |
| REQ-004-AC-4 | WHEN 予約済み患者が自身の予約詳細を開く THEN システムは自分の順番（あと N 人）と呼出見込み時刻を表示する | Should |
| REQ-004-AC-5 | IF 待ち時間を算出できない（診療開始前・休診・診察実績が3件未満かつ標準診察時間も未設定）THEN システムは「算出不可」と理由を表示し、数値を推測表示しない。診察実績が3件未満でも管理者設定の標準診察時間がある場合は、それを用いて算出し「標準時間による目安」と明示する | Must |
| REQ-004-AC-6 | 待ち時間表示は目安である旨の注記を併記する | Must |
| REQ-007-AC-1 | 待ち時間の公開表示には個人を特定できる情報（氏名等）を含めない | Must |

## タスク一覧
- WaitTimeService: 待ち人数 × 直近 20 件の平均所要時間 ÷ 診察中医師数（切り上げ）
- 実績 3 件未満は標準診察時間（basis=standard）、無ければ unavailable（reason 付き）
- GET /api/v1/wait-times（未ログイン可・10 秒 TTL キャッシュ・個人情報なし・注記付き）
- GET /api/v1/appointments/{id}/position（本人のみ。あと N 人と呼出見込み）

## 依存関係
- 先行タスク: #13
- 並列波形: P2

## GitHub Issue
- Issue 番号: #15
- Issue URL: https://github.com/kava2108/booking/issues/15
