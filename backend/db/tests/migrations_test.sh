#!/usr/bin/env bash
# マイグレーション検証（IMP 1-user-auth-db-migration TASK-0001）
# 使い方: TEST_DATABASE_URL=postgres://user@host:port/booking_test ./migrations_test.sh
# 注意: DB 名が *_test で終わらない場合は実行を拒否する（public スキーマを初期化するため）
source "$(dirname "$0")/lib.sh"
tables() { sql "SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('users','refresh_tokens','password_reset_tokens')"; }

echo "# up/down ペア"
for u in "$DIR"/*.up.sql; do [ -f "${u%.up.sql}.down.sql" ] && ok "down あり: $(basename "$u")" || ng "down なし: $(basename "$u")"; done
[ "$(ls "$DIR"/*.up.sql 2>/dev/null | wc -l)" -ge 3 ] && ok "up が 3 本以上ある" || ng "up が 3 本未満"

reset
echo "# up"
up && ok "up 成功" || ng "up 失敗"
[ "$(tables)" = "3" ] && ok "3 テーブル作成" || ng "テーブル数: $(tables)"

echo "# users 制約 (REQ-001-AC-1)"
expect_ok   "role 省略 → patient" "INSERT INTO users(email,password_hash,name) VALUES ('a@example.com','h','山田')"
[ "$(sql "SELECT role FROM users WHERE email='a@example.com'")" = "patient" ] && ok "role 既定値 patient" || ng "role 既定値"
for r in staff clinic_admin system_admin; do
  expect_ok "role=$r を許可" "INSERT INTO users(email,password_hash,name,role) VALUES ('$r@example.com','h','n','$r')"
done
expect_fail "role='user' は CHECK 違反" "INSERT INTO users(email,password_hash,name,role) VALUES ('b@example.com','h','n','user')" "23514\|check constraint"
expect_fail "email 重複は UNIQUE 違反" "INSERT INTO users(email,password_hash,name) VALUES ('a@example.com','h','n')" "duplicate key"
expect_fail "password_hash は 72 文字まで" "INSERT INTO users(email,password_hash,name) VALUES ('c@example.com','$(printf 'x%.0s' $(seq 73))','n')" "too long"
expect_fail "email NOT NULL" "INSERT INTO users(password_hash,name) VALUES ('h','n')" "null value"
UID_=$(sql "SELECT id FROM users WHERE email='a@example.com'")

echo "# refresh_tokens (REQ-003-AC-3)"
expect_ok   "refresh_tokens 登録" "INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES ('$UID_','t1', now()+interval '7 days')"
expect_fail "token_hash 重複は UNIQUE 違反" "INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES ('$UID_','t1', now())" "duplicate key"
expect_fail "存在しない user_id は FK 違反" "INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES (gen_random_uuid(),'t2', now())" "foreign key"
[ "$(sql "SELECT revoked_at IS NULL FROM refresh_tokens WHERE token_hash='t1'")" = "t" ] && ok "revoked_at 既定 NULL" || ng "revoked_at 既定"

echo "# password_reset_tokens (REQ-004-AC-1)"
expect_ok   "password_reset_tokens 登録" "INSERT INTO password_reset_tokens(user_id,token_hash,expires_at) VALUES ('$UID_','p1', now()+interval '1 hour')"
expect_fail "token_hash 重複は UNIQUE 違反" "INSERT INTO password_reset_tokens(user_id,token_hash,expires_at) VALUES ('$UID_','p1', now())" "duplicate key"
[ "$(sql "SELECT used_at IS NULL FROM password_reset_tokens WHERE token_hash='p1'")" = "t" ] && ok "used_at 既定 NULL" || ng "used_at 既定"

echo "# インデックス"
idxdef() { sql "SELECT indexdef FROM pg_indexes WHERE tablename='$1'"; }
grep -q "(user_id, revoked_at)" <<<"$(idxdef refresh_tokens)" && ok "refresh_tokens(user_id, revoked_at)" || ng "refresh_tokens(user_id, revoked_at) なし"
grep -q "(token_hash)" <<<"$(idxdef refresh_tokens)" && ok "refresh_tokens(token_hash)" || ng "refresh_tokens(token_hash) なし"
grep -q "(token_hash)" <<<"$(idxdef password_reset_tokens)" && ok "password_reset_tokens(token_hash)" || ng "password_reset_tokens(token_hash) なし"
grep -q "(email)" <<<"$(idxdef users)" && ok "users(email)" || ng "users(email) なし"

echo "# カスケード"
expect_ok "ユーザー削除でトークンも削除" "DELETE FROM users WHERE id='$UID_'"
[ "$(count refresh_tokens)" = "0" ] && [ "$(count password_reset_tokens)" = "0" ] && ok "トークンが残らない" || ng "トークンが残存"

echo "# down / 再 up (冪等性)"
down && ok "down 成功" || ng "down 失敗"
[ "$(tables)" = "0" ] && ok "全テーブル削除" || ng "残存テーブル: $(tables)"
up && ok "再 up 成功" || ng "再 up 失敗"
[ "$(tables)" = "3" ] && ok "再 up 後 3 テーブル" || ng "再 up 後テーブル数: $(tables)"

echo; echo "結果: pass=$pass fail=$fail"
[ "$fail" -eq 0 ]
