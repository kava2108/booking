#!/usr/bin/env bash
# マイグレーションテスト共通関数（source して使う）
set -uo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../migrations" && pwd)"
URL="${TEST_DATABASE_URL:?TEST_DATABASE_URL を指定してください}"
case "${URL%%\?*}" in *_test) ;; *) echo "DB 名が *_test ではありません: 中断" >&2; exit 2;; esac

PSQL=(psql "$URL" -v ON_ERROR_STOP=1 -X -q -t -A)
pass=0; fail=0
ok()   { pass=$((pass+1)); echo "  ok   $1"; }
ng()   { fail=$((fail+1)); echo "  FAIL $1"; }
sql()  { "${PSQL[@]}" -c "$1" 2>&1; }
expect_ok()   { if out=$(sql "$2"); then ok "$1"; else ng "$1 :: $out"; fi; }
expect_fail() { # $1 名前 $2 SQL $3 期待するエラー文字列
  if out=$(sql "$2"); then ng "$1 (成功してしまった)"; elif grep -q "$3" <<<"$out"; then ok "$1"; else ng "$1 :: $out"; fi; }
count()  { sql "SELECT count(*) FROM $1" | tr -d ' '; }

reset()  { "${PSQL[@]}" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;" >/dev/null; }
up()     { for f in "$DIR"/*.up.sql;   do "${PSQL[@]}" -f "$f" >/dev/null || { echo "up 失敗: $f"; return 1; }; done; }
down()   { for f in $(ls "$DIR"/*.down.sql | sort -r); do "${PSQL[@]}" -f "$f" >/dev/null || { echo "down 失敗: $f"; return 1; }; done; }

