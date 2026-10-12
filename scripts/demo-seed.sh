#!/usr/bin/env bash
# 紹介サイト・画面集用のデモデータを、開発用 DB に作り直す（日中の現実的な診療時間）。
#   使い方: scripts/demo-seed.sh        （平日の 14:00〜17:00 ごろに実行すると、午後の待機列が自然に見える）
# 前提: 開発用の PostgreSQL(docker: booking-dev-pg) と API(8080) が起動済み。登録済みの admin/staff@dev.test を使う。
# 注意: 開発用 DB の予約・待機列・枠・診療スケジュールを消して作り直す（ユーザーは残す）。
set -euo pipefail

PG=${PG:-booking-dev-pg}
DB=${DB:-booking_dev}
API=${API:-http://127.0.0.1:8080/api/v1}
PW='Dev-Password-123!'
psql() { docker exec -i "$PG" psql -U postgres -d "$DB" -qtA -v ON_ERROR_STOP=1 "$@"; }
today=$(TZ=Asia/Tokyo date +%F)

login() { curl -fs -X POST "$API/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"$PW\"}" | jq -r .access_token; }
call() { # call TOKEN METHOD PATH [JSON]
  curl -fs -X "$2" "$API$3" -H "Authorization: Bearer $1" -H 'Content-Type: application/json' ${4:+-d "$4"}
}

echo "==> アカウント（患者 10 名）"
names=("田中 一郎" "鈴木 花子" "高橋 健" "伊藤 美咲" "渡辺 誠" "小林 由美" "加藤 大輔" "吉田 恵" "山本 浩" "中村 彩")
for i in "${!names[@]}"; do
  n=$((i + 1))
  curl -s -o /dev/null -X POST "$API/auth/register" -H 'Content-Type: application/json' \
    -d "{\"email\":\"demo$n@dev.test\",\"password\":\"$PW\",\"name\":\"${names[$i]}\"}" || true
done

echo "==> 初期化（予約・待機列・枠・スケジュール）と診療科・医師"
psql <<'SQL' >/dev/null
TRUNCATE audit_logs, notifications, queue_entries, appointments, slots, closures, doctor_schedules RESTART IDENTITY CASCADE;
INSERT INTO departments (name, standard_consult_minutes) VALUES ('内科', 10), ('小児科', 15), ('整形外科', 12) ON CONFLICT (name) DO NOTHING;
UPDATE doctors SET name = '山田 健一' WHERE id = 1;
UPDATE doctors SET name = '佐藤 美穂' WHERE id = 2;
INSERT INTO doctors (department_id, name)
  SELECT d.id, '高木 修' FROM departments d WHERE d.name = '整形外科'
  AND NOT EXISTS (SELECT 1 FROM doctors x WHERE x.department_id = d.id);
SQL
dept() { psql -c "SELECT id FROM departments WHERE name='$1'"; }
doc() { psql -c "SELECT id FROM doctors WHERE department_id=$1 ORDER BY id LIMIT 1"; }
NAIKA=$(dept 内科); SHONI=$(dept 小児科); SEIKEI=$(dept 整形外科)
D_NAIKA=$(doc "$NAIKA"); D_SHONI=$(doc "$SHONI"); D_SEIKEI=$(doc "$SEIKEI")

ADMIN=$(login admin@dev.test); STAFF=$(login staff@dev.test)

echo "==> 受付期間・キャンセル期限・当日受付の締切"
call "$ADMIN" PUT /admin/settings "{\"cancel_deadline_minutes\":120,\"booking_window_days\":30,\"departments\":[
  {\"id\":$NAIKA,\"standard_consult_minutes\":10,\"walkin_cutoff\":\"16:30\"},
  {\"id\":$SHONI,\"standard_consult_minutes\":15,\"walkin_cutoff\":\"16:00\"},
  {\"id\":$SEIKEI,\"standard_consult_minutes\":12,\"walkin_cutoff\":\"16:30\"}]}" >/dev/null

echo "==> 診療スケジュール（午前 9:00-12:00 / 午後 14:00-17:30、土曜は午前のみ、日曜休診）"
mk() { # mk "曜日..." 開始 終了 枠 定員
  local w out=""
  for w in $1; do out+="$w|$2|$3|$4|$5 "; done
  echo "$out"
}
sched() {
  local items="" w s e m c spec
  for spec in $2; do
    IFS='|' read -r w s e m c <<<"$spec"
    items+="{\"weekday\":$w,\"start_time\":\"$s\",\"end_time\":\"$e\",\"slot_minutes\":$m,\"capacity\":$c},"
  done
  call "$ADMIN" PUT /admin/schedules "{\"doctor_id\":$1,\"schedules\":[${items%,}]}" >/dev/null
}
sched "$D_NAIKA"  "$(mk '1 2 3 4 5' 09:00 12:00 30 3) $(mk '1 2 3 4 5' 14:00 17:30 30 3) $(mk 6 09:00 12:00 30 3)"
sched "$D_SHONI"  "$(mk '1 2 3 4 5' 09:00 12:00 30 2) $(mk '1 3 5' 14:30 17:00 30 2) $(mk 6 09:00 12:00 30 2)"
sched "$D_SEIKEI" "$(mk '1 3 5' 09:00 12:00 30 2) $(mk '1 3 5' 14:00 17:00 30 2) $(mk 2 09:00 12:00 30 2)"

slot_at() { # slot_at DEPT DOCTOR DATE HH:MM -> slot id
  psql -c "SELECT id FROM slots WHERE department_id=$1 AND doctor_id=$2 AND start_at = ('$3 $4:00+09')::timestamptz"
}
patient_id() { psql -c "SELECT id FROM users WHERE email='demo$1@dev.test'"; }

echo "==> 今日（$today）の午前の履歴（診察済み）"
# 予約由来 6 件と当日受付 3 件。診察時間は 8〜13 分、午前の診療が終わった状態にする。
hist() { # hist DEPT DOCTOR TICKET SOURCE HH:MM(受付) 開始 完了 patient番号|表示名 [予約HH:MM 症状]
  local dept=$1 doc=$2 t=$3 src=$4 cin=$5 st=$6 en=$7 who=$8 slot=${9:-} note=${10:-}
  local pid="NULL" dname="NULL" aid="NULL" sid
  if [[ $who =~ ^[0-9]+$ ]]; then pid="'$(patient_id "$who")'"; else dname="'$who'"; fi
  if [ "$src" = reservation ]; then
    sid=$(slot_at "$dept" "$doc" "$today" "$slot")
    aid=$(psql -c "INSERT INTO appointments (appointment_no, patient_id, slot_id, visit_type, symptom_note, status)
      VALUES ('A-${today//-/}-$(printf '%04d' $((RANDOM % 9000 + 1000)))', $pid, '$sid', 'revisit', '$note', 'completed') RETURNING id")
    psql -c "UPDATE slots SET booked_count = booked_count + 1 WHERE id='$sid'" >/dev/null
    aid="'$aid'"
  fi
  psql -c "INSERT INTO queue_entries (department_id, appointment_id, patient_id, display_name, source, service_date, ticket_no, sort_time, status, checked_in_at, started_at, completed_at)
    VALUES ($dept, $aid, $pid, $dname, '$src', '$today', $t, '$today $cin+09', 'done', '$today $cin+09', '$today $st+09', '$today $en+09')" >/dev/null
}
hist "$NAIKA" "$D_NAIKA" 1 reservation 08:52 09:01 09:11 1 09:00 "高血圧の定期受診"
hist "$NAIKA" "$D_NAIKA" 2 reservation 09:22 09:31 09:42 2 09:30 "咳が続く"
hist "$NAIKA" "$D_NAIKA" 3 walk_in     09:40 09:47 09:56 "木村 和子"
hist "$NAIKA" "$D_NAIKA" 4 reservation 10:05 10:09 10:21 3 10:00 "健康診断の結果相談"
hist "$NAIKA" "$D_NAIKA" 5 walk_in     10:18 10:24 10:33 "森 達也"
hist "$NAIKA" "$D_NAIKA" 6 reservation 10:52 10:57 11:07 4 11:00 "のどの痛み・発熱"
hist "$NAIKA" "$D_NAIKA" 7 reservation 13:55 14:02 14:12 7 14:00 "糖尿病の経過観察"
hist "$SHONI" "$D_SHONI" 1 reservation 09:03 09:10 09:26 5 09:00 "予防接種の相談"
hist "$SHONI" "$D_SHONI" 2 reservation 10:27 10:33 10:47 6 10:30 "発熱・鼻水"
hist "$SEIKEI" "$D_SEIKEI" 1 walk_in   09:15 09:20 09:31 "岡田 里美"

echo "==> 今日の午後・明日以降の予約（患者本人が API で予約）"
book() { # book 患者番号 DEPT DOCTOR 日付 HH:MM first|revisit 症状
  local tok sid
  tok=$(login "demo$1@dev.test")
  sid=$(slot_at "$2" "$3" "$4" "$5")
  [ -n "$sid" ] || { echo "  枠なし: $2 $4 $5" >&2; return 0; }
  call "$tok" POST /appointments "{\"slot_id\":\"$sid\",\"visit_type\":\"$6\",\"symptom_note\":\"$7\"}" | jq -r .appointment_no
}
tomorrow=$(TZ=Asia/Tokyo date -d "$today +1 day" +%F); d2=$(TZ=Asia/Tokyo date -d "$today +2 day" +%F); d3=$(TZ=Asia/Tokyo date -d "$today +3 day" +%F)
# 今日の午後（来院受付の対象）。実行時刻によっては枠が過去になり API で予約できないため、DB に直接入れる。
book_today() { # book_today 患者番号 DEPT DOCTOR HH:MM 初診/再診 症状
  local sid; sid=$(slot_at "$2" "$3" "$today" "$4")
  psql -c "INSERT INTO appointments (appointment_no, patient_id, slot_id, visit_type, symptom_note)
    VALUES ('A-${today//-/}-$(printf '%04d' $((RANDOM % 9000 + 1000)))', '$(patient_id "$1")', '$sid', '$5', '$6')" >/dev/null
  psql -c "UPDATE slots SET booked_count = booked_count + 1 WHERE id='$sid'" >/dev/null
}
book_today 8  "$NAIKA" "$D_NAIKA" 14:30 first   "胃のもたれ"
book_today 9  "$NAIKA" "$D_NAIKA" 15:00 revisit "血圧の相談"
book_today 10 "$NAIKA" "$D_NAIKA" 16:00 first   "めまい"
book_today 1  "$SHONI" "$D_SHONI" 14:30 revisit "咳・夜間にひどい"
book_today 2  "$SHONI" "$D_SHONI" 15:30 first   "発疹"
# 明日以降（マイページ・予約一覧用）
book 3  "$NAIKA"  "$D_NAIKA"  "$tomorrow" 09:30 revisit "高脂血症の薬の継続" >/dev/null
book 4  "$NAIKA"  "$D_NAIKA"  "$tomorrow" 15:00 first   "頭痛が続く"         >/dev/null
book 5  "$SHONI"  "$D_SHONI"  "$d2" 10:00 revisit "ぜんそくの定期受診"     >/dev/null
book 6  "$NAIKA"  "$D_NAIKA"  "$d2" 11:00 revisit "インフルエンザ予防接種" >/dev/null
book 1  "$NAIKA"  "$D_NAIKA"  "$d3" 09:00 revisit "定期受診"               >/dev/null
for w in 2 3 4 5 7 9; do book "$w" "$NAIKA" "$D_NAIKA" "$tomorrow" 10:00 revisit "定期受診" >/dev/null 2>&1 || true; done

echo "==> 今日の午後の待機列（来院受付・当日受付・呼出・診察中）"
appt_id() { call "$STAFF" GET "/staff/appointments?department_id=$1&date=$today" | jq -r ".[] | select(.patient_name==\"$2\") | .id" | head -1; }
checkin() { call "$STAFF" POST "/staff/queue/$(appt_id "$1" "$2")/check-in" >/dev/null; }
checkin "$NAIKA" "吉田 恵"     # 14:30
checkin "$NAIKA" "山本 浩"     # 15:00
checkin "$SHONI" "田中 一郎"   # 14:30
call "$STAFF" POST /staff/walk-ins "{\"department_id\":$NAIKA,\"display_name\":\"松本 勇\"}" >/dev/null
call "$STAFF" POST /staff/walk-ins "{\"department_id\":$NAIKA,\"patient_id\":\"$(patient_id 5)\"}" >/dev/null
call "$STAFF" POST /staff/walk-ins "{\"department_id\":$SHONI,\"display_name\":\"井上 蓮（4歳）\"}" >/dev/null
call "$STAFF" POST /staff/walk-ins "{\"department_id\":$SEIKEI,\"display_name\":\"清水 久美子\"}" >/dev/null
call "$STAFF" POST /staff/walk-ins "{\"department_id\":$SEIKEI,\"display_name\":\"石井 剛\"}" >/dev/null
# 受付時刻・診察開始時刻を、実際の来院らしい時刻に直す（API は実行時刻で記録するため）。ticket_no は上の流れで決まる。
tweak() { # tweak DEPT TICKET 受付HH:MM [診察開始HH:MM]
  psql -c "UPDATE queue_entries q SET checked_in_at='$today $3+09',
      sort_time=GREATEST(COALESCE((SELECT s.start_at FROM appointments a JOIN slots s ON s.id=a.slot_id WHERE a.id=q.appointment_id), '$today $3+09'), '$today $3+09')${4:+, started_at='$today $4+09'}
    WHERE department_id=$1 AND service_date='$today' AND ticket_no=$2" >/dev/null
}
# 内科の先頭を診察中、次を呼出済みにする。小児科の先頭は呼出済み、整形外科の先頭は診察中。
first_ids() { call "$STAFF" GET "/staff/queue?department_id=$1" | jq -r '.[].id'; }
mapfile -t Q < <(first_ids "$NAIKA"); call "$STAFF" POST "/staff/queue/${Q[0]}/call" >/dev/null; call "$STAFF" POST "/staff/queue/${Q[0]}/start" >/dev/null
mapfile -t Q < <(first_ids "$SHONI"); call "$STAFF" POST "/staff/queue/${Q[0]}/call" >/dev/null
mapfile -t Q < <(first_ids "$SEIKEI"); call "$STAFF" POST "/staff/queue/${Q[0]}/call" >/dev/null; call "$STAFF" POST "/staff/queue/${Q[0]}/start" >/dev/null

# 内科: 8 吉田(診察中) 9 山本 10 松本(当日) 11 渡辺(当日)。小児科: 3 田中(呼出済み) 4 井上(当日)。整形外科: 2 清水(診察中) 3 石井(当日)
tweak "$NAIKA" 8 14:22 14:41;  tweak "$NAIKA" 9 14:50;  tweak "$NAIKA" 10 15:02;  tweak "$NAIKA" 11 15:04
tweak "$SHONI" 3 14:25;        tweak "$SHONI" 4 14:57
tweak "$SEIKEI" 2 14:35 14:44; tweak "$SEIKEI" 3 15:01

echo "==> 完了"
call "$STAFF" GET "/staff/queue?department_id=$NAIKA" | jq -r '.[] | "  内科 \(.ticket_no) \(.status) \(.patient_name)"'
