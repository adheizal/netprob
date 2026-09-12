# CLAUDE.md - NetProb Development Guide

## Project Overview

NetProb is a Go controller and probe agent with an embedded React dashboard. Agents connect outbound over WebSocket, execute scheduled ping or MTR jobs, and return results to the controller.

## Repository Layout

```text
netprob/
├── cmd/netprob/          # CLI entry point; server and agent modes
├── internal/config/      # YAML and environment configuration
├── internal/models/      # Domain and protocol models
├── internal/probe/       # ping and MTR command executors
├── internal/protocol/    # WebSocket envelope constants/types
├── internal/scheduler/   # Direction scheduling and job dispatch
├── internal/server/      # REST API, WebSocket hub, handlers
├── internal/storage/     # SQLite and VictoriaMetrics adapters
├── internal/webui/       # Embedded production frontend
├── web/                  # React, TypeScript, Vite and Tailwind source
├── README.md             # User-facing setup and API documentation
└── test_end_to_end.sh    # Two-agent integration test
```

## Development Environment

Run builds and tests on the canonical development host:

```text
root@139.99.122.135:/root/project/netprob
```

Do not assume port `8080` is available; it is used by another service on that host. Project test scripts default to `18080`.

## Configurable Ports

Backend:

```bash
NETPROB_SERVER_LISTEN=:18080 ./bin/netprob -mode server
```

Frontend development:

```bash
cd web
VITE_API_TARGET=http://127.0.0.1:18080 \
VITE_DEV_HOST=0.0.0.0 \
VITE_DEV_PORT=5173 \
npm run dev
```

See `web/.env.example` for all frontend environment variables.

## Build

### Versioned Release Artifacts

Use one explicit version for all distributable outputs:

```bash
cd /root/project/netprob
make release VERSION=0.1.0 IMAGE=netprob
```

This runs the frontend production build, embeds it in the controller, creates
the Linux controller and static Linux agent binaries under `dist/`, and builds
`netprob:0.1.0`. `make release-binaries` and `make docker-image` are available
when only one artifact category is needed.

Build metadata is injected into `internal/buildinfo` using Go linker flags.
`-version` prints the embedded version, commit, and UTC build timestamp. An
agent uses the embedded version unless `agent.version` is explicitly configured.
When `VERSION` is omitted, use `git describe` with `dev` as the no-Git fallback;
release automation must always pass `VERSION` explicitly.

The Dockerfile builds the React UI and Go controller in separate stages. The
runtime image is Debian-based and includes CA certificates, `ping`, and `mtr` so
it can run either `-mode server` (the default) or `-mode agent`. Controller data
belongs at `/var/lib/netprob` and must be backed by a volume.

### Development Build

Build the frontend first, synchronize it into the embedded filesystem, then build Go:

```bash
cd /root/project/netprob/web
npm ci
npm run lint
npm run build

cd ..
rsync -a --delete web/dist/ internal/webui/dist/
go build -o bin/netprob ./cmd/netprob
```

`web/dist/` is only Vite output. The Go binary embeds `internal/webui/dist/`; skipping the synchronization step serves an older UI.

### Backward-Compatible Linux Agent

Build distributable agent-only binaries without CGO so they do not inherit the build machine's glibc version:

```bash
make agent-linux
make verify-agent-linux-amd64
```

This produces `dist/netprob-agent-linux-amd64` and `dist/netprob-agent-linux-arm64`. The `agentonly` build tag excludes controller, embedded UI, storage, and `go-sqlite3` linkage. These artifacts accept `-mode agent` only; keep using the regular build for the controller.

The canonical ready-to-copy paths are:

```text
/root/project/netprob/dist/netprob-agent-linux-amd64  # x86_64
/root/project/netprob/dist/netprob-agent-linux-arm64  # aarch64/arm64
```

Release agent binaries receive their version from Makefile linker flags and
report it automatically in the WebSocket hello. Seeing `dev` means the binary
was built outside a Git checkout without an explicit `VERSION`, not that the
agent has a connectivity problem.

Production agents should keep the reusable enrollment token in a root-readable environment file (for example `/etc/netprob/agent.env`) and run the matching artifact through systemd. Identity is scoped by token plus agent instance ID, which defaults to hostname. Replacing `/usr/local/bin/netprob-agent` must not replace the environment file or create a new identity.

## Verification

```bash
cd /root/project/netprob
go test -race ./...
go vet ./...

CGO_ENABLED=0 go test -tags agentonly ./cmd/netprob
make verify-agent-linux-amd64
make agent-linux-arm64

cd web
npm run lint
npm run build

cd ..
NETPROB_PORT=18080 ./test_end_to_end.sh
```

To cross the WebSocket 60-second read deadline:

```bash
NETPROB_PORT=18080 NETPROB_E2E_WAIT_SECONDS=65 ./test_end_to_end.sh
```

The E2E script must stop only processes recorded in its own PID files. Never add broad `fuser -k` or `pkill` commands that could terminate unrelated services.

## Architecture Constraints

1. Only one goroutine may write to a Gorilla WebSocket connection at a time. Agent writes use `websocketWriter`; controller writes use `AgentConn.Send` and `writePump`.
2. The agent sends ping every 30 seconds. The controller must answer with pong before the agent's 60-second read deadline.
3. An old connection must not remove or mark a replacement connection offline.
4. Jobs are placed directly in `Envelope.Payload`. Results remain flat objects with `job_id`, `status`, `error`, `ping_result`, and `mtr_run`.
5. SQLite is the API/UI source of truth. When configured, VictoriaMetrics receives enriched ping samples, bounded MTR summaries, failed-ping status events, and 30-second controller inventory snapshots. Probe labels retain stable IDs and add hostname, address, link, region, and provider metadata. Do not export full per-hop MTR topology as labels.
6. API list endpoints must encode empty collections as `[]`, not `null`.
7. Client-side routes must fall back to embedded `index.html`.
8. Agent registration only issues identity and credentials. After token authentication, the WebSocket hello synchronizes hostname, version, capabilities, and usable global-unicast interface addresses. Invalid, loopback, and link-local hello addresses must never replace registered fallback addresses.
9. GeoIP is best-effort metadata enrichment: run it once at agent startup with a bounded timeout, reuse it across reconnects, persist public location plus `asname`/`isp` provider metadata, preserve stored values when it fails, and let a configured manual region override the provider.
10. The agent primary address is the local IP selected by the WebSocket route to the controller unless explicitly overridden. UI link defaults must prefer it over the first raw interface address.
11. Ping/MTR intervals and enabled state are direction-specific. The create-link UI applies its initial settings to both directions; the link detail UI can update each direction independently.
12. Link deletion must transactionally remove its directions and direction-scoped ping/MTR history so the scheduler cannot continue orphan jobs.
13. An enrollment token is persistent and reusable, although its plaintext value is returned only once. Resolve agents by `(token_hash, instance_id)`: the first instance claims the pending registration row, additional instance IDs create separate records, and reconnects reuse the matching record. Reject empty instance IDs and keep the pair unique.
14. One link models both A→B and B→A. The UI displays and configures the two directions inside that single link; users must not need duplicate reversed links.
15. Invoke mtr with JSON as the final output-mode option because mtr resolves conflicting output modes by argument order. Parse the nested `report.hubs` schema and accept both quoted and numeric hop numbers for mtr 0.93/0.95 compatibility. Persist failed MTR attempts with their error so an empty history never hides execution failures.
16. Retention settings are persistent application data. Zero means unlimited retention. A settings update cleans expired SQLite data immediately, the controller repeats cleanup hourly, and MTR cleanup must remove child hops together with expired runs.
17. Keep `grafana/netprob-overview.json` importable with a Prometheus-compatible datasource input. Dashboard queries must use the exported metric and label names; VictoriaMetrics retention is independent from the SQLite retention UI.
18. Link Detail refreshes probe data every 10 seconds without resetting the page to its initial loading state. MTR endpoint labels use the direction's synchronized hostname and primary address, with IDs only as fallback.
19. Dashboard authentication is enabled by default. Protect all management APIs with the admin session while leaving `/health`, authentication discovery/login, and agent `/ws` reachable. `/ws` must continue authenticating only through the token in the agent hello message. Rate-limit login per client IP and never expose raw storage errors in HTTP responses.
20. Bootstrap the singleton admin only when it does not exist, hash passwords with salted PBKDF2-SHA256, require the temporary password to be changed, rotate all sessions after a credential change, and store only session-token hashes. Authentication enablement is persistent and may be toggled from Settings; disabled means the management API is intentionally public. Delete expired sessions at startup and hourly.
21. WebSocket connection replacement and disconnect must close the socket and broadcast cancellation to every connection-owned goroutine. Wait for the writer pump before completing cleanup, and never let an old connection mark its replacement offline.
22. A parseable ping packet summary is a result even when the command exits non-zero for total loss. Persist 100% loss with null RTT fields; command startup, timeout, and unparsable output remain probe errors.
23. Serialize scheduler refresh and timestamp updates without holding its lock during job dispatch. Bound agent execution with a configurable worker pool and hard context deadline for each child process.
24. Open SQLite with WAL, a five-second busy timeout, and foreign-key enforcement. Agent deletion must transactionally remove related history and directions, and startup migrations must clean legacy orphans.
25. Link-list and MTR-history handlers must use bounded bulk queries rather than per-direction or per-run N+1 reads.

## Code Quality

- Add regression tests for backend bug fixes.
- Run `gofmt` on every changed Go file.
- Run `go test -race ./...`, `go vet ./...`, `npm run lint`, and `npm run build` before handoff.
- Validate release metadata with `-version` on the controller binary, agent binary, and Docker image.
- Keep ports configurable; do not hard-code a host-specific port in application code.
- Do not edit hashed files in `internal/webui/dist/` manually. Rebuild and synchronize them from `web/dist/`.
