# Purr & Prose

A tiny cat-run bookshop with a 1990s-web aesthetic: a Vite/vanilla-JavaScript
frontend, a Go HTTP API using the official Azure Cosmos DB SDK, and the free
Cosmos DB Linux vNext emulator. No cloud account, paid services, or checkout.
The complete workflow is **staff sign in -> add a listing -> browse the public
shelf -> remove a listing**.

## Prerequisites and startup

- Docker Engine / Docker Desktop running Linux containers and Docker Compose v2.
- Internet access for the first image pulls and dependency restores. Allow a few
  minutes and several GB of disk space for the emulator and build images.
- Ports 8088, 8086, and 8081 free. Go/Node are **not** required to run the containers.
- The Linux vNext emulator is a development emulator, not production Cosmos DB.
  See its [limitations and supported platforms](https://learn.microsoft.com/azure/cosmos-db/emulator-linux).

From the repository root, with no configuration files to create:

```sh
docker compose -f src/compose.yaml up --build
```

For the same app in the background:

```sh
docker compose -f src/compose.yaml up --build -d --wait --wait-timeout 240
```

Open **http://localhost:8088**. Staff portal: **http://localhost:8088/#admin**.
The local-only demo password is **`alligator123`** (API username: `admin`).
Use the public search to find a title, author, or cat seller.
The API is also available at **http://localhost:8086** and the Cosmos gateway at
**http://localhost:8081**; all three host ports bind only to loopback.

### This machine: Podman compatibility

This demo was verified with Docker Compose v2 against Podman's Docker-compatible
socket, not a native Docker daemon. User-local Docker CLI and Compose binaries
were installed because this machine had Podman but no `docker` command.
In a fresh terminal on this machine, select its existing rootless socket:

```sh
systemctl --user start podman.socket
export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
export DOCKER_BUILDKIT=0 COMPOSE_BAKE=false
```

Then use the same `docker compose` commands above and below. These environment
settings are not needed on a normal Docker Engine/Desktop installation.

Compose waits for database readiness, then API initialization, then serves the
frontend. The API retries initialization for up to three minutes, creates its
database and `/shelf`-partitioned container automatically, and atomically seeds
four books plus a seed marker once. Restarting never duplicates samples,
overwrites edits, or resurrects removed samples. Data lives in the named
`purr-and-prose_cosmos-data` volume.

## Configuration

All configuration is runtime environment configuration, not compiled frontend
settings. Browser requests use `/api` on the frontend origin; Nginx proxies them
to the API, so no CORS configuration or browser-visible container DNS is needed.

| Variable | Default / purpose |
| --- | --- |
| `FRONTEND_PORT` | `8088`; Compose host port, bound only to `127.0.0.1` |
| `ADMIN_PASSWORD` | `alligator123`; local-only demo default, with a startup warning if unset |
| `COSMOS_KEY` | Public base64 local-demo placeholder in Compose; required by SDK |
| `COSMOS_ENDPOINT` | API: `http://cosmos:8081` |
| `COSMOS_ALLOW_HTTP` | API: `true` only for the local HTTP emulator |
| `COSMOS_DATABASE` | API: `catbooks` |
| `COSMOS_CONTAINER` | API: `books` |
| `HTTP_ADDR` | Explicit API listen address; takes precedence over `PORT` |
| `PORT` | Aspire-managed API port; otherwise the API defaults to `:8080` |
| `API_UPSTREAM` | Frontend Nginx: `http://api:8080`; Vite dev proxy also accepts it |

`FRONTEND_PORT`, `ADMIN_PASSWORD`, and `COSMOS_KEY` can be overridden in the shell when running Compose. Other
values are ordinary service environment entries in `compose.yaml`; change them
there or provide a Compose override when needed. For example:

```sh
FRONTEND_PORT=8090 ADMIN_PASSWORD='another-local-password' docker compose -f src/compose.yaml up --build
```

The emulator **does not enforce database authentication**. Its key placeholder
is not a real secret. Database and API ports are published only on loopback.
Admin checks happen on the server for every write, using HTTP Basic credentials
held only in browser memory (no local storage or cookies). Reload or lock the
portal to discard them. HTTP and a shared public password are suitable only for
a trusted local demo. Do not expose this stack to the internet. Real deployment
would require HTTPS, real identity/access controls, and securely supplied
database credentials; none of those services are provisioned here.

## Run with the existing Aspire AppHost

The repository also has a file-based `apphost.cs` for local development. This
workflow requires the Aspire CLI 13.6.4, .NET 10+, Go 1.23+, Node 22.12+, and a
working container runtime. It runs Go and Vite on the host and starts its own
Cosmos emulator container; the Compose workflow above remains independent.

From the repository root:

```sh
# On this Podman machine, select the rootless container socket first:
export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"

aspire start --apphost ./apphost.cs
aspire wait frontend --apphost ./apphost.cs
aspire describe --apphost ./apphost.cs
```

Open the `frontend` HTTP URL printed by `aspire describe`, or use the dashboard
link from startup. Aspire allocates ports dynamically, including the API HTTP
endpoint and Cosmos HTTPS gateway. The staff portal is `/#admin` on that frontend
URL and uses the same **local-only** `alligator123` password.

On the `full-aspireified` branch, the AppHost uses **only `WithReference` for
dependency configuration**: API -> `books`, frontend -> API. Connection values
are not manually copied into environment variables in the AppHost.
`.WithHttpEndpoint(env: "PORT")` supplies the API's allocated listening port.
The API and frontend wait for their dependencies to become healthy.

The Go API consumes Aspire 13.6's individual connection properties without
parsing or rebuilding a connection string:

| Injected variable | Source |
| --- | --- |
| `BOOKS_URI` | Cosmos account endpoint |
| `BOOKS_ACCOUNTKEY` | Emulator account key |
| `BOOKS_DATABASENAME` | Referenced database name |
| `BOOKS_CONTAINERNAME` | Referenced container name |

If any of those reference variables are present, all four are required.
Incomplete references fail explicitly rather than silently mixing in standalone
`COSMOS_*` settings. With no reference properties, the API uses the existing
Compose/standalone configuration described above.

Vite reads `services__api__http__0` from `.WithReference(backend)` and proxies
`/api` and `/healthz` to that URL. The browser still calls the frontend origin;
no database key or connection string is exposed to browser JavaScript. Outside
Aspire, `API_UPSTREAM` (or `http://localhost:8080` for standalone development)
remains supported. Malformed discovery settings fail instead of falling back.
Production builds do not need discovery settings. Nginx startup reads the same
Aspire discovery variable, falling back to `API_UPSTREAM` for standalone Compose.

The dashboard exposes **`admin-password`** as a secret parameter. The AppHost
passes it to the API as `ADMIN_PASSWORD`; the local-only default is
`alligator123`. Override it using AppHost user secrets or the environment:

```sh
Parameters__admin_password='another-local-password' aspire start --apphost ./apphost.cs
```

Restart the API after changing its parameter value. Publishing does not give
this secret a default: deployment must supply it. The standalone Compose file
still provides its explicitly labeled local-demo default. This is not production
authentication.

Both Cosmos and the Go API use `/shelf` as the partition key. Aspire supplies
certificate trust for the emulator's HTTPS endpoint; TLS verification is not
disabled. Cosmos data is stored separately from Compose in the
`purr-and-prose-aspire-cosmos-data` named volume.

The Cosmos setup uses native Aspire APIs. The emulator declaration is active;
the Azure-only `AddAzureCosmosDB(...).WithAccessKeyAuthentication()` declaration
is commented out directly below it. To use Azure during local development,
comment out the emulator declaration and uncomment the Azure one. That requires
Azure credentials and can provision billable resources.

### Clear and seed command

On the **api** resource in the AppHost dashboard, choose **Clear and seed
database** and confirm the destructive prompt. It uses the same secret admin
parameter as the portal. It deletes the configured **books container**, then
recreates `/shelf` and restores exactly the four samples and seed marker; it
does not delete other containers, the database, or the account. Concurrent
requests in this API instance wait while reset runs. This is a destructive,
non-transactional maintenance action: if it fails midway, the command reports
failure and can be retried.

The underlying `POST /api/admin/database/reset` endpoint requires HTTP Basic
admin authentication **and** `X-Confirm-Reset: clear-and-seed`. It is not called
at startup or during normal shutdown. Do not invoke it on data you want to keep.

To exercise the existing browser tests against Aspire instead of Compose:

```sh
cd src/frontend
BASE_URL=http://localhost:<frontend-port> npm test
```

For diagnostics and a normal shutdown that preserves the named volume:

```sh
aspire logs api --apphost ./apphost.cs
aspire logs frontend --apphost ./apphost.cs
aspire stop --apphost ./apphost.cs
```

### Backend traces

The Go API exports OpenTelemetry traces to the Aspire dashboard using the
automatically injected OTLP settings; no additional AppHost wiring is needed.
Each API request has a route-named server span, with child HTTP client spans for
its Cosmos calls. Incoming W3C `traceparent`/`tracestate` and baggage propagate
through the API. Failed requests include their HTTP error status; `/healthz`
probes and their Cosmos reads are excluded to keep the demo readable.
Request/response bodies, passwords, and authorization headers are not captured.

Open **Traces** in the Aspire dashboard, select **api**, then reload the shelf
or add/remove a listing. The trace shows the API request and its Cosmos HTTP
call(s). Alternatively, from the repository root:

```sh
aspire otel traces api --apphost ./apphost.cs
```

Tracing is disabled when neither `OTEL_EXPORTER_OTLP_ENDPOINT` nor
`OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` is configured, so ordinary Compose needs no
collector. `OTEL_SDK_DISABLED=true` also disables it. For another collector, use
standard OTLP environment variables: `OTEL_SERVICE_NAME` (default `api`),
`OTEL_RESOURCE_ATTRIBUTES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, protocol (`grpc`,
the default, or `http/protobuf`), headers, and certificates. Signal-specific
`OTEL_EXPORTER_OTLP_TRACES_*` settings override the general exporter settings.
HTTPS certificate validation stays enabled. Pending spans are flushed after
active HTTP requests finish during normal shutdown; export errors are logged.

## Publish with Aspire to Docker Compose

The AppHost includes the native Docker Compose environment. It publishes the
Go API using `src/api/Dockerfile` and the built Vite site using the existing
Nginx Dockerfile, not a Vite development server. The Nginx runtime consumes
`WithReference` service discovery, so browser requests stay same-origin.
Health checks, API-before-frontend readiness, loopback port bindings, and
the telemetry dashboard are declared in the AppHost.

```sh
# Artifacts only: no Azure provisioning and no deployment.
aspire publish --apphost ./apphost.cs -o ./aspire-output
```

**Native `RunAsEmulator` applies only to local Aspire run mode.** Aspire
publishing generates Azure Cosmos/Key Vault Bicep infrastructure and Compose
services for the API, frontend, and telemetry dashboard. It does **not** package
the emulator into generated Compose. `WithAccessKeyAuthentication()` supplies
the account key required by the Go SDK through Aspire's native secret handling.
For an entirely local, account-free Compose demo, keep using
`docker compose -f src/compose.yaml up --build`.

Review `aspire-output/docker-compose.yaml`, `.env`, and the Bicep files before
deployment. `.env` contains blank image, password, and Azure-output placeholders;
`aspire publish` alone does not build images or resolve Azure credentials.
The output directory is git-ignored. Do not commit resolved environment files.

When you intentionally want to provision/deploy, configure Azure sign-in,
subscription, location, and resource group through Aspire's deployment
configuration and supply the secret parameter:

```sh
export Parameters__admin_password='choose-a-private-deployment-password'
aspire deploy --apphost ./apphost.cs --list-steps
# Creates billable Azure resources and starts the Compose deployment:
aspire deploy --apphost ./apphost.cs
```

The deployed frontend is `http://localhost:8088`, API `http://localhost:8086`,
and telemetry dashboard `http://localhost:18888`. Keep these loopback bindings
for a trusted demo; add real HTTPS/identity before exposing a server publicly.
The deployed standalone dashboard displays telemetry, not AppHost resource
commands or parameters. The authenticated reset endpoint remains available.
Do not run generated and standalone Compose concurrently on these same ports.

For a normal stop of an already prepared deployment, use the generated
environment-specific `.env.<environment>` file:

```sh
docker compose --env-file aspire-output/.env.<environment> -f aspire-output/docker-compose.yaml stop
```

This preserves data and does not delete Azure infrastructure. Do not use
`aspire destroy` as a normal shutdown; tearing down cloud data requires a
separate, explicit decision.

## Verification

```sh
docker compose -f src/compose.yaml ps
curl --fail http://localhost:8088/healthz
curl --fail http://localhost:8088/api/books
docker compose -f src/compose.yaml logs --tail=50 api cosmos
```

In the UI: open **Staff only**, enter the password, add a book with a cat seller
and price, return to **The bookshelf**, search for its title, then reload.
The listing must still be present. Sign back in and remove it; reload the shelf
and confirm it is gone. Wrong passwords and anonymous API writes return 401.

Focused automated checks (optional host tooling: Go 1.23+ and Node 22.12+):

```sh
cd src/api
go test -race ./...
go vet ./...
cd ../frontend
npm ci
npm run build
npm run test:config
npx playwright install chromium
npm test
```

`npm test` runs the discovery/proxy configuration tests and the browser suite.
The browser suite runs against the already-running Compose app by default. It uses real
Cosmos data for add/reload/remove, tests password rejection, and separately mocks
responses for deterministic loading, empty, error/retry, and markup-safety states.
It cleans up its test listing. Use `BASE_URL` / `ADMIN_PASSWORD` to match overrides.
Go handler tests use an in-memory test double and cover validation and database
failures. Configuration tests cover Aspire reference properties, complete-reference
requirements, Compose compatibility, HTTP opt-in, and managed port binding.
Tracing tests check context propagation, Cosmos child spans, error/route naming,
health-probe exclusion, and authenticated OTLP export/shutdown over gRPC and HTTP.
Docker API builds run these tests too.
`TestCosmosResetIntegration` is opt-in with `COSMOS_INTEGRATION_TEST=true` and
local emulator connection settings. It creates a uniquely named temporary
database, exercises reset twice through the authenticated API, verifies an
unrelated container survives, and removes only its own temporary database.

From the repository root, test actual database/API restart persistence:

```sh
node src/verify-persistence.mjs
```

This creates two temporary books, deletes one, restarts the three app containers,
waits for readiness, checks the remaining book's complete fields and durable
deletion, confirms existing data/sample counts are unchanged, and removes its
temporary listings. It leaves the stack running. Avoid editing the shelf
concurrently with this check. Docker must be on `PATH`; `BASE_URL` and
`ADMIN_PASSWORD` can override defaults.

## Stop without losing data

```sh
docker compose -f src/compose.yaml down
```

This stops/removes app containers and the network, **not** the database volume.
The next `up --build` reuses all saved data. `stop` / `start` also preserve it.

**Optional destructive reset: deletes every listing permanently**:

```sh
docker compose -f src/compose.yaml down --volumes
docker compose -f src/compose.yaml up --build
```

Only use this when you explicitly want a clean database and the original samples.
