#!/usr/bin/env bash
# E2E テスト（Playwright）。Docker だけで、実際の PostgreSQL・バックエンド・フロントエンドを起動して実行する。
#   使い方: scripts/e2e.sh [playwright の引数...]   例: scripts/e2e.sh e2e/patient.spec.ts
# 必要なもの: docker（snap 版でも可）。ホストに Go / Node / psql は不要。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
W="$ROOT/.e2e"
PG=booking-e2e-pg
WEB=booking-e2e-web
PG_PORT=55433
API_PORT=18090
WEB_PORT=4173
PW_IMAGE="mcr.microsoft.com/playwright:v1.64.0-noble" # frontend の @playwright/test のバージョンと揃える
UIDGID="$(id -u):$(id -g)"
API_PID=""

mkdir -p "$W/gocache" "$W/gopath"
cleanup() {
  [ -n "$API_PID" ] && kill "$API_PID" 2>/dev/null || true
  docker rm -f "$PG" "$WEB" >/dev/null 2>&1 || true
}
trap cleanup EXIT
cleanup

psql() { docker exec -i "$PG" psql -U postgres -d booking_e2e -qtA -v ON_ERROR_STOP=1 "$@"; }

echo "==> PostgreSQL"
docker run -d --name "$PG" -e POSTGRES_HOST_AUTH_METHOD=trust -p "127.0.0.1:$PG_PORT:5432" postgres:16 >/dev/null
until docker exec "$PG" pg_isready -U postgres -q; do sleep 0.5; done
sleep 1
docker exec "$PG" psql -U postgres -qc "CREATE DATABASE booking_e2e"
for f in "$ROOT"/backend/db/migrations/*.up.sql; do psql < "$f" >/dev/null; done
psql -c "INSERT INTO departments(name, standard_consult_minutes) VALUES ('内科', 10)" \
     -c "INSERT INTO doctors(department_id, name) VALUES (1, '山田')" >/dev/null

echo "==> backend build"
docker run --rm --user "$UIDGID" -e HOME=/tmp -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath -e CGO_ENABLED=0 \
  -v "$ROOT/backend":/src -v "$W/gocache":/tmp/gocache -v "$W/gopath":/tmp/gopath -w /src golang:1.25 \
  go build -buildvcs=false -o /src/.e2e-api ./cmd/api
mv "$ROOT/backend/.e2e-api" "$W/api"

echo "==> backend start"
DB_DSN="postgres://postgres@127.0.0.1:$PG_PORT/booking_e2e" JWT_SECRET="e2e-secret-e2e-secret-e2e-secret-00" JWT_REFRESH_SECRET="unused-but-required" \
SERVER_PORT="$API_PORT" COOKIE_SECURE=false APP_BASE_URL="http://127.0.0.1:$WEB_PORT" \
RATE_LIMIT_SENSITIVE_MAX=10000 RATE_LIMIT_LOGIN_IP_MAX=10000 RATE_LIMIT_PUBLIC_MAX=10000 \
  "$W/api" > "$W/api.log" 2>&1 &
API_PID=$!
until curl -fs "http://127.0.0.1:$API_PORT/healthz" >/dev/null; do
  kill -0 "$API_PID" 2>/dev/null || { echo "backend が起動しませんでした:"; cat "$W/api.log"; exit 1; }
  sleep 0.3
done

echo "==> frontend build + preview"
docker run --rm --user "$UIDGID" -e HOME=/tmp -v "$ROOT":/repo -w /repo/frontend node:20 sh -c "npm ci --silent && npm run build --silent"
docker run -d --name "$WEB" --network host --user "$UIDGID" -e HOME=/tmp -e VITE_API_PROXY="http://127.0.0.1:$API_PORT" \
  -v "$ROOT":/repo -w /repo/frontend node:20 npx vite preview --host 127.0.0.1 --port "$WEB_PORT" --strictPort >/dev/null
until curl -fs "http://127.0.0.1:$WEB_PORT/" >/dev/null; do sleep 0.5; done

echo "==> seed（管理者・スタッフ・診療スケジュール）"
API="http://127.0.0.1:$WEB_PORT/api/v1" # フロントのプロキシ経由（本番と同じ同一オリジン構成）
register() { curl -fs -o /dev/null -X POST "$API/auth/register" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"E2e-Password-123!\",\"name\":\"$2\"}"; }
register admin@e2e.test "管理 太郎"
register staff@e2e.test "受付 花子"
psql -c "UPDATE users SET role='clinic_admin' WHERE email='admin@e2e.test'" -c "UPDATE users SET role='staff' WHERE email='staff@e2e.test'" >/dev/null
ADMIN_TOKEN="$(curl -fs -X POST "$API/auth/login" -H 'Content-Type: application/json' -d '{"email":"admin@e2e.test","password":"E2e-Password-123!"}' | sed -E 's/.*"access_token":"([^"]+)".*/\1/')"
SCHEDULES="$(for d in 0 1 2 3 4 5 6; do printf '{"weekday":%s,"start_time":"00:00","end_time":"23:30","slot_minutes":30,"capacity":3},' "$d"; done)"
curl -fs -o /dev/null -X PUT "$API/admin/schedules" -H 'Content-Type: application/json' -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d "{\"doctor_id\":1,\"schedules\":[${SCHEDULES%,}]}"

echo "==> playwright"
docker run --rm --network host --ipc=host --user "$UIDGID" -e HOME=/tmp -e E2E_BASE_URL="http://127.0.0.1:$WEB_PORT" \
  -v "$ROOT/frontend":/work -w /work "$PW_IMAGE" npx playwright test "$@"
