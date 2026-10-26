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
- Port 8088 free. Go/Node are **not** required to run the containers.
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
| `ADMIN_PASSWORD` | `alligator123`; local-only demo credential, required by API |
| `COSMOS_KEY` | Public base64 local-demo placeholder in Compose; required by SDK |
| `COSMOS_ENDPOINT` | API: `http://cosmos:8081` |
| `COSMOS_ALLOW_HTTP` | API: `true` only for the local HTTP emulator |
| `COSMOS_DATABASE` | API: `catbooks` |
| `COSMOS_CONTAINER` | API: `books` |
| `HTTP_ADDR` | API listen address: `:8080` |
| `API_UPSTREAM` | Frontend Nginx: `http://api:8080`; Vite dev proxy also accepts it |

The first three can be overridden in the shell when running Compose. Other
values are ordinary service environment entries in `compose.yaml`; change them
there or provide a Compose override when needed. For example:

```sh
FRONTEND_PORT=8090 ADMIN_PASSWORD='another-local-password' docker compose -f src/compose.yaml up --build
```

The emulator **does not enforce database authentication**. Its key placeholder
is not a real secret. Database and API ports are deliberately not published.
Admin checks happen on the server for every write, using HTTP Basic credentials
held only in browser memory (no local storage or cookies). Reload or lock the
portal to discard them. HTTP and a shared public password are suitable only for
a trusted local demo. Do not expose this stack to the internet. Real deployment
would require HTTPS, real identity/access controls, and securely supplied
database credentials; none of those services are provisioned here.

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
npx playwright install chromium
npm test
```

The browser suite runs against the already-running Compose app. It uses real
Cosmos data for add/reload/remove, tests password rejection, and separately mocks
responses for deterministic loading, empty, error/retry, and markup-safety states.
It cleans up its test listing. Use `BASE_URL` / `ADMIN_PASSWORD` to match overrides.
Go handler tests use an in-memory test double and cover validation and database
failures; Docker API builds run these tests too.

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
