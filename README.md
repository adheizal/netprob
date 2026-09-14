# NetProb

NetProb monitors bidirectional network connectivity using lightweight agents controlled by a central Go server. Agents execute ping and MTR probes, return results over WebSocket, and expose collected data through a React dashboard and REST API.

## Components

```text
Agent A ──┐                          ┌── REST API
          ├── WebSocket ── Server ──┤
Agent B ──┘              SQLite     └── Embedded React UI
                              └──── optional VictoriaMetrics mirror ← Grafana
```

- **Controller:** schedules probes, manages agents/links, and serves API/UI traffic.
- **Agent:** connects outbound and runs ping or MTR.
- **Web UI:** React, TypeScript, Vite, Tailwind CSS, and Chart.js.
- **Storage:** SQLite for API/UI data with optional VictoriaMetrics mirroring.

## Requirements

- Go compatible with `go.mod`.
- Node.js and npm for frontend development.
- SQLite build dependencies required by `go-sqlite3`.
- Linux agents with `ping` and `mtr` installed.
- Root or `CAP_NET_RAW` on agents when required by ping.

NetProb uses the JSON report output supported by mtr 0.93 and newer. Successful and failed scheduled MTR attempts are retained in MTR History; a failed run displays its error instead of appearing as missing history.

### Agent Resource Footprint

The following measurements are a representative production sample from two
anonymized Linux hosts. Both agents had an active bidirectional link, with ping
running every 10 seconds and MTR every 120 seconds:

| Resource | Agent A | Agent B |
|---|---:|---:|
| Resident memory (RSS) | 9.8 MiB | 7.6 MiB |
| Peak service memory | 9.9 MiB | 9.2 MiB |
| CPU, including probe child processes | 0.047% of one core | 0.038% of one core |
| Threads | 11 | 11 |
| Open file descriptors | 7 | 7 |
| Static agent binary | 5.63 MiB | 5.63 MiB |
| WebSocket TCP payload | about 94 B/s | about 94 B/s |
| Estimated daily TCP payload | about 7.7 MiB | about 7.7 MiB |

Together, the two agents used approximately 17.3 MiB of resident memory and
0.085% of one CPU core during the sample. The traffic estimate covers NetProb's
WebSocket payload in both directions; TCP/IP overhead adds some network usage.
Actual consumption varies with ping and MTR intervals, route length, reconnects,
and the number of configured directions.

A Go process may report roughly 1 GiB or more of virtual address space even
while its RSS remains below 10 MiB. This is address space reserved by the Go
runtime and must not be interpreted as physical RAM consumption.

## Build

### Versioned Release

Build the controller binary, static agent binaries, and Docker image with one
shared version:

```bash
make release VERSION=0.1.0 IMAGE=netprob
```

The release produces:

```text
dist/netprob-server-linux-amd64
dist/netprob-agent-linux-amd64
dist/netprob-agent-linux-arm64
Docker image netprob:0.1.0
```

All binaries and the image contain the same version, commit, and UTC build
timestamp. Inspect them with:

```bash
./dist/netprob-server-linux-amd64 -version
./dist/netprob-agent-linux-amd64 -version
docker run --rm netprob:0.1.0 -version
```

### Automated GitHub Release

Every push and pull request to `main` runs frontend lint/build, Go formatting,
race tests, vet, static agent checks, and the end-to-end test. A semantic version
tag publishes the release binaries and a multi-architecture container image:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The workflow creates a GitHub Release containing the controller and agent
binaries plus `SHA256SUMS`. It also publishes `linux/amd64` and `linux/arm64`
images to:

```text
ghcr.io/adheizal/netprob:0.2.0
ghcr.io/adheizal/netprob:0.2
ghcr.io/adheizal/netprob:latest
```

Pull and run a published controller image with:

```bash
docker pull ghcr.io/adheizal/netprob:0.2.0
docker run -d --name netprob \
  -p 18080:8080 \
  -v netprob-data:/var/lib/netprob \
  ghcr.io/adheizal/netprob:0.2.0
```

Prerelease tags such as `v0.2.0-rc.1` are marked as prereleases and do not move
the `latest` image tag. GHCR publishing uses the repository's built-in
`GITHUB_TOKEN`; no registry password is required.

`VERSION` should be set explicitly for a release. When omitted, the Makefile
uses `git describe` and falls back to `dev` outside a Git checkout. A configured
YAML `agent.version` remains an explicit override; otherwise agents report the
embedded build version to the controller and Agents UI.

To build only one output category:

```bash
make release-binaries VERSION=0.1.0
make docker-image VERSION=0.1.0 IMAGE=netprob
```

### Development Build

```bash
cd web
npm ci
npm run lint
npm run build

cd ..
rsync -a --delete web/dist/ internal/webui/dist/
go build -o bin/netprob ./cmd/netprob
```

The synchronization step is required because Go embeds `internal/webui/dist/`, not `web/dist/`.

### Linux Agent for Older Distributions

The regular `netprob` binary includes the controller and SQLite, so it uses CGO and may inherit the build host's glibc requirement. Build the dedicated agent artifact for Ubuntu 20.04 or other older Linux distributions:

```bash
make agent-linux-amd64
# output: dist/netprob-agent-linux-amd64
```

For ARM64 hosts, use `make agent-linux-arm64`; `make agent-linux` builds both architectures. These agent-only artifacts use pure Go (`CGO_ENABLED=0`) and do not depend on the target host's glibc. They intentionally support only `-mode agent`; use the regular build for the controller. Pass an explicit release version, such as `make agent-linux VERSION=0.2.0`, to embed it; otherwise the Git-derived development version is used. Ready-to-deploy artifacts are written under `dist/`.

Select the artifact using the target host architecture reported by `uname -m`:

| `uname -m` | Artifact |
|---|---|
| `x86_64` | `netprob-agent-linux-amd64` |
| `aarch64`, `arm64` | `netprob-agent-linux-arm64` |

### Docker

The image contains the embedded dashboard, controller, agent runtime, `ping`,
and `mtr`. It runs the controller by default:

```bash
docker run -d --name netprob \
  -p 18080:8080 \
  -v netprob-data:/var/lib/netprob \
  -e NETPROB_ADMIN_EMAIL=admin@example.com \
  -e NETPROB_ADMIN_PASSWORD='replace-this-password' \
  netprob:0.1.0
```

Open `http://<host>:18080`. The named volume keeps the SQLite database across
container replacement. The bootstrap email/password only initialize a new
database and do not overwrite an existing admin account.

The same image can run as an agent. Host networking gives probes and automatic
address detection the host network view; `NET_RAW` permits ICMP:

```bash
docker run -d --name netprob-agent \
  --network host \
  --cap-add NET_RAW \
  --hostname agent-jakarta \
  -e NETPROB_CONTROLLER=http://controller.example.com:18080 \
  -e NETPROB_AGENT_TOKEN='<registration-token>' \
  netprob:0.1.0 -mode agent
```

Use a stable `--hostname` (or `NETPROB_AGENT_ID`) so replacing the container
reuses the same agent identity.

On Ubuntu 20.04, install the runtime probe commands and CA certificates:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates iputils-ping mtr-tiny
chmod +x netprob-agent-linux-amd64
```

The same persistent agent environment variables are used when starting this artifact:

```bash
NETPROB_CONTROLLER=https://controller.example.com \
NETPROB_AGENT_TOKEN='<registration-token>' \
./netprob-agent-linux-amd64 -mode agent
```

## Run the Controller

```bash
NETPROB_SERVER_LISTEN=:18080 \
NETPROB_DATA_DIR=./data \
./bin/netprob -mode server
```

Defaults are `:8080` and `./data`. If port `8080` is occupied, select an available port and use the same controller URL for agents and Vite.

YAML configuration is also supported:

```bash
./bin/netprob -mode server -config config.yaml
```

An explicitly supplied configuration path must exist and contain valid YAML.

## Register and Run an Agent

Registration creates a reusable enrollment token. The token value is shown only once in the registration response, so save it securely. Each connecting machine automatically becomes a separate agent identity and synchronizes its hostname, version, capabilities, region, and network addresses.

```bash
curl -X POST http://127.0.0.1:18080/api/agents/register \
  -H 'Content-Type: application/json' \
  -d '{}'
```

The same token can be installed on multiple machines. The controller combines the token with the agent's instance ID, which defaults to its hostname, so every machine appears separately in the UI while reconnects retain the same database identity. If two machines intentionally share a hostname, set a unique `NETPROB_AGENT_ID` on each one.

Copy the matching static artifact to the agent host, then start it:

```bash
chmod +x ./netprob-agent-linux-amd64
NETPROB_CONTROLLER=http://controller.example.com:18080 \
NETPROB_AGENT_TOKEN='<registration-token>' \
./netprob-agent-linux-amd64 -mode agent
```

The agent name defaults to the operating-system hostname, so `NETPROB_AGENT_NAME` is optional. The agent advertises non-loopback global-unicast IPv4 and IPv6 interface addresses. Loopback and link-local addresses are excluded, IPv4 addresses are listed first, and duplicates are removed. If detection returns no usable address, metadata supplied through the registration API is preserved as a fallback.

The primary address is selected from the local side of the established WebSocket connection, which identifies the interface used by the default route toward the controller. Link creation uses this address instead of the first detected bridge/container address. `NETPROB_AGENT_PRIMARY_ADDRESS` can override it for VPN or overlay-network monitoring.

At process startup, the agent performs one GeoIP lookup and reuses the result across WebSocket reconnects. The default ip-api-compatible endpoint detects the agent's public egress IP, location, AS name, and ISP. The Agents page shows the AS/ISP in its Provider column so the public network or cloud provider is easy to identify. GeoIP failure is logged but never prevents the agent from connecting. Set a manual region when the provider result is unsuitable, or set an empty GeoIP URL to disable lookup:

```bash
NETPROB_AGENT_REGION=jakarta \
NETPROB_GEOIP_URL= \
NETPROB_CONTROLLER=http://controller.example.com:18080 \
NETPROB_AGENT_TOKEN='<registration-token>' \
./netprob-agent-linux-amd64 -mode agent
```

### Persistent Agent Service

Store the controller URL and reusable enrollment token in a root-readable environment file so they survive restarts and binary upgrades:

```bash
sudo install -m 0755 ./netprob-agent-linux-amd64 /usr/local/bin/netprob-agent
sudo install -d -m 0750 /etc/netprob
sudo sh -c 'umask 077; printf "%s\n" \
  "NETPROB_CONTROLLER=http://controller.example.com:18080" \
  "NETPROB_AGENT_TOKEN=<registration-token>" \
  > /etc/netprob/agent.env'
```

Create `/etc/systemd/system/netprob-agent.service`:

```ini
[Unit]
Description=NetProb Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=/etc/netprob/agent.env
ExecStart=/usr/local/bin/netprob-agent -mode agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and inspect the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now netprob-agent
sudo systemctl status netprob-agent
sudo journalctl -u netprob-agent -f
```

The hostname is detected automatically. GeoIP uses the public egress IP by default, so neither `NETPROB_AGENT_NAME` nor `NETPROB_AGENT_REGION` normally needs to be stored in the environment file. Keep `/etc/netprob/agent.env` when replacing the binary to retain the same agent identity.

## Frontend Development

```bash
cd web
cp .env.example .env
npm run dev
```

| Variable | Default | Purpose |
|---|---:|---|
| `VITE_API_TARGET` | `http://127.0.0.1:8080` | Backend for `/api` and `/ws` proxies |
| `VITE_DEV_HOST` | `0.0.0.0` | Development listen host |
| `VITE_DEV_PORT` | `5173` | Development port |
| `VITE_PREVIEW_PORT` | `4173` | Production preview port |

Example with backend port `18080`:

```bash
cd web
VITE_API_TARGET=http://127.0.0.1:18080 VITE_DEV_PORT=5173 npm run dev
```

## Configuration

### Controller

| Variable | Default | Description |
|---|---:|---|
| `NETPROB_SERVER_LISTEN` | `:8080` | HTTP/WebSocket listen address |
| `NETPROB_DATA_DIR` | `./data` | SQLite database directory |
| `NETPROB_ADMIN_EMAIL` | `admin@netprob.local` | Initial admin email, used only when creating the first admin account |
| `NETPROB_ADMIN_PASSWORD` | `changeme` | Initial temporary password, used only when creating the first admin account |
| `NETPROB_METRICS_BACKEND` | `sqlite` | `sqlite` or `victoriametrics`; the latter mirrors probe and controller metrics for Grafana |
| `NETPROB_VM_URL` | empty | VictoriaMetrics base URL, required when the backend is `victoriametrics` |

### Agent

| Variable | Required | Description |
|---|---:|---|
| `NETPROB_AGENT_TOKEN` | yes | Reusable enrollment token returned during registration |
| `NETPROB_CONTROLLER` | yes | Controller base URL including port |
| `NETPROB_AGENT_ID` | no | Stable instance ID; defaults to the hostname and only needs overriding for duplicate hostnames |
| `NETPROB_AGENT_NAME` | no | Defaults to the system hostname |
| `NETPROB_AGENT_REGION` | no | Manual region override; wins over GeoIP |
| `NETPROB_AGENT_PRIMARY_ADDRESS` | no | Manual primary target address; otherwise derived from the controller route |
| `NETPROB_GEOIP_URL` | no | ip-api-compatible URL; default `http://ip-api.com/json/`, empty disables lookup |
| `NETPROB_AGENT_MAX_CONCURRENT_JOBS` | no | Maximum simultaneous ping/MTR child processes; default `4`, maximum `64` |
| `NETPROB_AGENT_PROBE_TIMEOUT_SECONDS` | no | Hard timeout for each ping/MTR process; default `60`, maximum `3600` |

## API

Authentication is enabled by default. On a new database, sign in as `admin@netprob.local` with the temporary password `changeme`; the UI requires an immediate password change. Set `NETPROB_ADMIN_EMAIL` and `NETPROB_ADMIN_PASSWORD` before the first controller start to override those bootstrap credentials. Later environment changes do not overwrite an existing admin account.

The dashboard and management endpoints use a seven-day `HttpOnly`, `SameSite=Strict` session cookie. Expired sessions are removed at controller startup and then hourly. Login is limited to 10 attempts per client IP per minute. `/health` and the authentication discovery/login endpoints remain public. `/ws` uses the reusable agent enrollment token from its initial hello message and never accepts the admin session as agent authentication. Authentication can be disabled or re-enabled from Settings; disabling it intentionally makes the dashboard and management API public.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/auth/session` | Get authentication state and the current admin session |
| `POST` | `/api/auth/login` | Create an admin session |
| `POST` | `/api/auth/logout` | Revoke the current admin session |
| `PUT` | `/api/auth/account` | Change admin email/password and rotate the session |
| `POST` | `/api/agents/register` | Create and return a reusable agent enrollment token |
| `GET` | `/api/agents` | List agents; supports `page` and `page_size` pagination parameters |
| `GET` | `/api/agents/{id}` | Get an agent |
| `DELETE` | `/api/agents/{id}` | Delete an agent |
| `POST` | `/api/links` | Create a link |
| `GET` | `/api/links` | List links and direction summaries |
| `GET` | `/api/links/{id}` | Get a link |
| `PUT` | `/api/links/{id}` | Update a link |
| `DELETE` | `/api/links/{id}` | Delete a link |
| `POST` | `/api/links/{link_id}/directions` | Create a direction |
| `GET` | `/api/links/{link_id}/directions` | List directions |
| `PUT` | `/api/links/{link_id}/directions/{direction_id}` | Update probe intervals or enable/disable ping and MTR |
| `GET` | `/api/links/{link_id}/directions/{direction_id}/ping` | Query ping results |
| `GET` | `/api/links/{link_id}/directions/{direction_id}/mtr` | Query MTR runs |
| `GET` | `/api/mtr-runs/{id}` | Get one MTR run |
| `GET` | `/api/settings/retention` | Get ping and MTR history retention settings |
| `PUT` | `/api/settings/retention` | Update retention settings and immediately clean expired history |
| `PUT` | `/api/settings/security` | Enable or disable dashboard/API authentication |
| `GET` | `/health` | Health check |
| `GET` | `/ws` | Agent WebSocket endpoint |

Ping and MTR intervals can be configured for both directions while creating a link in the UI. After creation, open the link detail page and use the settings button on either direction to adjust that direction independently.
Links can be deleted from the Links page after confirmation; deletion also removes their directions and associated ping/MTR history.
Deleting an agent also disconnects it and transactionally removes directions and probe history that reference it. A link left without directions is removed as part of the same transaction.

The Agents page loads 20 records at a time and lets the user select 10, 20, 50, or 100 rows. The paginated API response includes `agents`, `page`, `page_size`, `total`, and `total_pages`; `page_size` is capped at 100. Calling `/api/agents` without pagination parameters retains the original array response for existing integrations.

A link already contains both A→B and B→A directions; do not create a second reversed link for the same pair. The Links page shows the latest latency and packet loss for both directions. Open a link to inspect latency history and MTR results.

Link Detail refreshes metrics and MTR history every 10 seconds. MTR details identify both endpoints by hostname and primary IP address. The Settings page controls local SQLite retention independently for ping metrics and MTR runs in days. A value of `0` keeps that history forever. Saving applies cleanup immediately, and the controller repeats cleanup hourly. Reducing retention permanently deletes data older than the selected limit, including the hops belonging to expired MTR runs.

Total ping loss is stored as a real sample with `100%` packet loss and no RTT values, so an outage remains distinguishable from a probe that never ran. SQLite runs with WAL mode, a five-second busy timeout, and foreign-key enforcement. Startup migration 9 removes relationship rows left orphaned by older releases.

## Grafana and VictoriaMetrics

Enable the VictoriaMetrics mirror on the controller:

```bash
NETPROB_METRICS_BACKEND=victoriametrics \
NETPROB_VM_URL=http://victoriametrics.example.internal:8428 \
./dist/netprob-server-linux-amd64 -mode server
```

Add the same VictoriaMetrics URL to Grafana as a **Prometheus** datasource, then
import [`grafana/netprob-overview.json`](grafana/netprob-overview.json). Select
that datasource when Grafana asks for `DS_PROMETHEUS`. The dashboard refreshes
every 30 seconds and includes controller inventory, agent availability, ping
RTT/loss/jitter, probe success, and bounded MTR summaries.

Probe series expose both stable IDs and readable labels:

- `link`, `link_id`, `direction_id`, and `target_address`
- `source`, `source_agent_id`, `source_address`, `source_region`, and `source_provider`
- `destination`, `destination_agent_id`, `destination_address`, `destination_region`, and `destination_provider`

`source` and `destination` contain hostnames. Provider uses GeoIP `asname` and
falls back to `isp`. Metadata is captured when each sample is exported, so a
hostname or metadata change starts a new labeled series while the stable agent
ID remains available.

Available ping and probe metrics:

```text
netprob_ping_rtt_avg_ms
netprob_ping_rtt_min_ms
netprob_ping_rtt_max_ms
netprob_ping_jitter_ms
netprob_ping_packet_loss_percent
netprob_ping_packets_sent
netprob_ping_packets_received
netprob_probe_success{probe="ping|mtr"}
```

Available MTR summary metrics:

```text
netprob_mtr_hop_count
netprob_mtr_destination_rtt_avg_ms
netprob_mtr_max_hop_loss_percent
```

The latest MTR route and its per-hop measurements are also exported for the
Grafana hop table:

```text
netprob_mtr_hop_observed_timestamp_seconds{hop_number,hop_host,hop_ip,...}
netprob_mtr_hop_observed_timestamp_milliseconds{hop_number,hop_host,hop_ip,...}
netprob_mtr_hop_loss_percent{hop_number,hop_host,hop_ip,...}
netprob_mtr_hop_rtt_avg_ms{hop_number,hop_host,hop_ip,...}
netprob_mtr_hop_rtt_best_ms{hop_number,hop_host,hop_ip,...}
netprob_mtr_hop_rtt_worst_ms{hop_number,hop_host,hop_ip,...}
```

These metrics expose hop hostnames and IP addresses to VictoriaMetrics. The
observation-time metric lets the dashboard select one coherent latest route per
direction even after a route changes. The table follows the dashboard
Source filter, sorts hop numbers numerically, and shows loss, average/best/worst
RTT, and the observation time. Complete historical run details remain in SQLite
and the NetProb UI.

Controller inventory is exported at startup and every 30 seconds:

```text
netprob_controller_info
netprob_controller_uptime_seconds
netprob_controller_agents
netprob_controller_agents_online
netprob_controller_links
netprob_controller_directions
netprob_agent_online
netprob_agent_last_seen_seconds
netprob_link_info
netprob_direction_info
netprob_direction_ping_enabled
netprob_direction_mtr_enabled
netprob_direction_ping_interval_seconds
netprob_direction_mtr_interval_seconds
```

NetProb still saves ping and complete MTR history to SQLite for its own UI.
VictoriaMetrics receives new samples only; existing SQLite history is not
backfilled. The retention setting in NetProb applies only to SQLite—configure
VictoriaMetrics retention separately. Keep VictoriaMetrics on a trusted network
or protect it independently; NetProb admin sessions do not authenticate direct
Grafana-to-VictoriaMetrics traffic.

## Testing

```bash
go test -race ./...
go vet ./...

cd web
npm run lint
npm run build
```

Verify the backward-compatible amd64 agent artifact is static:

```bash
make verify-agent-linux-amd64
```

Two-agent E2E test on a configurable port:

```bash
go build -o bin/netprob ./cmd/netprob
NETPROB_PORT=18080 ./test_end_to_end.sh
```

Keepalive regression test:

```bash
NETPROB_PORT=18080 NETPROB_E2E_WAIT_SECONDS=65 ./test_end_to_end.sh
```

The script uses isolated `/tmp` data and stops only processes it starts.

## WebSocket Envelope

All messages contain `type` and `payload`:

```json
{"type":"ping","payload":{}}
```

Jobs are sent as `type: "job"`; results use `type: "result"` with `job_id`, `status`, `error`, `ping_result`, and `mtr_run` fields in the payload.
