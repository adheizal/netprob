#!/bin/bash
set -euo pipefail

export PATH=/usr/local/go/bin:$PATH
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

NETPROB_PORT="${NETPROB_PORT:-18080}"
NETPROB_E2E_WAIT_SECONDS="${NETPROB_E2E_WAIT_SECONDS:-10}"
NETPROB_E2E_MTR_WAIT_SECONDS="${NETPROB_E2E_MTR_WAIT_SECONDS:-35}"
NETPROB_E2E_BINARY="${NETPROB_E2E_BINARY:-$SCRIPT_DIR/bin/netprob}"
BASE_URL="http://127.0.0.1:${NETPROB_PORT}"
PID_FILE="/tmp/netprob-e2e-server.pid"
JKT_PID_FILE="/tmp/netprob-e2e-jkt-agent.pid"
SG_PID_FILE="/tmp/netprob-e2e-sg-agent.pid"
COOKIE_JAR="/tmp/netprob-e2e-admin.cookies"

api_curl() {
  curl -b "$COOKIE_JAR" "$@"
}

stop_from_pid_file() {
  local file="$1"
  if [ -f "$file" ] && kill -0 "$(cat "$file")" 2>/dev/null; then
    local pid
    pid="$(cat "$file")"
    kill "$pid"
    for _ in {1..20}; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.1
    done
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid"
    fi
    wait "$(cat "$file")" 2>/dev/null || true
  fi
}

cleanup() {
  stop_from_pid_file "$JKT_PID_FILE"
  stop_from_pid_file "$SG_PID_FILE"
  stop_from_pid_file "$PID_FILE"
  rm -f -- "$COOKIE_JAR"
}
trap cleanup EXIT

# Stop only processes created by an earlier run of this test.
cleanup
sleep 1

# Clean and start server
rm -rf /tmp/netprob-data
mkdir -p /tmp/netprob-data
NETPROB_SERVER_LISTEN=":${NETPROB_PORT}" NETPROB_DATA_DIR=/tmp/netprob-data "$NETPROB_E2E_BINARY" -mode server > /tmp/netprob-server.log 2>&1 &
echo $! > "$PID_FILE"
sleep 2

echo "=== Admin authentication ==="
UNAUTHENTICATED_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/agents")
if [ "$UNAUTHENTICATED_STATUS" != "401" ]; then
  echo "management API returned $UNAUTHENTICATED_STATUS without authentication, expected 401" >&2
  exit 1
fi
curl -fsS -c "$COOKIE_JAR" -X POST "$BASE_URL/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@netprob.local","password":"changeme"}' | python3 -c '
import sys, json
session = json.load(sys.stdin)
assert session["authenticated"] is True, session
assert session["must_change_password"] is True, session
'
curl -fsS -b "$COOKIE_JAR" -c "$COOKIE_JAR" -X PUT "$BASE_URL/api/auth/account" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@netprob.local","current_password":"changeme","new_password":"e2e-secure-password"}' | python3 -c '
import sys, json
session = json.load(sys.stdin)
assert session["authenticated"] is True, session
assert session["must_change_password"] is False, session
'

# Register one reusable enrollment token for both agents
echo "=== Register shared enrollment token ==="
REGISTER_RESP=$(api_curl -s -X POST "$BASE_URL/api/agents/register" \
  -H 'Content-Type: application/json' \
  -d '{}')
echo "$REGISTER_RESP"
SHARED_TOKEN=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")

# Start two distinct instances with the same token
echo "=== Starting JKT agent with shared token ==="
NETPROB_AGENT_NAME=jkt-01 NETPROB_AGENT_REGION=test-jakarta NETPROB_GEOIP_URL= \
  NETPROB_CONTROLLER="$BASE_URL" NETPROB_AGENT_TOKEN=$SHARED_TOKEN \
  "$NETPROB_E2E_BINARY" -mode agent > /tmp/jkt-agent.log 2>&1 &
echo $! > "$JKT_PID_FILE"

echo "=== Starting SG agent with shared token ==="
NETPROB_AGENT_NAME=sg-01 NETPROB_AGENT_REGION=test-singapore NETPROB_GEOIP_URL= \
  NETPROB_CONTROLLER="$BASE_URL" NETPROB_AGENT_TOKEN=$SHARED_TOKEN \
  "$NETPROB_E2E_BINARY" -mode agent > /tmp/sg-agent.log 2>&1 &
echo $! > "$SG_PID_FILE"

echo "=== Waiting for both agents to register ==="
AGENT_DEADLINE=$((SECONDS + 30))
while true; do
  AGENTS_RESP=$(api_curl -fsS "$BASE_URL/api/agents")
  if AGENT_IDS=$(echo "$AGENTS_RESP" | python3 -c '
import sys, json
agents = json.load(sys.stdin)
ids = {agent["hostname"]: agent["id"] for agent in agents}
if "jkt-01" not in ids or "sg-01" not in ids:
    raise SystemExit(1)
print(ids["jkt-01"], ids["sg-01"])
'); then
    read -r JKT_ID SG_ID <<< "$AGENT_IDS"
    break
  fi
  if (( SECONDS >= AGENT_DEADLINE )); then
    echo "timed out waiting for both agents: $AGENTS_RESP" >&2
    echo "JKT agent log:" >&2
    cat /tmp/jkt-agent.log >&2
    echo "SG agent log:" >&2
    cat /tmp/sg-agent.log >&2
    exit 1
  fi
  sleep 1
done
if [ "$JKT_ID" = "$SG_ID" ]; then
  echo "shared enrollment token resolved both instances to the same agent" >&2
  exit 1
fi
echo "Distinct agents from one token: JKT=$JKT_ID SG=$SG_ID"

# Create link
echo "=== Creating link ==="
LINK_RESP=$(api_curl -s -X POST "$BASE_URL/api/links" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Jakarta ↔ Singapore","description":"JKT-SG link"}')
LINK_ID=$(echo "$LINK_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
echo "Link ID: $LINK_ID"

echo "=== Retention settings ==="
api_curl -fsS -X PUT "$BASE_URL/api/settings/retention" \
  -H 'Content-Type: application/json' \
  -d '{"ping_retention_days":0,"mtr_retention_days":0}' | python3 -c '
import sys, json
settings = json.load(sys.stdin)
assert settings["ping_retention_days"] == 0, settings
assert settings["mtr_retention_days"] == 0, settings
assert settings["cleanup"]["ping_results_deleted"] == 0, settings
assert settings["cleanup"]["mtr_runs_deleted"] == 0, settings
'

# Create both directions
echo "=== Direction JKT→SG ==="
JKT_DIR_RESP=$(api_curl -s -X POST "$BASE_URL/api/links/$LINK_ID/directions" \
  -H 'Content-Type: application/json' \
  -d "{\"source_agent_id\":\"$JKT_ID\",\"destination_agent_id\":\"$SG_ID\",\"target_address\":\"127.0.0.1\",\"ping_interval_seconds\":2}")
echo "$JKT_DIR_RESP"
JKT_DIR_ID=$(echo "$JKT_DIR_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

echo "=== Direction SG→JKT ==="
SG_DIR_RESP=$(api_curl -s -X POST "$BASE_URL/api/links/$LINK_ID/directions" \
  -H 'Content-Type: application/json' \
  -d "{\"source_agent_id\":\"$SG_ID\",\"destination_agent_id\":\"$JKT_ID\",\"target_address\":\"127.0.0.1\",\"ping_interval_seconds\":2,\"mtr_interval_seconds\":120}")
echo "$SG_DIR_RESP"
SG_DIR_ID=$(echo "$SG_DIR_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")

echo "=== Updating JKT→SG probe settings ==="
api_curl -fsS -X PUT "$BASE_URL/api/links/$LINK_ID/directions/$JKT_DIR_ID" \
  -H 'Content-Type: application/json' \
  -d '{"ping_interval_seconds":3,"mtr_interval_seconds":180,"ping_enabled":true,"mtr_enabled":false}'
echo

# Wait for agents to connect and start pinging
echo "=== Waiting for agents to connect and run jobs ==="
sleep "$NETPROB_E2E_WAIT_SECONDS"

# Check agent status
echo "=== Agent status ==="
api_curl -s "$BASE_URL/api/agents" | python3 -c "
import sys, json
agents = json.load(sys.stdin)
for a in agents:
    print(f'{a[\"hostname\"]}: online={a[\"online\"]}, last_seen={a[\"last_seen\"]}')
    assert a['online'], f'agent {a[\"id\"]} is offline'
    assert a['hostname'] in ('jkt-01', 'sg-01'), f'hostname was not synchronized: {a}'
    assert a['addresses'], f'addresses were not detected: {a}'
    assert '127.0.0.1' not in a['addresses'], f'loopback address was advertised: {a}'
    assert a['primary_address'], f'primary address was not selected: {a}'
    assert set(a['capabilities']) == {'ping', 'mtr'}, f'capabilities were not synchronized: {a}'
    expected_region = 'test-jakarta' if a['hostname'] == 'jkt-01' else 'test-singapore'
    assert a['location']['region'] == expected_region, f'region was not synchronized: {a}'
"

# Check ping results
echo "=== Links with data ==="
api_curl -s "$BASE_URL/api/links" | python3 -c "
import sys, json
links = json.load(sys.stdin)
for l in links:
    print(f'Link: {l[\"name\"]}')
    for d in l['directions']:
        src = d['source_agent']['hostname'] if d.get('source_agent') else d['source_agent_id']
        dest = d['dest_agent']['hostname'] if d.get('dest_agent') else d['destination_agent_id']
        ping = d.get('latest_ping', {})
        print(f'  {src} -> {dest}: online={d[\"online\"]}, avg_rtt={ping.get(\"avg_rtt_ms\",\"N/A\")}, loss={ping.get(\"packet_loss_percent\",\"N/A\")}%')
        if d['id'] == '$JKT_DIR_ID':
            assert d['ping_interval_seconds'] == 3, f'ping interval was not updated: {d}'
            assert d['mtr_interval_seconds'] == 180, f'MTR interval was not updated: {d}'
            assert d['ping_enabled'] is True, f'ping enabled state was not updated: {d}'
            assert d['mtr_enabled'] is False, f'MTR enabled state was not updated: {d}'
"

echo "=== Waiting for a successful MTR run ==="
MTR_DEADLINE=$((SECONDS + NETPROB_E2E_MTR_WAIT_SECONDS))
while true; do
  MTR_RESP=$(api_curl -fsS "$BASE_URL/api/links/$LINK_ID/directions/$SG_DIR_ID/mtr?limit=1")
  if echo "$MTR_RESP" | python3 -c '
import sys, json
runs = json.load(sys.stdin)
if not runs:
    raise SystemExit(1)
run = runs[0]
assert run["status"] == "success", run
assert run["hops"], run
'; then
    echo "$MTR_RESP"
    break
  fi
  if (( SECONDS >= MTR_DEADLINE )); then
    echo "timed out waiting for a successful MTR run: $MTR_RESP" >&2
    exit 1
  fi
  sleep 1
done

echo "=== Deleting link ==="
api_curl -fsS -X DELETE "$BASE_URL/api/links/$LINK_ID"
echo
api_curl -s "$BASE_URL/api/links" | python3 -c "
import sys, json
links = json.load(sys.stdin)
assert all(link['id'] != '$LINK_ID' for link in links), 'deleted link is still listed'
"

# Check server logs
echo "=== Server log (last 20 lines) ==="
tail -20 /tmp/netprob-server.log

echo "=== JKT agent log ==="
cat /tmp/jkt-agent.log

echo "=== SG agent log ==="
cat /tmp/sg-agent.log
