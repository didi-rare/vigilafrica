#!/usr/bin/env bash
# Rehearses an Umami upgrade on a throwaway database before it reaches a real one.
#
# Boots OLD_IMAGE against a fresh Postgres, records some traffic, then boots the
# image built from analytics/Dockerfile on the SAME database and checks that
# exactly migrations 20-26 apply, existing data survives, login and tracking
# still work, and the GHSA-3xx8-742v-m4mq share-token replay is refused. The
# replay also runs against OLD_IMAGE as a control: on a vulnerable release it
# must return protected session data, or a 401 on the new image proves nothing.
#
# It also seeds the two data shapes the migrations REWRITE, so their effect is
# asserted rather than assumed: duplicate session_data keys (migration 23
# deletes all but the newest per session and key) and a mixed-case username
# (migration 24 lowercases it). A fresh database is still not production: run
# the preflight counts in docs/operations/analytics.md against the real one.
#
# Usage: scripts/rehearse-umami-upgrade.sh [OLD_IMAGE]
#   OLD_IMAGE defaults to the Umami 3.1.0 image that was pinned before 3.4.0.
# Needs Docker and python3; uses host ports 3901/3902 and cleans up after itself.
# Not -e: a failed check must record FAIL and carry on, so every check reports.
set -uo pipefail
cd "$(dirname "$0")/.."

OLD=${1:-ghcr.io/umami-software/umami@sha256:e3f80c0625aad7179b49da27d475357cabb1068b8cacd9a792cfd6966888b123}
NEW=vigil-umami:rehearsal
PG=postgis/postgis:15-3.4@sha256:3f4a5d48e0be9580ed70ed618cd039ce57bbb2dd113053d3836e28513f1f87cd
NET=umami-rehearsal
SECRET=rehearsal_app_secret_0123456789abcdef
UA='Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36'
BODY=$(mktemp)
FAIL=0
EXPECTED_NEW_MIGRATIONS="20_add_heatmap,21_add_session_link,22_add_2fa,23_update_session_data,24_lowercase_username,25_add_annotation,26_add_api_key"

cleanup() { docker rm -f reh-pg reh-old reh-new >/dev/null 2>&1; docker network rm "$NET" >/dev/null 2>&1; rm -f "$BODY"; }
trap cleanup EXIT
check() { if [ "$2" = "$3" ]; then echo "PASS  $1 ($2)"; else echo "FAIL  $1: got '$2', want '$3'"; FAIL=1; fi; }
json() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)" 2>/dev/null; }
psql() { docker exec reh-pg psql -U postgres -d umami -tAc "$1"; }
wait_up() {
  for _ in $(seq 1 90); do curl -sf "http://127.0.0.1:$1/api/heartbeat" >/dev/null && return 0; sleep 2; done
  echo "FAIL  $2 never became healthy"; docker logs --tail 40 "$2"; exit 1
}
run_umami() {
  docker run -d --name "$1" --network "$NET" -p "127.0.0.1:$3:3000" \
    -e DATABASE_URL=postgresql://umami:umami_pw@reh-pg:5432/umami -e DATABASE_TYPE=postgresql \
    -e APP_SECRET="$SECRET" --cap-drop ALL --security-opt no-new-privileges:true "$2" >/dev/null
}
login() {
  curl -s -X POST "http://127.0.0.1:$1/api/auth/login" -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"umami"}' | json ".get('token','')"
}
send() {
  curl -s -X POST "http://127.0.0.1:$1/api/send" -H 'Content-Type: application/json' -H "User-Agent: $UA" \
    -d "{\"type\":\"event\",\"payload\":{\"website\":\"$2\",\"hostname\":\"vigilafrica.org\",\"url\":\"$3\",\"language\":\"en\",\"screen\":\"1920x1080\",\"title\":\"t\"}}"
}
replay() { # HTTP status of reading sessions with a tracking cache token as the share token
  local cache; cache=$(send "$1" "$2" /replay | json ".get('cache','')")
  [ -z "$cache" ] && { echo "no-cache-token"; return; }
  curl -s -o "$BODY" -w "%{http_code}" \
    "http://127.0.0.1:$1/api/websites/$2/sessions?startAt=0&endAt=9999999999999&pageSize=100" \
    -H "x-umami-share-token: $cache" -H 'x-umami-share-context: 1'
}
events() { psql 'select count(*) from website_event'; }

cleanup
echo "== building $NEW from analytics/Dockerfile"
docker build -q -t "$NEW" analytics >/dev/null || { echo "FAIL  build"; exit 1; }
docker network create "$NET" >/dev/null

echo "== Postgres"
docker run -d --name reh-pg --network "$NET" -e POSTGRES_PASSWORD=pg "$PG" >/dev/null
for _ in $(seq 1 30); do docker exec reh-pg pg_isready -U postgres >/dev/null 2>&1 && break; sleep 2; done
sleep 3
docker exec reh-pg psql -U postgres -c "CREATE ROLE umami WITH LOGIN PASSWORD 'umami_pw';" \
  -c "CREATE DATABASE umami OWNER umami;" >/dev/null || { echo "FAIL  Postgres never became ready"; exit 1; }

echo "== OLD: $OLD"
run_umami reh-old "$OLD" 3901; wait_up 3901 reh-old
echo "      node $(docker exec reh-old node -v)"
TOKEN=$(login 3901)
WID=$(curl -s -X POST http://127.0.0.1:3901/api/websites -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"rehearsal","domain":"vigilafrica.org"}' | json "['id']")
[ -n "$WID" ] || { echo "FAIL  could not create a website on OLD"; exit 1; }
for p in / /for-partners /a /b /c; do send 3901 "$WID" "$p" >/dev/null; done
check "share-token replay on OLD (control: vulnerable release)" "$(replay 3901 "$WID")" 200
check "control replay returned this website's protected sessions" \
  "$(json "['data'][0]['websiteId']" < "$BODY")" "$WID"
sleep 2
BEFORE=$(events); OLD_MIGRATIONS=$(psql 'select count(*) from _prisma_migrations where finished_at is not null')
echo "      events=$BEFORE migrations=$OLD_MIGRATIONS"

# Seed what migrations 23 and 24 rewrite: two values for one session key (the
# newer must survive), one unique key, and a mixed-case username.
SID=$(psql "select session_id from session where website_id = '$WID' limit 1")
psql "insert into session_data (session_data_id, website_id, session_id, data_key, string_value, data_type, created_at) values
  (gen_random_uuid(), '$WID', '$SID', 'plan', 'free', 1, now() - interval '1 hour'),
  (gen_random_uuid(), '$WID', '$SID', 'plan', 'pro',  1, now()),
  (gen_random_uuid(), '$WID', '$SID', 'org',  'ngo',  1, now())" >/dev/null
psql "insert into \"user\" (user_id, username, password, role) values (gen_random_uuid(), 'Analyst', 'x', 'user')" >/dev/null
check "seeded session_data rows on OLD" "$(psql 'select count(*) from session_data')" 3
docker stop reh-old >/dev/null

echo "== NEW: analytics/Dockerfile, same database"
run_umami reh-new "$NEW" 3902; wait_up 3902 reh-new
check "node version on NEW" "$(docker exec reh-new node -v)" "v$(sed -n 's/^ENV NODE_VERSION=//p' analytics/Dockerfile)"
docker logs reh-new 2>&1 | grep -E "Applying migration" | sed 's/^/      /'
check "finished migrations: exactly seven more" \
  "$(psql 'select count(*) from _prisma_migrations where finished_at is not null')" "$((OLD_MIGRATIONS + 7))"
check "the seven are 20-26" \
  "$(psql "select string_agg(migration_name, ',' order by migration_name) from _prisma_migrations where finished_at is not null and migration_name >= '20_'")" \
  "$EXPECTED_NEW_MIGRATIONS"
check "unfinished or rolled-back migrations" \
  "$(psql 'select count(*) from _prisma_migrations where finished_at is null or rolled_back_at is not null')" 0
check "events preserved across upgrade" "$(events)" "$BEFORE"
check "migration 23 kept the newest value per session key" \
  "$(psql "select string_agg(data_key || '=' || string_value, ',' order by data_key) from session_data")" "org=ngo,plan=pro"
check "migration 24 lowercased the username" \
  "$(psql "select count(*) from \"user\" where username = 'analyst'")" 1
TOKEN=$(login 3902)
check "admin login on NEW" "${TOKEN:+ok}" ok
check "pageviews visible via authenticated API" \
  "$(curl -s "http://127.0.0.1:3902/api/websites/$WID/stats?startAt=0&endAt=9999999999999" -H "Authorization: Bearer $TOKEN" | json "['pageviews']")" "$BEFORE"
check "share-token replay on NEW" "$(replay 3902 "$WID")" 401
send 3902 "$WID" /after-upgrade >/dev/null; sleep 2
check "tracking records on NEW (both sends counted)" "$(events)" "$((BEFORE + 2))"
check "tracker script served" "$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:3902/script.js)" 200
ERRS=$(docker logs reh-new 2>&1 | grep -ciE "error|fail")
check "error/fail lines in NEW logs" "$ERRS" 0

[ "$FAIL" = 0 ] && echo "== ALL CHECKS PASSED" || { echo "== SOME CHECKS FAILED"; exit 1; }
