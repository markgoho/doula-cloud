#!/usr/bin/env bash
# Empties every store that holds Practice data (#2408): the application
# schema in Postgres, dropped and rebuilt by goose from zero; every
# Identity Platform user; every object in the attachments bucket; and,
# when named, every task in the notification nudge queue. The runbook,
# the dry run and the rollback are on #2408.
#
# Nothing has a default, so the script cannot reach production unless
# production is named in every variable:
#   WIPE_INSTANCE            instance connection name, project:region:instance
#   WIPE_DSN                 postgres://user:pass@_/dbname, the shape of
#                            doula-cloud-pg-migrate-dsn; the host is replaced
#                            by the Auth Proxy's 127.0.0.1:WIPE_PROXY_PORT
#   WIPE_BUCKET              attachments bucket name, without gs://
#   WIPE_AUTH_PROJECT        Identity Platform project id
# Optional:
#   WIPE_AUTH_EMULATOR_HOST  host:port of the Auth emulator in place of
#                            Identity Platform
#   WIPE_TASKS_QUEUE         projects/P/locations/L/queues/Q to purge
#   WIPE_PROXY_PORT          defaults to 15432
#
# It names every target, then reads one line from stdin, which must be the
# instance connection name, before it touches anything.
set -euo pipefail

PRODUCTION_INSTANCE="doula-cloud:us-central1:doula-cloud-pg"
PRODUCTION_BUCKET="doula-cloud-attachments"
PRODUCTION_PROJECT="doula-cloud"

PROXY_BIN="${CLOUD_SQL_PROXY_BIN:-cloud-sql-proxy}"
GCLOUD_BIN="${GCLOUD_BIN:-gcloud}"
PSQL_BIN="${PSQL_BIN:-psql}"
CURL_BIN="${CURL_BIN:-curl}"
GOOSE_BIN="${GOOSE_BIN:-go tool goose}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIR="$SCRIPT_DIR/../api"

die() {
	echo "wipe: $*" >&2
	exit 1
}

: "${WIPE_INSTANCE:?must be set: the instance connection name, project:region:instance}"
: "${WIPE_DSN:?must be set: postgres://user:pass@_/dbname}"
: "${WIPE_BUCKET:?must be set: the attachments bucket, without gs://}"
: "${WIPE_AUTH_PROJECT:?must be set: the Identity Platform project id}"
AUTH_EMULATOR="${WIPE_AUTH_EMULATOR_HOST:-}"
TASKS_QUEUE="${WIPE_TASKS_QUEUE:-}"
PROXY_PORT="${WIPE_PROXY_PORT:-15432}"

[[ "$WIPE_INSTANCE" =~ ^[a-z0-9-]+:[a-z0-9-]+:[a-z0-9-]+$ ]] ||
	die "WIPE_INSTANCE must be project:region:instance, got '$WIPE_INSTANCE'"
[[ "$WIPE_DSN" =~ ^postgres(ql)?://[^@/]+@[^/]*/[^/?]+ ]] ||
	die "WIPE_DSN must be postgres://user:pass@host/dbname"
[[ "$WIPE_BUCKET" != gs://* && "$WIPE_BUCKET" != */* ]] ||
	die "WIPE_BUCKET is a bucket name, not a URL or path"
if [[ -n "$TASKS_QUEUE" && ! "$TASKS_QUEUE" =~ ^projects/[^/]+/locations/[^/]+/queues/[^/]+$ ]]; then
	die "WIPE_TASKS_QUEUE must be projects/P/locations/L/queues/Q"
fi

production=()
scratch=()
classify() {
	if [[ "$2" == "$3" ]]; then production+=("$1"); else scratch+=("$1"); fi
}
classify instance "$WIPE_INSTANCE" "$PRODUCTION_INSTANCE"
classify bucket "$WIPE_BUCKET" "$PRODUCTION_BUCKET"
if [[ -n "$AUTH_EMULATOR" ]]; then
	scratch+=(auth)
else
	classify auth "$WIPE_AUTH_PROJECT" "$PRODUCTION_PROJECT"
fi
if [[ -n "$TASKS_QUEUE" ]]; then
	classify queue "${TASKS_QUEUE%%/locations/*}" "projects/$PRODUCTION_PROJECT"
fi
if ((${#production[@]} > 0 && ${#scratch[@]} > 0)); then
	die "refusing a mix of production (${production[*]}) and scratch (${scratch[*]}) targets"
fi

without_scheme="${WIPE_DSN#*://}"
userpass="${without_scheme%%@*}"
after_host="${without_scheme#*@}"
path_and_query="/${after_host#*/}"
DSN="postgres://${userpass}@127.0.0.1:${PROXY_PORT}${path_and_query}"
if [[ "$DSN" != *\?* ]]; then DSN="${DSN}?sslmode=disable"; fi
DB_NAME="${path_and_query#/}"
DB_NAME="${DB_NAME%%\?*}"

if [[ -n "$AUTH_EMULATOR" ]]; then
	AUTH_BASE="http://${AUTH_EMULATOR}/identitytoolkit.googleapis.com/v1/projects/${WIPE_AUTH_PROJECT}"
else
	AUTH_BASE="https://identitytoolkit.googleapis.com/v1/projects/${WIPE_AUTH_PROJECT}"
fi

echo "wipe: this deletes every Practice's data from:"
echo "  Postgres   ${WIPE_INSTANCE}, database ${DB_NAME}, schema public (dropped, rebuilt by goose)"
echo "  Auth       ${AUTH_BASE}"
echo "  Bucket     gs://${WIPE_BUCKET}"
echo "  Tasks      ${TASKS_QUEUE:-(not named, left alone)}"
if ((${#production[@]} > 0)); then
	echo "wipe: THESE ARE THE PRODUCTION TARGETS."
fi
printf 'wipe: type the instance connection name to go on: '
confirmation=""
read -r confirmation || true
echo
[[ "$confirmation" == "$WIPE_INSTANCE" ]] || die "the confirmation did not match ${WIPE_INSTANCE}; nothing was touched"

$PROXY_BIN --port "$PROXY_PORT" "$WIPE_INSTANCE" >&2 &
PROXY_PID=$!
trap 'kill "$PROXY_PID" 2>/dev/null || true; wait "$PROXY_PID" 2>/dev/null || true' EXIT

proxy_ready=false
for _ in $(seq 1 30); do
	if (exec 3<>"/dev/tcp/127.0.0.1/$PROXY_PORT") 2>/dev/null; then
		proxy_ready=true
		break
	fi
	kill -0 "$PROXY_PID" 2>/dev/null || die "the Cloud SQL Auth Proxy exited before it listened"
	sleep 1
done
[[ "$proxy_ready" == true ]] || die "the Cloud SQL Auth Proxy did not listen on 127.0.0.1:$PROXY_PORT within 30s"

sql() {
	$PSQL_BIN "$DSN" -v ON_ERROR_STOP=1 -X -q -At "$@"
}

table_counts() {
	sql -F $'\t' -c "SELECT table_name, (xpath('/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM public.%I', table_name), false, true, '')))[1]::text FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE' ORDER BY table_name"
}

sum_counts() {
	awk -F '\t' '{ tables += 1; rows += $2 } END { printf "%d tables, %d rows\n", tables, rows }'
}

schema_shape() {
	sql -c "SELECT nspowner::regrole::text || ' ' || coalesce(nspacl::text, '(default)') FROM pg_namespace WHERE nspname = 'public'"
}

auth_header() {
	if [[ -n "$AUTH_EMULATOR" ]]; then
		echo "Authorization: Bearer owner"
	else
		echo "Authorization: Bearer $($GCLOUD_BIN auth print-access-token)"
	fi
}

auth_post() {
	local response
	response="$($CURL_BIN -sS -X POST "${AUTH_BASE}/$1" \
		-H "$(auth_header)" \
		-H "x-goog-user-project: ${WIPE_AUTH_PROJECT}" \
		-H "Content-Type: application/json" \
		--data "$2")"
	if jq -e '.error' >/dev/null 2>&1 <<<"$response"; then
		die "Identity Platform refused $1: $(jq -c '.error' <<<"$response")"
	fi
	printf '%s' "$response"
}

auth_uids() {
	local page="" response
	while :; do
		response="$($CURL_BIN -sS -G "${AUTH_BASE}/accounts:batchGet" \
			-H "$(auth_header)" \
			-H "x-goog-user-project: ${WIPE_AUTH_PROJECT}" \
			--data-urlencode "maxResults=1000" \
			${page:+--data-urlencode "nextPageToken=$page"})"
		if jq -e '.error' >/dev/null 2>&1 <<<"$response"; then
			die "Identity Platform refused accounts:batchGet: $(jq -c '.error' <<<"$response")"
		fi
		jq -r '.users // [] | .[].localId' <<<"$response"
		page="$(jq -r '.nextPageToken // empty' <<<"$response")"
		[[ -n "$page" ]] || break
	done
}

bucket_objects() {
	local out status=0
	out="$($GCLOUD_BIN storage ls --all-versions "gs://${WIPE_BUCKET}/**" 2>&1)" || status=$?
	if ((status != 0)); then
		[[ "$out" == *"matched no objects"* ]] || die "could not list gs://${WIPE_BUCKET}: $out"
		return 0
	fi
	grep -v '/$' <<<"$out" || true
}

queue_parts() {
	local rest="${TASKS_QUEUE#projects/}"
	QUEUE_PROJECT="${rest%%/*}"
	rest="${rest#*/locations/}"
	QUEUE_LOCATION="${rest%%/*}"
	QUEUE_NAME="${rest##*/queues/}"
}

queue_tasks() {
	$GCLOUD_BIN tasks list --queue="$QUEUE_NAME" --location="$QUEUE_LOCATION" --project="$QUEUE_PROJECT" --format='value(name)'
}

count_lines() {
	grep -c . || true
}

echo "wipe: connected as $(sql -c 'SELECT current_user') to $(sql -c 'SELECT current_database()')"
others="$(sql -c "SELECT string_agg(DISTINCT table_schema, ', ') FROM information_schema.tables WHERE table_schema NOT IN ('public', 'pg_catalog', 'information_schema')")"
[[ -z "$others" ]] || die "tables outside the public schema ($others); this script wipes only public, so nothing was touched"
[[ -n "$TASKS_QUEUE" ]] && queue_parts
$GCLOUD_BIN storage buckets describe "gs://${WIPE_BUCKET}" --format='value(name)' >/dev/null ||
	die "gs://${WIPE_BUCKET} is not a bucket this account can read; nothing was touched"

echo "wipe: BEFORE"
before_tables="$(table_counts)"
printf '%s\n' "$before_tables" | sed 's/^/  pg  /'
before_shape="$(schema_shape)"
before_pg="$(printf '%s\n' "$before_tables" | grep . | sum_counts)"
before_auth="$(auth_uids | count_lines)"
before_bucket="$(bucket_objects | count_lines)"
before_queue="-"
[[ -n "$TASKS_QUEUE" ]] && before_queue="$(queue_tasks | count_lines)"
echo "  schema public: ${before_shape}"

echo "wipe: dropping and recreating schema public"
sql <<'SQL'
BEGIN;
SET LOCAL lock_timeout = '15s';
SET LOCAL client_min_messages = warning;
DO $wipe$
DECLARE
	owner_name text;
	grants text[];
	statement text;
BEGIN
	SELECT nspowner::regrole::text INTO owner_name FROM pg_namespace WHERE nspname = 'public';
	SELECT coalesce(array_agg(format('GRANT %s ON SCHEMA public TO %s', a.privilege_type,
			CASE WHEN a.grantee = 0 THEN 'PUBLIC' ELSE a.grantee::regrole::text END)), '{}')
		INTO grants
		FROM pg_namespace n, aclexplode(n.nspacl) a
		WHERE n.nspname = 'public' AND a.grantee <> n.nspowner;
	DROP SCHEMA public CASCADE;
	CREATE SCHEMA public;
	EXECUTE format('ALTER SCHEMA public OWNER TO %s', owner_name);
	FOREACH statement IN ARRAY grants LOOP
		EXECUTE statement;
	END LOOP;
END
$wipe$;
COMMIT;
SQL

after_shape="$(schema_shape)"
[[ "$after_shape" == "$before_shape" ]] ||
	die "schema public came back as '${after_shape}', not '${before_shape}'; goose has not run"

echo "wipe: applying every migration from zero"
(cd "$API_DIR" && $GOOSE_BIN -dir db/migrations postgres "$DSN" up)
goose_status="$(cd "$API_DIR" && $GOOSE_BIN -dir db/migrations postgres "$DSN" status 2>&1)"
total="$(grep -cE -- '-- [0-9]+_' <<<"$goose_status" || true)"
pending="$(grep -cE -- 'Pending +-- [0-9]+_' <<<"$goose_status" || true)"
applied=$((total - pending))
((total > 0)) || die "goose status listed no migrations: $goose_status"
((pending == 0)) || die "goose reports ${pending} pending migrations after up"

echo "wipe: deleting Identity Platform users"
mapfile -t uids < <(auth_uids)
for ((i = 0; i < ${#uids[@]}; i += 1000)); do
	batch="$(printf '%s\n' "${uids[@]:i:1000}" | jq -R . | jq -sc '{localIds: ., force: true}')"
	response="$(auth_post accounts:batchDelete "$batch")"
	if jq -e '(.errors // []) | length > 0' >/dev/null <<<"$response"; then
		die "Identity Platform did not delete every user: $(jq -c '.errors' <<<"$response")"
	fi
done

echo "wipe: deleting every object in gs://${WIPE_BUCKET}"
if [[ "$before_bucket" != 0 ]]; then
	$GCLOUD_BIN storage rm --all-versions "gs://${WIPE_BUCKET}/**"
fi

if [[ -n "$TASKS_QUEUE" ]]; then
	echo "wipe: purging ${TASKS_QUEUE}"
	$GCLOUD_BIN tasks queues purge "$QUEUE_NAME" --location="$QUEUE_LOCATION" --project="$QUEUE_PROJECT" --quiet
fi

echo "wipe: AFTER"
after_tables="$(table_counts)"
printf '%s\n' "$after_tables" | sed 's/^/  pg  /'
after_pg="$(printf '%s\n' "$after_tables" | grep . | sum_counts)"
after_auth="$(auth_uids | count_lines)"
after_bucket="$(bucket_objects | count_lines)"
after_queue="-"
[[ -n "$TASKS_QUEUE" ]] && after_queue="$(queue_tasks | count_lines)"
echo "  schema public: ${after_shape}"

echo "wipe: SUMMARY"
printf '  %-10s %-28s %s\n' store before after
printf '  %-10s %-28s %s\n' postgres "$before_pg" "$after_pg"
printf '  %-10s %-28s %s\n' auth "$before_auth users" "$after_auth users"
printf '  %-10s %-28s %s\n' bucket "$before_bucket objects" "$after_bucket objects"
printf '  %-10s %-28s %s\n' tasks "$before_queue" "$after_queue"
printf '  %-10s %s migrations applied, %s pending\n' goose "$applied" "$pending"
