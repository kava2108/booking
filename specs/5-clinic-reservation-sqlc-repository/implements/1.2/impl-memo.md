---
issue_id: 5-clinic-reservation-sqlc-repository
task_id: 1.2
created_at: 2026-10-10T16:49:22+09:00
---

# 実装判断メモ: 5-clinic-reservation-sqlc-repository / 1.2

## 採用したアプローチ

- **ListOpenSlots → ListSlots**: 満席・休診の枠も返し、選択不可の判定は service が行う（REQ-001-AC-3 は「選択不可として表示」のため。open のみだと表示できない）。
- **\*db.Queries を実装として使う**: interface は sqlc 生成メソッドの部分集合。薄い adapter を書かず、コンパイル時アサーションで保証。service は interface 経由でのみ受け取る（Steering）。
- **Store.WithinTx + Repositories**: 予約確定は「枠確保 + 予約作成」を 1 Tx で行う必要があるため、Tx 版の Repositories を fn に渡す。service のユニットテストは MockTxRunner で足りる。callback の error はラップせず返す（pgx.ErrNoRows を判定できるように）。
- **状態遷移は SQL で表現**: `UPDATE … WHERE status = 現在値 RETURNING`。0 行 = 不正遷移 → service が 409。競合しても二重遷移しない。
- **通知はリース方式**: 取得時に send_after を先送りし attempts を加算。Tx を保持しないので外部 SMTP の遅延でロックが伸びない。リース切れで再取得される。最大試行回数の判定（5 回で failed）は worker 側。
- **ticket_no は MAX+1**: 衝突は UNIQUE (23505) で検出して service が再試行（テストで検証）。
- **CountOnDutyDoctors は Asia/Tokyo 固定**: 単一医療機関・国内運用の前提。多拠点化するなら設定化。
- **symptom_note の露出範囲**: ListAppointmentsByPatient（本人）と ListStaffQueue（スタッフ）のみ。公開向けクエリ（CountAhead/CountWaiting 等）は件数のみ。
- **テストを専用スキーマ化**: 当初は public を毎回再作成したが、`go test ./...` のパッケージ並行実行で干渉したため pgtest.NewPool（テストごとのスキーマ）に変更。#2 のテストも同ヘルパーへ移行。

## 検討した代替案

| 代替案 | 不採用の理由 |
|---|---|
| ドメイン型を別途定義して adapter で変換 | 現段階では二重定義になり YAGNI。必要になったら interface の裏で差し替え可能 |
| 通知を Tx 保持（FOR UPDATE のまま送信） | SMTP 遅延で長時間ロック。リースの方が堅い |
| ticket_no 用の DB シーケンス | 日次・診療科別リセットが面倒 |

## 既存コードへの影響

#2 のテストヘルパーを変更（テスト内容は同一）。生成コードの models.go は clinic テーブルの型が増えた。

## TODO・未解決事項

- [ ] 予約の受付期間（booking_window_days）・キャンセル期限の判定は service（P1 2.2）
- [ ] 待機列の sort_time 規則（GREATEST(予約時刻, 受付時刻)）は service が設定する
- [ ] 診療科単位の休診が必要になれば closures に department_id を追加
- [ ] password_hash を含む db.User を handler に出さない型分離（user-auth P1）
