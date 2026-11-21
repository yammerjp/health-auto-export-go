FROM golang:1.25-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w" \
    -o /out/health-auto-export .

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/health-auto-export /usr/local/bin/health-auto-export

ENV PORT=8080

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/health-auto-export"]
