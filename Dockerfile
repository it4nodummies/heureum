# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:alpine AS builder
WORKDIR /app

# Cache deps separately from source
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o bin/server ./cmd/server/main.go

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.24
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/bin/server   ./server
COPY --from=builder /app/migrations/  ./migrations/
EXPOSE 8080
CMD ["./server"]
