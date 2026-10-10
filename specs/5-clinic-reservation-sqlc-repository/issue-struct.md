# Issue 構造定義: 5-clinic-reservation-sqlc-repository

## 概要
sqlc クエリと repository interface

## フィーチャー参照
- フィーチャー: clinic-reservation
- タスク参照: tasks.md#1.2
- 要件参照: .kiro/specs/clinic-reservation/requirements.md
- 設計参照: .kiro/specs/clinic-reservation/design.md

## 受け入れ基準
| AC-ID | EARS 記法 | 優先度 |
|-------|----------|-------|
| REQ-002-AC-2 | WHEN 同一枠に複数の予約が同時に送信された THEN システムは排他制御により定員を超える予約を確定せず、後続に HTTP 409 を返す | Must |
| REQ-004-AC-2 | システムは待ち時間を直近の診察実績（平均診察時間）と待ち人数から算出する | Must |

## タスク一覧
- 読み取り用・書き込み用 SQL を分けて作成（slots 条件付き UPDATE、queue 集計、通知の SKIP LOCKED 取得を含む）
- Queries を包む repository interface を定義し mockery でモック生成

## 依存関係
- 先行タスク: なし（前提: user-auth 完了）
- 並列波形: P0

## GitHub Issue
- Issue 番号: #5
- Issue URL: https://github.com/kava2108/booking/issues/5
