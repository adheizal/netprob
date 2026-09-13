# AGENTS.md - Repository Instructions

## Scope

These instructions apply to the entire NetProb repository.

## Components

- Go controller exposing REST and WebSocket endpoints.
- Go agents connecting outbound and executing ping/MTR jobs.
- React dashboard embedded into the Go binary.
- SQLite persistence with optional VictoriaMetrics mirroring.

## Development Host

Build and test on:

```text
root@139.99.122.135:/root/project/netprob
```

Synchronize only files relevant to the current change and preserve unrelated work. Port `8080` is occupied on this host; never stop or replace that service.

Use a configurable backend port, normally `18080`:

```bash
NETPROB_SERVER_LISTEN=:18080 ./bin/netprob -mode server
```

Point Vite at the same backend:

```bash
cd web
VITE_API_TARGET=http://127.0.0.1:18080 npm run dev
```

## Required Validation

Backend changes:

```bash
gofmt -w <changed-go-files>
go test -race ./...
go vet ./...
```

Changes affecting Linux agent packaging or dependencies:

```bash
make verify-agent-linux-amd64
make agent-linux-arm64
CGO_ENABLED=0 go test -tags agentonly ./cmd/netprob
```

Frontend changes:

```bash
cd web
npm ci
npm run lint
npm run build
```

Changes affecting scheduling, API storage, WebSockets, probes, ports, or agent lifecycle:

```bash
cd /root/project/netprob
go build -o bin/netprob ./cmd/netprob
NETPROB_PORT=18080 ./test_end_to_end.sh
```

Use `NETPROB_E2E_WAIT_SECONDS=65` when validating keepalive behavior.

## Embedded Frontend

The Go server does not serve `web/dist/` directly. After a frontend build:

```bash
cd /root/project/netprob
rsync -a --delete web/dist/ internal/webui/dist/
go build -o bin/netprob ./cmd/netprob
```

Do not hand-edit hashed assets under `internal/webui/dist/`.

## Implementation Invariants

- Serialize every WebSocket write.
- Keep ping/pong bidirectional and stop connection-scoped goroutines when their connection closes.
- Treat connection shutdown as a broadcast, close the underlying WebSocket on every exit path, and wait for the connection writer to stop.
- Never let a stale connection mark its replacement offline.
- Dispatch a direction to its source agent and derive direction online state from that source.
- Encode empty API collections as JSON arrays.
- Preserve SPA fallback for client-side routes.
- Save ping and complete MTR results in SQLite. When VictoriaMetrics is configured, mirror enriched ping metrics, bounded MTR summaries, failed-ping status, latest MTR route metadata and numeric per-hop measurements, and periodic controller inventory. Never put error text or numeric measurements in labels.
- Persist a parseable ping summary even when `ping` exits non-zero for total packet loss; keep unavailable RTT fields null.
- Treat probe targets as data, never command options.
- Validate direction intervals as positive values and expose per-direction probe settings through the UI.
- Delete a link and all direction-scoped ping/MTR data atomically; never leave orphan directions scheduled.
- Delete an agent, its directions, and related probe history transactionally; enable SQLite WAL, busy timeout, and foreign-key enforcement on every connection.
- Protect scheduler timestamps with the scheduler lock and never dispatch while holding that lock.
- Bound agent job concurrency and execute every probe with a context deadline.
- Keep MTR JSON parsing compatible with the nested `report.hubs` output from mtr 0.93/0.95, and persist failed attempts so the UI can expose probe errors.
- Persist ping and MTR retention independently in days; zero keeps history forever. Clean immediately after updates and hourly thereafter, including child hops for expired MTR runs.
- Keep Link Detail live with a 10-second refresh and render MTR endpoints from synchronized hostname and primary-address metadata, falling back to IDs only when metadata is unavailable.
- Enable admin authentication by default for the dashboard and every management API. Keep `/health`, auth discovery/login, and `/ws` reachable; agent WebSockets authenticate separately using the enrollment token in hello.
- Store salted password hashes and hashed session tokens only. Force bootstrap-password replacement, revoke prior sessions when credentials change, and keep the persistent Settings toggle explicit because disabling auth exposes management data and mutations.
- Error when an explicitly requested configuration file cannot be read.
- Keep `grafana/netprob-overview.json` compatible with a Prometheus datasource pointed at VictoriaMetrics. Preserve stable ID labels alongside readable hostname, address, link, region, and provider labels.

## Safety and Documentation

- Test scripts must track and stop only processes they start.
- Use dedicated `/tmp/netprob-*` directories for destructive E2E fixtures.
- Do not commit tokens, `.env` files, databases, logs, or `web/dist/`.
- Keep `README.md` user-facing and `CLAUDE.md` focused on detailed development workflow.

## Agent Operation

### Distribution Artifacts

Create all release outputs with one explicit semantic version:

```bash
make release VERSION=0.1.0 IMAGE=netprob
```

This produces the controller and agent binaries under `dist/` plus the Docker
image `netprob:0.1.0`. Use `make release-binaries` or `make docker-image` for an
individual output category. Every release artifact must receive the same
`VERSION`, `COMMIT`, and `BUILD_DATE` linker metadata and support `-version`.
Release automation must pass `VERSION`; the Git-derived/default `dev` value is
for development builds only.

Build output on the canonical development host is stored in:

```text
/root/project/netprob/dist/netprob-server-linux-amd64
/root/project/netprob/dist/netprob-agent-linux-amd64
/root/project/netprob/dist/netprob-agent-linux-arm64
```

Deploy the amd64 artifact to `x86_64` hosts and the arm64 artifact to `aarch64`/`arm64` hosts. Keep controller builds under `bin/`; never deploy the CGO-linked controller binary as the compatibility artifact for Ubuntu 20.04.

### Lifecycle

1. Register through `POST /api/agents/register` with an empty JSON object and securely save the returned reusable enrollment token, which is displayed only once.
2. Connect outbound to `ws://<controller>/ws` or `wss://<controller>/ws`.
3. Detect non-loopback global-unicast interface addresses and send them with hostname, version, capabilities, and token in the authenticated `hello` envelope.
4. Execute dispatched ping/MTR jobs and return result envelopes.
5. Send keepalive every 30 seconds; the controller must answer before the 60-second read deadline.

### Agent Configuration

| Variable | Required | Description |
|---|---:|---|
| `NETPROB_AGENT_TOKEN` | yes | Token returned during registration |
| `NETPROB_CONTROLLER` | yes | Controller URL including its port |
| `NETPROB_AGENT_ID` | no | Stable instance ID; defaults to hostname |
| `NETPROB_AGENT_NAME` | no | Defaults to the system hostname |
| `NETPROB_AGENT_REGION` | no | Manual region override; wins over GeoIP |
| `NETPROB_AGENT_PRIMARY_ADDRESS` | no | Manual primary address override for VPN/overlay routing |
| `NETPROB_GEOIP_URL` | no | GeoIP provider URL; empty disables lookup |
| `NETPROB_AGENT_MAX_CONCURRENT_JOBS` | no | Concurrent probe worker limit; default 4, maximum 64 |
| `NETPROB_AGENT_PROBE_TIMEOUT_SECONDS` | no | Per-probe hard timeout; default 60 seconds, maximum 3600 |

Equivalent YAML:

```yaml
agent:
  id: "jkt-01"
  name: "jkt-01"
  controller: "http://10.0.0.1:18080"
  token: "agent-token-here"
  version: "1.0.0"
  region: "jakarta"
  primary_address: "100.81.110.123"
  geoip_url: "http://ip-api.com/json/"
  max_concurrent_jobs: 4
  probe_timeout_seconds: 60
  capabilities:
    ping: ""
    mtr: ""
```

Run an agent:

```bash
NETPROB_CONTROLLER=http://127.0.0.1:18080 \
NETPROB_AGENT_TOKEN='<registration-token>' \
./dist/netprob-agent-linux-amd64 -mode agent
```

Agents require `ping`, `mtr`, and sufficient raw ICMP privileges. For connection failures, verify `/health`, the controller port, firewall access, and the persistent registration token.

Enrollment tokens are reusable across physical agents. Resolve identity using the combination of token hash and non-empty hello `agent_id`; the first instance claims the pending registration row and later instances create separate records sharing the token hash. A reconnect with the same combination must reuse its record. The agent ID defaults to hostname; duplicate hostnames require distinct `NETPROB_AGENT_ID` overrides. Store production credentials in a root-readable environment file such as `/etc/netprob/agent.env`.

Use the `agentonly` build tag with `CGO_ENABLED=0` for distributable Linux agent binaries. This excludes controller/SQLite code, avoids a host glibc dependency, and allows the artifacts produced by `make agent-linux` to run on Ubuntu 20.04. Agent-only artifacts must reject `-mode server` clearly.

An empty `agent.version` uses the version embedded by linker flags. Keep
`agent.version` as an explicit configuration override for compatibility. A
release-built agent must therefore display the supplied release version in the
Agents UI; `dev` is only the fallback for builds without release/Git metadata.

The Docker image runs controller mode by default, persists SQLite under
`/var/lib/netprob`, and includes `ping` plus `mtr` for optional agent mode. Mount
a persistent volume for controller data. Agent containers require `NET_RAW` and
normally host networking so probes and automatic address detection see the host
network; keep their hostname or `NETPROB_AGENT_ID` stable across replacement.

Registration metadata remains a compatibility fallback. A successfully authenticated hello must update the stored hostname, version, capabilities, and addresses; an empty or unusable detected address list must not erase existing manually registered addresses.

GeoIP lookup runs once per agent process with a short timeout and is cached for reconnects. It must never block the WebSocket lifecycle. Synchronize the provider's `asname` and `isp` metadata so the Agents UI can identify the public network/cloud provider. `agent.region`/`NETPROB_AGENT_REGION` overrides the provider result, and an empty `NETPROB_GEOIP_URL` disables external lookup.

Derive the default primary address from the local address of the established controller WebSocket. Link creation must prefer `primary_address` over the raw interface list. Preserve `NETPROB_AGENT_PRIMARY_ADDRESS` as an explicit override for VPN and overlay routes.

A logical link contains both A→B and B→A directions. The UI must not require users to create a second reversed link for the same agent pair. Display current latency/loss per direction, allow direction-specific probe settings, and expose confirmed link deletion from the Links page.
