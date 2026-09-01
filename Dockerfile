# ==========================================
# Stage 1: Build Vue 3 Frontend
# ==========================================
FROM node:20-alpine AS frontend-builder
WORKDIR /app/web

COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ==========================================
# Stage 2: Build Go Backend
# ==========================================
FROM golang:1.22-alpine AS backend-builder
WORKDIR /app

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /hephaestus ./cmd/server

# ==========================================
# Stage 3: Minimal Production Image
# ==========================================
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata postgresql16-client mysql-client openssh-client iputils

WORKDIR /app

# Copy binary & static web assets
COPY --from=backend-builder /hephaestus /app/hephaestus
COPY --from=frontend-builder /app/web/dist /app/web/dist
COPY internal/database/migrations /app/internal/database/migrations

# Create storage directories
RUN mkdir -p /app/data/mibs /app/logs /app/backups

ENV PORT=5000 \
    APP_ENV=production \
    LOGS_DIR=/app/logs \
    DATA_DIR=/app/data

EXPOSE 5000

ENTRYPOINT ["/app/hephaestus"]
