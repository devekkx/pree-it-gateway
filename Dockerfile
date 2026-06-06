# ── Stage 1: deps ─────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS deps

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# ── Stage 2: build ────────────────────────────────────────────────────────────
FROM deps AS builder

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o /bin/gateway \
    ./cmd/gateway

# ── Stage 3: dev (air hot-reload) ─────────────────────────────────────────────
FROM deps AS dev

RUN go install github.com/air-verse/air@latest

WORKDIR /app
COPY . .

EXPOSE 8080 9090
CMD ["air", "-c", ".air.toml"]

# ── Stage 4: production ────────────────────────────────────────────────────────
FROM alpine:3.22.4 AS production

RUN apk add --no-cache ca-certificates tzdata wget

RUN addgroup -g 10001 -S app \
 && adduser  -u 10001 -S app -G app

COPY --from=builder /bin/gateway /gateway

USER app

EXPOSE 8080 9090

ENTRYPOINT ["/gateway"]