# Gocanto Bank Test

Bank test coding challenge implemented with Go, Encore, and Temporal.

Temporal owns the bill lifecycle workflow. SQLite stores durable bill snapshots
for API reads and recovery-friendly summaries after successful workflow changes.

## Requirements

- Docker
- Go version from `ghcr.io/oullin/go-fmt:v0.4.2-full` when running commands directly on the host

Normal local development uses the Dockerized toolbox. Its Go version is resolved
from `go-fmt`, so Encore and Temporal CLIs do not need to be installed on the
host.

## Run

Start Temporal:

```bash
make temporal
```

In another terminal, run the Encore service:

```bash
make run
```

The API listens on `http://localhost:4000`.

Local SQLite state is stored at `storage/database/gocanto.sqlite3`. The
directory is created automatically and ignored by git.

## API

Create a bill:

```bash
curl -X POST http://localhost:4000/bills \
  -H 'content-type: application/json' \
  -d '{"bill_id":"bill-001","period_start":"2026-06-01T00:00:00Z","period_end":"2026-07-01T00:00:00Z"}'
```

Add line items:

```bash
curl -X POST http://localhost:4000/bills/bill-001/line-items \
  -H 'content-type: application/json' \
  -d '{"id":"li-001","description":"Card processing fee","amount":{"amount":1250,"currency":"USD"}}'
```

Close the bill:

```bash
curl -X POST http://localhost:4000/bills/bill-001/close
```

Read a summary:

```bash
curl http://localhost:4000/bills/bill-001
```

## Checks

```bash
make check
```

`make check` runs unit tests, Temporal workflow tests, Testcontainers end-to-end tests, and `go vet`.
