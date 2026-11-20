# ==========================================
# Stage 1: Build Vue 3 Frontend
# ==========================================
FROM --platform=$BUILDPLATFORM node:24-alpine AS frontend-builder
WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Build Go Backend
# ==========================================
FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS backend-builder
ARG TARGETARCH
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o /hephaestus ./cmd/server

# ==========================================
# Stage 3: Minimal Production Image
# ==========================================
FROM postgres:16-alpine AS postgres16-client

FROM alpine:3.24.4

RUN apk upgrade --no-cache && \
    apk add --no-cache ca-certificates tzdata postgresql-client postgresql17-client postgresql18-client mariadb-client openssh-client iputils curl net-snmp-tools bind-tools tar gzip socat bash

WORKDIR /app

# Copy binary & static web assets
COPY --from=backend-builder /hephaestus /app/hephaestus
COPY --from=postgres16-client /usr/local/bin/pg_dump /usr/local/bin/pg_dump16
COPY --from=frontend-builder /app/web/dist /app/web/dist
COPY internal/database/migrations /app/internal/database/migrations

# Create storage directories
RUN mkdir -p /app/data/mibs /app/logs /app/backups /opt/backups

ENV PORT=5000 \
    APP_ENV=production \
    LOGS_DIR=/app/logs \
    DATA_DIR=/app/data

EXPOSE 5000

ENTRYPOINT ["/app/hephaestus"]
