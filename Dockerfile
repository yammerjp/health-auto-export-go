FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build \
    -tags "sqlite_omit_load_extension" \
    -ldflags "-s -w" \
    -o /out/health-auto-export .

FROM gcr.io/distroless/base-debian12:nonroot

COPY --from=builder /out/health-auto-export /usr/local/bin/health-auto-export

ENV PORT=8080 \
    DB_PATH=/data/health.db

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/health-auto-export"]
