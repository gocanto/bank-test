# PaveBank Fees API

Fees API coding challenge implemented with Go, Encore, and Temporal.

## Requirements

- Docker
- Go 1.26.4 when running commands directly on the host

Normal local development uses the Dockerized toolbox, so Encore and Temporal CLIs do not need to be installed on the host.

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

