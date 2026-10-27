# Tall Order

A tiny fruit store for giraffes: React with a 90s mail-order/web aesthetic,
an ASP.NET Core (.NET 10) API, and PostgreSQL 17. Buy one fruit, receive a real
order receipt, and see stock decrease. The staff room adds/removes inventory.
No payment processing, customer accounts, external services, or cloud deployment.
The ambiguous database brief is interpreted as PostgreSQL (not two databases);
the unspecified deployment target "A" is left out in favor of runnable Compose.

## Prerequisites and startup

Docker Engine or Docker Desktop with Docker Compose v2 and a running daemon.
Port 8080 must be free. No host .NET, Node, or database installation is needed.
From the repository root:

```sh
docker compose -f src/compose.yaml up --build -d --wait
docker compose -f src/compose.yaml ps
```

The foreground command also works: `docker compose -f src/compose.yaml up --build`.
Open **http://localhost:8080**. API requests use the same origin at `/api`;
`http://localhost:8080/health` checks database-backed readiness. Database and API
ports are not published. Only the frontend is exposed, bound to loopback.

### Podman

Podman with a Compose provider can run the exact same file:

```sh
podman compose -f src/compose.yaml up --build -d --wait
podman compose -f src/compose.yaml ps
```

Use `podman compose` in place of `docker compose` for all commands below,
including non-destructive shutdown. The Docker Compose provider needs Podman's
API socket; on systemd Linux, enable it with `systemctl --user start podman.socket`.

## Demo (about two minutes)

1. Browse four seeded fruits. Click **Buy one** to persist an order and decrement
   stock. The receipt shows the server-calculated total; no money changes hands.
2. Open **Staff room**, enter local demo password **`4`**, and add a new fruit
   with price and stock. Switch to **Fruit stand** and buy it.
3. Refresh the browser to show persisted stock. Return to the staff room to remove
   the fruit (a confirmation is required). Removal preserves historical orders.
4. Restart the containers, wait for readiness, and refresh. Added fruit, purchases,
   stock, and removals survive. Seeding runs once per database, not every restart:

```sh
docker compose -f src/compose.yaml restart
docker compose -f src/compose.yaml up -d --wait
```

The named volume `tall-order-demo_fruit-data` stores PostgreSQL data. Initialization
and seed versioning run in a transaction with an advisory lock. The API retries
transient initialization failures, and Compose waits for database/API health.
Purchases atomically decrement stock and insert an order in one transaction;
concurrent shoppers cannot buy the same last fruit.

## Configuration

All defaults below are **local-only demo credentials**, not real secrets.
Do not expose this deliberately weak password gate on a public network. There
are no sessions or user accounts; each admin request is checked server-side.
The browser keeps the entered password only in memory until lock/reload.

| Variable | Default | Purpose |
|---|---|---|
| `FRONTEND_PORT` | `8080` | Host loopback port |
| `DB_PASSWORD` | `local-fruit-demo` | PostgreSQL password, passed to both database and API |
| `ADMIN_PASSWORD` | `4` | Demo staff-room gate |
| `ConnectionStrings__Store` | Set by Compose | API PostgreSQL connection string; `Host=db` between containers |
| `Admin__Password` | Set from `ADMIN_PASSWORD` | Required API setting |
| `ASPNETCORE_URLS` | `http://+:8080` | API listening address |
| `API_UPSTREAM` | `http://api:8080` | Frontend nginx API proxy target (no trailing slash) |

Set Compose overrides in your shell or a git-ignored `src/.env`; for example:

```sh
FRONTEND_PORT=8090 ADMIN_PASSWORD=4 docker compose -f src/compose.yaml up --build -d --wait
```

Changing `DB_PASSWORD` does not rotate a password inside an existing database;
existing volumes must use their original password unless rotated deliberately.
For another orchestrator, reuse the two Dockerfiles, provide the API settings
and proxy upstream, attach persistent PostgreSQL storage, and route traffic to
the frontend. No orchestrator-specific SDK or discovery code is used.

## OpenTelemetry

The API instruments incoming ASP.NET Core requests, outgoing HTTP calls,
PostgreSQL commands (`Npgsql`), HTTP/database metrics, and .NET runtime metrics.
Application logs include trace/span correlation; successful purchases produce
an order log. Existing console logging remains enabled. Request bodies, admin
password headers, and database parameter values are not explicitly captured.

OTLP export of **logs, traces, and metrics** is enabled only when
`OTEL_EXPORTER_OTLP_ENDPOINT` is set. No collector is required or started by
default. Provide an existing OTLP receiver using these standard settings:

| Variable | Default | Purpose |
|---|---|---|
| `OTEL_SERVICE_NAME` | `tall-order-api` | Telemetry service identity |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Unset | Receiver base URL; enables export |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` | Use `grpc` (usually port 4317) or `http/protobuf` (usually 4318) |
| `OTEL_EXPORTER_OTLP_HEADERS` | Unset | Optional receiver credentials; supply externally, never commit |

For example, if your existing receiver is reachable at `otel-collector` on the
same container network:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 \
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf \
podman compose -f src/compose.yaml up --build -d --wait
```

Use the receiver's reachable address, not `localhost` for a different container.
For local `dotnet run`, set these same environment variables directly.
Buy a fruit and look for the `POST /api/orders` server span, its child PostgreSQL
spans, the correlated `Order … placed …` log, and `http.server.request.duration`
metrics in your receiver. HTTP/protobuf export appends `/v1/traces`, `/v1/metrics`,
and `/v1/logs` to the base endpoint. Metric export is periodic (normally 60 seconds).

## Focused checks

### Local API launch profile

`src/api/Properties/launchSettings.json` provides an `http` development profile
at **http://localhost:5080**, matching the Vite development proxy. It uses the
local-only admin password `4` and does not change the Compose configuration.
Set `ConnectionStrings__Store` to a PostgreSQL instance reachable from your host
before launching; the Compose database is intentionally not exposed to the host.

```sh
dotnet run --project src/api/Store.Api.csproj --launch-profile http
```

### Build and workflow tests

The running stack is the test environment. Host checks require Node 22.12+ (or 24)
and, for the standalone API build, .NET SDK 10. Run from the repository root:

```sh
dotnet build src/api/Store.Api.csproj -c Release
npm --prefix src/frontend ci
npm --prefix src/frontend run build
cd src/frontend
npx playwright install chromium
# On Linux, if browser system libraries are missing:
# npx playwright install-deps chromium
npm test
```

Set `BASE_URL` if the frontend uses a different port, and `ADMIN_PASSWORD` if
overridden. Tests exercise the actual API/database through nginx, including
password rejection, validation, concurrent last-item purchases, and the complete
add → buy → refresh → remove browser workflow. They also check loading, empty,
and error UI states. Test products are removed, not seeded catalog items.

Run the restart-persistence check from the repository root (Node 22.12+).
It restarts only this demo stack, verifies the catalog and stored order, then
removes its temporary fruit. Like normal removals, test order history is retained:

```sh
node src/tests/persistence.mjs
# With Podman:
CONTAINER_CLI=podman node src/tests/persistence.mjs
```

To inspect persisted orders after a demo purchase:

```sh
docker compose -f src/compose.yaml exec db psql -U giraffe -d tallorder \
  -c 'SELECT id, product_name, total FROM orders ORDER BY created_at DESC LIMIT 5;'
docker compose -f src/compose.yaml logs --tail=50 api
```

## Normal shutdown (keeps data)

```sh
docker compose -f src/compose.yaml down
```

The next startup reuses the named volume. `stop` also preserves containers/data.

### Optional destructive reset — deletes all inventory and orders

Only when you explicitly want a fresh database and the original sample fruit:

```sh
docker compose -f src/compose.yaml down --volumes
docker compose -f src/compose.yaml up --build -d --wait
```
