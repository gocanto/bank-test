# Gocanto Bank Test

Gocanto Bank Test is a small Go backend for managing bank fee bills. It exposes
an HTTP API for creating bills, adding line items, closing bills, and reading the
latest bill summary.

The service is built with Encore, Temporal, and SQLite. Each one owns a
distinct concern, which keeps the domain logic free of framework and transport
details:

- **Encore exposes the local HTTP API and service runtime.** The `//encore:api`
  handlers in `fees/` are the whole delivery layer — Encore generates the
  program entrypoint, routes requests, and injects the Temporal client and
  SQLite store — so the handlers only translate HTTP calls into domain
  operations and never carry business rules themselves.
- **Temporal owns each bill's lifecycle and workflow state.** A bill is a
  long-lived entity with update rules (open → closed) and a delayed close when
  the billing period ends. Running that as a workflow makes the current state
  durable and recoverable: the workflow is the source of truth, and the API
  merely sends updates (`add line item`, `close`) and queries against it rather
  than mutating state in place.
- **SQLite stores append-only bill snapshots for reads and recovery-friendly
  summaries after workflow changes.** After every successful workflow change the
  API writes a snapshot, so reads can be served locally and still return a last
  known bill even when Temporal is unreachable. Because the table is
  append-only, it doubles as a history of observed states without extra
  infrastructure or migrations.

## What It Does

A bill starts in the `open` state for a billing period. While it is open, callers
can add fee line items. Each line item has a stable ID, description, amount, and
currency.

The service keeps totals grouped by currency. Bills can be closed manually
through the API, or by the Temporal workflow when the billing period ends. Once a
bill is closed, new line items are rejected.

Current domain rules:

- bill IDs and line item IDs are required
- line item descriptions are required
- line item IDs must be unique per bill
- amounts must be positive
- supported currencies are `USD` and `GEL`
- closed bills cannot be changed

## Requirements

- Docker
- Go only when running commands directly on the host

Normal local development uses the Dockerized toolbox. The toolbox installs the
Encore and Temporal CLIs, and its Go version is resolved from
`ghcr.io/oullin/go-fmt:v0.4.2-full`.

## Why This Shape

The roles of Encore, Temporal, and SQLite are covered above. Beyond those,
Docker keeps the local setup repeatable: the Dockerized toolbox pins the Encore
and Temporal CLIs and the Go version, so you should not need to install any of
them directly on your machine to run the service.

## Project Layout

Encore dictates the top-level shape: a service is a package with `//encore:service`
and `//encore:api` annotations and its `config.cue` alongside it, and Encore generates
the program entrypoint (there is no `cmd/main.go`). The service package holds only what
Encore requires; everything framework-free lives under `internal/`, split into generic
reusable packages, the service's bounded context, and a shared platform kernel.

```
fees/                         # Encore service — HTTP transport / delivery layer
  service.go  api.go          # //encore:service and //encore:api handlers
  config.go  config.cue       # Encore configuration (must live in the service pkg)
  errors.go                   # fees-specific error classification (domain → Fault)
internal/
  bootstrap/                  # service wiring: open SQLite, dial Temporal, start worker
  errorsx/                    # generic error → Encore code / HTTP status mapping
  response/                   # generic Response[T] envelope
  database/                   # generic snapshot persistence glue (Persist / Stored)
  temporal/                   # generic Temporal workflow query helper
  fees/                       # "fees" bounded context (no framework imports)
    domain/                   # pure bill/money domain, validation, and state machine
    billstore/                # append-only SQLite bill-snapshot store
    workflows/                # Temporal bill lifecycle workflow
  platform/                   # shared kernel, reusable by any future service
    sqlite/                   # SQLite open/migrate bootstrap
```

Dependencies point inward: `domain` imports nothing from the rest of the tree, the
`billstore` and `workflows` packages depend only on `domain`, and the generic
`bootstrap`, `errorsx`, `response`, `database`, and `temporal` packages carry no
bounded-context knowledge. The `fees` service package is a thin adapter that classifies
its own errors and delegates wiring to `bootstrap`. The `internal/` prefix keeps every
non-service package module-private, so nothing here can be imported from outside the
module.

A second service (say `accounts`) would be added as another top-level Encore package
with its own `internal/accounts/…` context, reusing `internal/bootstrap`,
`internal/errorsx`, `internal/response`, `internal/database`, `internal/temporal`, and
`internal/platform/…` unchanged.

## How It Works

The public API is under `http://localhost:4000/v1/bank`.

- `POST /v1/bank/bills` validates the request, starts a Temporal bill workflow,
  and saves the initial bill snapshot.
- `POST /v1/bank/bills/:billID/line-items` sends a workflow update, waits for
  the updated bill summary, and saves a new snapshot.
- `POST /v1/bank/bills/:billID/close` sends a workflow close update and saves
  the closed bill snapshot.
- `GET /v1/bank/bills/:billID` returns a closed bill from SQLite when available.
  Otherwise, it queries Temporal and refreshes the local snapshot.

API responses are wrapped in a `data` object:

```json
{
  "data": {
    "id": "bill-001",
    "state": "open"
  }
}
```

## Examples

The examples below are split into two parts: running the **server** (the
Temporal dev server plus the Encore API) and acting as a **consumer** (calling
the API over HTTP).

### Server

The service has two long-running processes. Start each in its own terminal.

**1. Start Temporal** (terminal 1):

```bash
make temporal
```

Temporal listens on `localhost:7233`, and the Temporal UI is available at
`http://localhost:8233`.

**2. Start the Encore API** (terminal 2):

```bash
make run
```

The API listens on `http://localhost:4000`.

Local SQLite state is stored at `storage/database/gocanto.sqlite3`. The
directory is created automatically and ignored by git.

### Consumers

With the server running, a consumer drives a bill through its lifecycle with the
four endpoints below. Every response wraps the bill under a `data` object.

**1. Create a bill** — starts the Temporal workflow and saves the first snapshot:

```bash
curl -X POST http://localhost:4000/v1/bank/bills \
  -H 'content-type: application/json' \
  -d '{"bill_id":"bill-001","period_start":"2026-06-01T00:00:00Z","period_end":"2026-07-01T00:00:00Z"}'
```

**2. Add a line item** — allowed only while the bill is open:

```bash
curl -X POST http://localhost:4000/v1/bank/bills/bill-001/line-items \
  -H 'content-type: application/json' \
  -d '{"id":"li-001","description":"Card processing fee","amount":{"amount":1250,"currency":"USD"}}'
```

**3. Close the bill** — after this, new line items are rejected:

```bash
curl -X POST http://localhost:4000/v1/bank/bills/bill-001/close
```

**4. Read a summary** — returns the current bill state:

```bash
curl http://localhost:4000/v1/bank/bills/bill-001
```

A closed bill returns this shape:

```json
{
  "data": {
    "id": "bill-001",
    "state": "closed",
    "period_start": "2026-06-01T00:00:00Z",
    "period_end": "2026-07-01T00:00:00Z",
    "line_items": [
      {
        "id": "li-001",
        "description": "Card processing fee",
        "amount": {
          "amount": 1250,
          "currency": "USD"
        },
        "created_at": "2026-06-01T00:00:00Z"
      }
    ],
    "totals": [
      {
        "amount": 1250,
        "currency": "USD"
      }
    ],
    "created_at": "2026-06-01T00:00:00Z",
    "closed_at": "2026-06-01T00:00:00Z"
  }
}
```

## Checks

```bash
make lint
make test
make test-e2e
make vet
make check
```

- `make lint` checks formatting with the Dockerized `go-fmt` tooling.
- `make test` runs unit and Temporal workflow tests.
- `make test-e2e` runs Testcontainers-backed service and Temporal checks.
- `make vet` runs `go vet`.
- `make check` runs `test`, `test-e2e`, and `vet`.
