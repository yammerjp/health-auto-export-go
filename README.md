# health-auto-export-go

A small self-hosted server that ingests data exported by the iOS app
[Health Auto Export](https://www.healthyapps.dev/health-auto-export) and
stores it in PostgreSQL. Single static Go binary.

> Since v1.0.0 the storage is PostgreSQL (earlier versions used SQLite).
> To move an existing `health.db`, copy the six tables (`metrics`,
> `blood_pressure`, `heart_rate`, `sleep_analysis`, `workouts`,
> `workout_routes`) into the tables this server creates on first start;
> the column names are unchanged.

The server speaks the same JSON contract as the iOS app's REST export and
manual export, so it works as a drop-in receiver alongside dashboards
(Grafana Infinity plugin etc.).

## Features

- Single static binary (pure Go, `jackc/pgx`); tables are created on first start.
- PostgreSQL storage; each request is written in one transaction.
- CLI subcommand to import a manual-export JSON or ZIP into the same database.
- Bearer-style API tokens (`api-key` header) for read and write paths.
- Built-in `/dashboard` HTML view for sanity-checking ingested data.
- Multi-arch Docker image published to GitHub Container Registry.

## Quick start

```sh
docker network create hae
docker run -d --name hae-pg --network hae -e POSTGRES_PASSWORD=postgres \
  -v "$PWD/pgdata:/var/lib/postgresql" postgres:18-alpine
docker run --rm -p 8080:8080 --network hae \
  -e DATABASE_URL='postgres://postgres:postgres@hae-pg:5432/postgres?sslmode=disable' \
  -e READ_TOKEN=sk-$(openssl rand -hex 16) \
  -e WRITE_TOKEN=sk-$(openssl rand -hex 16) \
  ghcr.io/yammerjp/health-auto-export-go:latest
```

Configure the iOS app to POST to `http://<host>:8080/api/data` with the
`api-key` header set to your `WRITE_TOKEN`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port. |
| `DATABASE_URL` | (required) | PostgreSQL URL or DSN, e.g. `postgres://user:pass@host:5432/db?sslmode=verify-full&sslrootcert=/etc/pg-ca/ca.crt`. The connection is retried for 60 s at startup. |
| `READ_TOKEN` | (empty) | Required for read endpoints. Must start with `sk-`. |
| `WRITE_TOKEN` | (empty) | Required for `POST /api/data`. Must start with `sk-`. |

## API

| Method | Path | Auth | Purpose |
|---|---|---|---|
| `POST` | `/api/data` | `WRITE_TOKEN` | Ingest payload from the iOS app. |
| `GET` | `/api/metrics/{type}` | `READ_TOKEN` | Query metrics. Supports `from`, `to`, `include`, `exclude`. |
| `GET` | `/api/workouts` | `READ_TOKEN` | List workouts. Supports `startDate`, `endDate`, `include`, `exclude`. |
| `GET` | `/api/workouts/{id}` | `READ_TOKEN` | Workout detail with heart-rate samples and GPS route. |
| `GET` | `/dashboard` | (token entered in the page) | HTML dashboard. |
| `GET` | `/` | none | Health check. |

Date filters accept ISO 8601 (`2026-01-21`, `2026-01-21T00:00:00Z`),
`YYYY/MM/DD`, `YYYY-MM-DD HH:MM:SS`, or Unix timestamps in seconds or
milliseconds.

## Importing a manual export

```sh
./health-auto-export import path/to/HealthAutoExport_YYYYMMDDHHMMSS.zip
```

JSON files are accepted as well. Existing rows are upserted (per-metric on
`(date, source)`, per-workout on `workout_id`).

## Build from source

Requires Go 1.25+. Tests need a PostgreSQL to create throwaway schemas in
(they are skipped when `TEST_DATABASE_URL` is unset).

```sh
go build -o health-auto-export .
docker run -d --rm --name hae-test-pg -e POSTGRES_PASSWORD=postgres -p 127.0.0.1:55432:5432 postgres:18-alpine
TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable' go test ./...
```

## Kubernetes

A minimal Deployment (the database is assumed to exist elsewhere; the URL is
read from a Secret):

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: health-auto-export
spec:
  replicas: 1
  selector:
    matchLabels: { app: health-auto-export }
  strategy: { type: Recreate }
  template:
    metadata:
      labels: { app: health-auto-export }
    spec:
      containers:
        - name: health-auto-export
          image: ghcr.io/yammerjp/health-auto-export-go:latest
          env:
            - { name: PORT, value: "8080" }
            - name: DATABASE_URL
              valueFrom: { secretKeyRef: { name: health-auto-export-db, key: uri } }
            - name: READ_TOKEN
              valueFrom: { secretKeyRef: { name: health-auto-export, key: READ_TOKEN } }
            - name: WRITE_TOKEN
              valueFrom: { secretKeyRef: { name: health-auto-export, key: WRITE_TOKEN } }
          ports: [{ containerPort: 8080 }]
```

Expose via your Ingress controller of choice; pair with HTTP basic auth or
similar at the edge if the deployment is reachable from the public
internet.

## Acknowledgements

- iOS app: [Health Auto Export — JSON, CSV](https://apps.apple.com/us/app/health-auto-export-json-csv/id1115567069) by [HealthyApps](https://www.healthyapps.dev/).
- The JSON contract is documented in the [Lybron/health-auto-export wiki](https://github.com/Lybron/health-auto-export/wiki/API-Export---JSON-Format).

This project is an independent reimplementation against the iOS app's
public export format. It is not affiliated with HealthyApps and does not
reuse any of their server source.

## License

MIT — see [LICENSE](./LICENSE).
