# health-auto-export-go

A small self-hosted server that ingests data exported by the iOS app
[Health Auto Export](https://www.healthyapps.dev/health-auto-export) and
stores it in SQLite. Single Go binary, no external services.

The server speaks the same JSON contract as the iOS app's REST export and
manual export, so it works as a drop-in receiver alongside dashboards
(Grafana Infinity plugin etc.).

## Features

- Single binary (CGO for `mattn/go-sqlite3`); no external database service required.
- File-based SQLite database; backups are a single file copy.
- CLI subcommand to import a manual-export JSON or ZIP into the same database.
- Bearer-style API tokens (`api-key` header) for read and write paths.
- Built-in `/dashboard` HTML view for sanity-checking ingested data.
- Multi-arch Docker image published to GitHub Container Registry.

## Quick start

```sh
docker run --rm -p 8080:8080 \
  -e READ_TOKEN=sk-$(openssl rand -hex 16) \
  -e WRITE_TOKEN=sk-$(openssl rand -hex 16) \
  -v "$PWD/data:/data" \
  ghcr.io/yammerjp/health-auto-export-go:latest
```

Configure the iOS app to POST to `http://<host>:8080/api/data` with the
`api-key` header set to your `WRITE_TOKEN`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Listen port. |
| `DB_PATH` | `health.db` (image: `/data/health.db`) | SQLite file path. |
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

Requires Go 1.25+ and a C toolchain (for `mattn/go-sqlite3`).

```sh
CGO_ENABLED=1 go build -o health-auto-export .
go test ./...
```

## Kubernetes

A minimal Deployment with a `local-path` PVC for the SQLite file:

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
            - { name: DB_PATH, value: /data/health.db }
            - name: READ_TOKEN
              valueFrom: { secretKeyRef: { name: health-auto-export, key: READ_TOKEN } }
            - name: WRITE_TOKEN
              valueFrom: { secretKeyRef: { name: health-auto-export, key: WRITE_TOKEN } }
          ports: [{ containerPort: 8080 }]
          volumeMounts: [{ name: data, mountPath: /data }]
      volumes:
        - name: data
          persistentVolumeClaim: { claimName: health-auto-export-data }
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata: { name: health-auto-export-data }
spec:
  accessModes: [ReadWriteOnce]
  resources: { requests: { storage: 1Gi } }
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
