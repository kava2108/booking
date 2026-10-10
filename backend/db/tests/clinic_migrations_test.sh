#!/usr/bin/env bash
# clinic-reservation マイグレーション検証（IMP 4-clinic-reservation-db-migration TASK-0001）
# 使い方: TEST_DATABASE_URL=postgres://user@host:port/booking_test ./clinic_migrations_test.sh
source "$(dirname "$0")/lib.sh"

CLINIC_TABLES="'clinic_settings','departments','doctors','doctor_schedules','slots','appointments','queue_entries','closures','notifications','audit_logs'"
ctables() { sql "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ($CLINIC_TABLES)"; }

echo "# up/down ペア"
n=0; for u in "$DIR"/0000{04,05,06,07,08}_*.up.sql; do [ -f "$u" ] || continue; n=$((n+1)); [ -f "${u%.up.sql}.down.sql" ] && ok "down あり: $(basename "$u")" || ng "down なし: $(basename "$u")"; done
[ "$n" -eq 5 ] && ok "000004〜000008 が揃っている" || ng "000004〜000008 のファイル数: $n"

reset
echo "# 000001〜000008 連続 up"
up && ok "up 成功" || ng "up 失敗"
[ "$(ctables)" = "10" ] && ok "clinic テーブル 10 本" || ng "clinic テーブル数: $(ctables)"

# --- フィクスチャ ---
sql "INSERT INTO users(email,password_hash,name) VALUES ('p1@example.com','h','患者1'),('p2@example.com','h','患者2')" >/dev/null
P1=$(sql "SELECT id FROM users WHERE email='p1@example.com'"); P2=$(sql "SELECT id FROM users WHERE email='p2@example.com'")
sql "INSERT INTO departments(name) VALUES ('内科')" >/dev/null
DEP=$(sql "SELECT id FROM departments WHERE name='内科'")
sql "INSERT INTO doctors(department_id,name) VALUES ($DEP,'山田')" >/dev/null
DOC=$(sql "SELECT id FROM doctors WHERE name='山田'")

echo "# 診療科・医師・スケジュール・設定 (REQ-006)"
expect_fail "診療科名の重複" "INSERT INTO departments(name) VALUES ('内科')" "duplicate key"
expect_fail "標準診察時間は正の値" "INSERT INTO departments(name,standard_consult_minutes) VALUES ('外科',0)" "check constraint"
expect_fail "weekday は 0〜6" "INSERT INTO doctor_schedules(doctor_id,weekday,start_time,end_time,slot_minutes,capacity) VALUES ($DOC,7,'09:00','12:00',30,1)" "check constraint"
expect_fail "end_time > start_time" "INSERT INTO doctor_schedules(doctor_id,weekday,start_time,end_time,slot_minutes,capacity) VALUES ($DOC,1,'12:00','09:00',30,1)" "check constraint"
expect_ok   "スケジュール登録" "INSERT INTO doctor_schedules(doctor_id,weekday,start_time,end_time,slot_minutes,capacity) VALUES ($DOC,1,'09:00','12:00',30,2)"
[ "$(count clinic_settings)" = "1" ] && ok "clinic_settings に既定行が 1 件" || ng "clinic_settings 件数: $(count clinic_settings)"
expect_fail "clinic_settings は 1 行のみ" "INSERT INTO clinic_settings(id) VALUES (2)" "check constraint"

echo "# slots (REQ-002-AC-2)"
expect_ok   "枠登録 capacity=1" "INSERT INTO slots(department_id,doctor_id,start_at,capacity) VALUES ($DEP,$DOC,'2026-10-12 09:00+09',1)"
SLOT=$(sql "SELECT id FROM slots LIMIT 1")
[ "$(sql "SELECT booked_count FROM slots WHERE id='$SLOT'")" = "0" ] && ok "booked_count 既定 0" || ng "booked_count 既定"
expect_ok   "booked_count = capacity は許可" "UPDATE slots SET booked_count=1 WHERE id='$SLOT'"
expect_fail "booked_count > capacity は拒否" "UPDATE slots SET booked_count=2 WHERE id='$SLOT'" "check constraint"
expect_fail "booked_count 負数は拒否" "UPDATE slots SET booked_count=-1 WHERE id='$SLOT'" "check constraint"
expect_fail "同一医師・同一時刻は重複不可" "INSERT INTO slots(department_id,doctor_id,start_at,capacity) VALUES ($DEP,$DOC,'2026-10-12 09:00+09',1)" "duplicate key"
expect_fail "status は open/closed" "UPDATE slots SET status='x' WHERE id='$SLOT'" "check constraint"
sql "UPDATE slots SET booked_count=0 WHERE id='$SLOT'" >/dev/null

echo "# appointments (REQ-002, 003)"
A="INSERT INTO appointments(appointment_no,patient_id,slot_id,visit_type,symptom_note)"
expect_ok   "予約登録" "$A VALUES ('A-1','$P1','$SLOT','revisit','咳')"
expect_fail "同一患者・同一枠の booked 二重" "$A VALUES ('A-2','$P1','$SLOT','revisit','')" "duplicate key"
expect_ok   "別患者は同一枠に予約可" "$A VALUES ('A-3','$P2','$SLOT','first','')"
expect_fail "appointment_no 重複" "$A VALUES ('A-1','$P2','$SLOT','first','')" "duplicate key"
expect_fail "visit_type 不正" "$A VALUES ('A-4','$P2','$SLOT','x','')" "check constraint"
expect_ok   "キャンセル" "UPDATE appointments SET status='canceled', canceled_at=now() WHERE appointment_no='A-1'"
expect_ok   "キャンセル後の再予約は可能" "$A VALUES ('A-5','$P1','$SLOT','revisit','')"
expect_fail "status 不正" "UPDATE appointments SET status='x' WHERE appointment_no='A-5'" "check constraint"
expect_fail "予約のある患者は削除不可 (RESTRICT)" "DELETE FROM users WHERE id='$P1'" "foreign key"

echo "# queue_entries (REQ-005, 008)"
APP=$(sql "SELECT id FROM appointments WHERE appointment_no='A-5'")
Q="INSERT INTO queue_entries(department_id,appointment_id,patient_id,display_name,source,service_date,ticket_no,sort_time,checked_in_at)"
expect_ok   "予約患者の受付" "$Q VALUES ($DEP,'$APP','$P1',NULL,'reservation','2026-10-12',1,now(),now())"
expect_fail "同一予約の二重受付" "$Q VALUES ($DEP,'$APP','$P1',NULL,'reservation','2026-10-12',2,now(),now())" "duplicate key"
expect_ok   "当日受付（無アカウント・氏名のみ）" "$Q VALUES ($DEP,NULL,NULL,'電話 太郎','walk_in','2026-10-12',2,now(),now())"
expect_fail "同日・同診療科の受付番号重複" "$Q VALUES ($DEP,NULL,NULL,'x','walk_in','2026-10-12',2,now(),now())" "duplicate key"
expect_ok   "翌日は同じ番号を再利用可" "$Q VALUES ($DEP,NULL,NULL,'x','walk_in','2026-10-13',2,now(),now())"
expect_fail "reservation は appointment_id 必須" "$Q VALUES ($DEP,NULL,'$P1',NULL,'reservation','2026-10-12',9,now(),now())" "check constraint"
expect_fail "walk_in は appointment_id 不可" "$Q VALUES ($DEP,'$APP','$P1',NULL,'walk_in','2026-10-12',9,now(),now())" "check constraint"
expect_fail "patient_id と display_name が両方 NULL" "$Q VALUES ($DEP,NULL,NULL,NULL,'walk_in','2026-10-12',9,now(),now())" "check constraint"
expect_fail "status 不正" "UPDATE queue_entries SET status='x' WHERE ticket_no=1" "check constraint"
[ "$(sql "SELECT status FROM queue_entries WHERE ticket_no=1 AND service_date='2026-10-12'")" = "waiting" ] && ok "status 既定 waiting" || ng "status 既定"
idxdef() { sql "SELECT indexdef FROM pg_indexes WHERE tablename='$1'"; }
grep -q "(department_id, status, sort_time)" <<<"$(idxdef queue_entries)" && ok "queue_entries(department_id,status,sort_time)" || ng "queue 用インデックスなし"

echo "# closures / notifications / audit_logs"
expect_ok   "医師の休診" "INSERT INTO closures(doctor_id,period,reason) VALUES ($DOC, tstzrange('2026-10-12 09:00+09','2026-10-12 12:00+09'), '学会')"
expect_ok   "全体休診 (doctor_id NULL)" "INSERT INTO closures(doctor_id,period,reason) VALUES (NULL, tstzrange('2026-10-14 00:00+09','2026-10-15 00:00+09'), '臨時休診')"
[ "$(sql "SELECT count(*) FROM closures WHERE period @> '2026-10-12 10:00+09'::timestamptz")" = "1" ] && ok "期間検索" || ng "期間検索"
expect_ok   "通知登録" "INSERT INTO notifications(user_id,kind,payload) VALUES ('$P2','appointment_confirmed','{\"appointment_no\":\"A-3\"}')"
[ "$(sql "SELECT status||attempts FROM notifications LIMIT 1")" = "pending0" ] && ok "通知 既定 pending/0" || ng "通知 既定値"
expect_fail "通知 status 不正" "UPDATE notifications SET status='x'" "check constraint"
grep -q "(status, send_after)" <<<"$(idxdef notifications)" && ok "notifications(status,send_after)" || ng "通知用インデックスなし"
expect_ok   "監査ログ登録" "INSERT INTO audit_logs(actor_id,action,target) VALUES ('$P2','view_symptom_note','appointment:A-3')"
expect_ok   "ユーザー削除後も監査ログは残る (actor NULL)" "DELETE FROM notifications; DELETE FROM queue_entries; DELETE FROM appointments WHERE patient_id='$P2'; DELETE FROM users WHERE id='$P2'"
[ "$(count audit_logs)" = "1" ] && [ "$(sql "SELECT actor_id IS NULL FROM audit_logs")" = "t" ] && ok "監査ログ保持" || ng "監査ログ保持"

echo "# 症状メモは appointments のみ (REQ-007-AC-2)"
[ "$(sql "SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND column_name LIKE '%symptom%'")" = "1" ] && ok "symptom 列は 1 箇所" || ng "symptom 列が複数"

echo "# users は変更されていない"
[ "$(sql "SELECT count(*) FROM information_schema.columns WHERE table_name='users'")" = "8" ] && ok "users 8 列のまま" || ng "users の列数が変化"

echo "# down / 再 up (冪等性)"
down && ok "down 成功" || ng "down 失敗"
[ "$(ctables)" = "0" ] && ok "clinic テーブル全削除" || ng "残存: $(ctables)"
[ "$(sql "SELECT count(*) FROM information_schema.tables WHERE table_name='users'")" = "0" ] && ok "全 down で users も削除" || ng "users 残存"
up && ok "再 up 成功" || ng "再 up 失敗"
[ "$(ctables)" = "10" ] && ok "再 up 後 10 テーブル" || ng "再 up 後: $(ctables)"

echo; echo "結果: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
