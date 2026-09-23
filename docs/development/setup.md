# Local Development Setup Guide

This guide walks you through setting up a complete IvyTicketing local development environment on Linux or macOS.

---

## 1. Prerequisites

Ensure you have the following installed on your host system:
- **Go**: Version 1.22 or higher
- **Node.js**: Version 20 LTS or higher with `pnpm` (`corepack enable pnpm`)
- **Docker & Docker Compose**: For local PostgreSQL 16 and Redis 7 instances
- **Goose**: Migration CLI (`go install github.com/pressly/goose/v3/cmd/goose@latest`)

---

## 2. Infrastructure Containers

Start the local PostgreSQL and Redis containers:

```bash
docker compose up -d postgres redis
```

Verify services are healthy:
```bash
docker ps
# Expected:
# ivyticketing-pg (Port 5432)
# ivyticketing-redis (Port 6379)
```

---

## 3. Database Migration and Seeding

Run Goose migrations against your local database:

```bash
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ivyticketing?sslmode=disable"
goose -dir database/migrations postgres "$DATABASE_URL" up
```

---

## 4. Backend Configuration (`.env`)

Create `.env` in the repository root or export variables:

```ini
APP_ENV=local
APP_NAME=ivyticketing
API_PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/ivyticketing?sslmode=disable
REDIS_URL=redis://localhost:6379/0
WEB_ORIGIN=http://localhost:4321
JWT_SECRET=super-secret-local-jwt-signing-key-minimum-32-chars
TICKET_QR_SECRET=super-secret-local-ticket-qr-signing-key-32
ACCESS_TOKEN_TTL=15m
REFRESH_TOKEN_TTL=168h
ORDER_EXPIRATION=15m
QUEUE_RELEASE_INTERVAL=10s
QUEUE_CHECKOUT_WINDOW=5m
QUEUE_DEFAULT_RELEASE_RATE=100
EMAIL_DRIVER=log
```

---

## 5. Running the Application

### 1. Start the API Server
```bash
cd services/api
go run cmd/api/main.go
```
The REST API server will listen on `http://localhost:8080`.

### 2. Start the Background Worker Daemon
In a separate terminal:
```bash
cd services/api
go run cmd/worker/main.go
```

### 3. Start the Astro Web Portal
In a third terminal:
```bash
cd apps/web
pnpm install
pnpm dev
```
Access the web frontend at `http://localhost:4321`.

### 4. Start the Svelte Scanner PWA
```bash
cd apps/scanner
pnpm install
pnpm dev
```
Access the mobile scanner interface at `http://localhost:5173`.
