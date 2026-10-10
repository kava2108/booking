# Red フェーズ: 4-clinic-reservation-db-migration / 1.1

テスト: `backend/db/tests/clinic_migrations_test.sh`（共通関数は `lib.sh`）

000004〜000008 未作成の状態で実行 → 全ケースが失敗（relation does not exist）。

| 観点 | ケース | AC |
|---|---|---|
| 構造 | 000004〜000008 の up/down ペア、10 テーブル作成 | — |
| 設定・スケジュール | 診療科名重複 / 標準診察時間 >0 / weekday 0-6 / end>start / clinic_settings 単一行 | REQ-006 |
| slots | booked_count 既定 0 / =capacity 許可 / >capacity 拒否 / 負数拒否 / (doctor,start) 重複 / status | REQ-002-AC-2 |
| appointments | 同一患者・同一枠の booked 二重拒否 / キャンセル後再予約可 / 別患者可 / no 重複 / 患者削除は RESTRICT | REQ-002, 003 |
| queue_entries | 二重受付拒否 / 無アカウント当日受付 / 日次 ticket_no / source と appointment_id の整合 / identity / 既定 waiting / 並び用インデックス | REQ-005, 008 |
| closures / notifications / audit | 期間検索 / 全体休診 / 通知既定値 / ユーザー削除後も監査ログ保持 | REQ-005-AC-3, REQ-007-AC-3 |
| 個人情報 | symptom 列が appointments のみ | REQ-007-AC-2 |
| 非干渉 | users は 8 列のまま | — |
| 冪等 | 全 down → 全削除 → 再 up | — |
