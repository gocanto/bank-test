# Gocanto Bank Test

Gocanto Bank Test is a small Go backend for managing bank fee bills. It exposes
an HTTP API for creating bills, adding line items, closing bills, and reading the
latest bill summary.

The service is built with Encore, Temporal, and SQLite:

- Encore exposes the local HTTP API and service runtime.
- Temporal owns each bill's lifecycle and workflow state.
- SQLite stores append-only bill snapshots for reads and recovery-friendly
  summaries after workflow changes.

## What It Does

A bill starts in the `open` state for a billing period. While it is open, callers
can add fee line items. Each line item has a stable ID, description, amount, and
currency.

The service keeps totals grouped by currency. Bills can be closed manually
through the API, or by the Temporal workflow when the billing period ends. Once a
bill is closed, new line items are rejected.

Current domain rules:

- bill IDs and line item IDs are required
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

Temporal is used because a bill has lifecycle state, update rules, and delayed
close behavior. Keeping that state in a workflow makes create, update, query,
and close operations explicit and recoverable.

SQLite is used for pragmatic local persistence. The API writes a snapshot after
successful workflow changes, then reads the latest snapshot when it can. The
snapshot table is append-only, which keeps a history of observed bill states
without adding more infrastructure.

Docker keeps the local setup repeatable. You should not need to install Encore
or Temporal directly on your machine to run the service.

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

## Run

Start Temporal:

```bash
make temporal
```

Temporal listens on `localhost:7233`, and the Temporal UI is available at
`http://localhost:8233`.

In another terminal, start the Encore API:

```bash
make run
```

The API listens on `http://localhost:4000`.

Local SQLite state is stored at `storage/database/gocanto.sqlite3`. The
directory is created automatically and ignored by git.

## Use The API

Create a bill:

```bash
curl -X POST http://localhost:4000/v1/bank/bills \
  -H 'content-type: application/json' \
  -d '{"bill_id":"bill-001","period_start":"2026-06-01T00:00:00Z","period_end":"2026-07-01T00:00:00Z"}'
```

Add line items:

```bash
curl -X POST http://localhost:4000/v1/bank/bills/bill-001/line-items \
  -H 'content-type: application/json' \
  -d '{"id":"li-001","description":"Card processing fee","amount":{"amount":1250,"currency":"USD"}}'
```

Close the bill:

```bash
curl -X POST http://localhost:4000/v1/bank/bills/bill-001/close
```

Read a summary:

```bash
curl http://localhost:4000/v1/bank/bills/bill-001
```

Responses include the bill under `data`. A closed bill looks like this shape:

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
