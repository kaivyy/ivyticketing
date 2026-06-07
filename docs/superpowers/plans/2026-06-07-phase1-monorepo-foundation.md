# Phase 1: Monorepo Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a thin-but-live monorepo foundation proving the chain `Astro web → Go API → Postgres + Redis` works locally.

**Architecture:** Go modular monolith (`services/api`) with Chi router, sqlc for type-safe queries, goose for migrations. Astro frontend (`apps/web`) calls the API's readiness endpoint and renders dependency health. Postgres + Redis run natively via Homebrew. No Docker, no business logic — only a `system` module with `/healthz` and `/readyz`.

**Tech Stack:** Go 1.25, Chi v5, pgx v5, go-redis v9, sqlc, goose, Astro, TypeScript, Tailwind CSS, pnpm.

**Reference spec:** `docs/superpowers/specs/2026-06-07-phase1-monorepo-foundation-design.md`

**Environment facts (verified):**
- Homebrew 5.1.14; `postgresql@16` is keg-only (binaries not on PATH by default — use full path `$(brew --prefix postgresql@16)/bin`).
- Go installs binaries to `/Users/kaivy/go/bin` (GOPATH=`/Users/kaivy/go`). Ensure this is on PATH for `goose`/`sqlc`.
- Node 25.2.1, pnpm 10.14.0 present.
- Git repo already initialized; `.gitignore` already committed with `.env`, `node_modules/`, `dist/`, `/services/api/bin/`.

---

## File Structure

```txt
ivyticketing/
├── .env.example                         # root env template (api)
├── Makefile                             # dev workflow targets
├── README.md                            # setup-from-zero guide
├── scripts/dev/
│   ├── setup-local.sh                   # install+start pg/redis, createdb, tools, migrate, pnpm i
│   └── start-local.sh                   # run api + web in parallel
├── database/
│   ├── migrations/
│   │   └── 00001_create_schema_health.sql   # goose migration
│   └── queries/
│       └── system.sql                   # sqlc query (health ping)
└── services/api/
    ├── go.mod
    ├── sqlc.yaml
    ├── .env.example
    ├── cmd/api/main.go                  # entrypoint
    └── internal/
        ├── app/
        │   ├── config.go                # env parsing + validation
        │   └── server.go                # router assembly, CORS, start
        ├── platform/
        │   ├── database/postgres.go     # pgxpool connect + ping
        │   ├── redis/redis.go           # go-redis connect + ping
        │   ├── logger/logger.go         # slog setup
        │   └── middleware/requestid.go  # request id middleware
        ├── db/                          # sqlc-generated (do not edit)
        └── modules/system/
            ├── handler.go               # /healthz + /readyz
            ├── handler_test.go          # unit tests
            └── routes.go                # route registration
apps/web/
├── package.json
├── astro.config.mjs
├── tsconfig.json
├── tailwind.config.mjs
├── .env.example
└── src/
    ├── layouts/PublicLayout.astro
    ├── lib/api.ts                       # fetch wrapper to API
    ├── pages/index.astro                # render readiness status
    └── styles/global.css
```

---

## Task 1: Go module + entrypoint skeleton

**Files:**
- Create: `services/api/go.mod`
- Create: `services/api/cmd/api/main.go`

- [ ] **Step 1: Initialize Go module**

Run from repo root:
```bash
cd services/api && go mod init github.com/varin/ivyticketing/services/api && cd ../..
```
Expected: creates `services/api/go.mod` with `go 1.25` (or similar).

- [ ] **Step 2: Write a minimal entrypoint that compiles**

Create `services/api/cmd/api/main.go`:
```go
package main

import "fmt"

func main() {
	fmt.Println("ivyticketing api: boot placeholder")
}
```

- [ ] **Step 3: Verify it builds and runs**

Run:
```bash
cd services/api && go run ./cmd/api && cd ../..
```
Expected: prints `ivyticketing api: boot placeholder`.

- [ ] **Step 4: Commit**

```bash
git add services/api/go.mod services/api/cmd/api/main.go
git commit -m "feat(api): initialize go module and entrypoint skeleton"
```

---

## Task 2: Config loader with validation

**Files:**
- Create: `services/api/internal/app/config.go`
- Test: `services/api/internal/app/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `services/api/internal/app/config_test.go`:
```go
package app

import "testing"

func TestLoadConfig_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/ivyticketing?sslmode=disable")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("API_PORT", "")
	t.Setenv("APP_ENV", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIPort != "8080" {
		t.Errorf("APIPort = %q, want 8080", cfg.APIPort)
	}
	if cfg.AppEnv != "local" {
		t.Errorf("AppEnv = %q, want local", cfg.AppEnv)
	}
}

func TestLoadConfig_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "redis://localhost:6379")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: FAIL — `undefined: LoadConfig`.

- [ ] **Step 3: Write minimal implementation**

Create `services/api/internal/app/config.go`:
```go
package app

import (
	"fmt"
	"os"
)

type Config struct {
	AppEnv      string
	AppName     string
	APIPort     string
	DatabaseURL string
	RedisURL    string
	WebOrigin   string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppName:     getEnv("APP_NAME", "ivyticketing"),
		APIPort:     getEnv("API_PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		WebOrigin:   getEnv("WEB_ORIGIN", "http://localhost:4321"),
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("config: REDIS_URL is required")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/app/ -run TestLoadConfig -v; cd ../..
```
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/app/config.go services/api/internal/app/config_test.go
git commit -m "feat(api): add config loader with required-var validation"
```

---

## Task 3: Logger and request-id middleware

**Files:**
- Create: `services/api/internal/platform/logger/logger.go`
- Create: `services/api/internal/platform/middleware/requestid.go`
- Test: `services/api/internal/platform/middleware/requestid_test.go`

- [ ] **Step 1: Add Chi dependency**

Run:
```bash
cd services/api && go get github.com/go-chi/chi/v5@latest && cd ../..
```
Expected: adds chi to `go.mod`.

- [ ] **Step 2: Write the failing test for request-id middleware**

Create `services/api/internal/platform/middleware/requestid_test.go`:
```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_SetsHeader(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got == "" {
		t.Fatal("expected X-Request-Id header to be set, got empty")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/platform/middleware/ -v; cd ../..
```
Expected: FAIL — `undefined: RequestID`.

- [ ] **Step 4: Implement middleware**

Create `services/api/internal/platform/middleware/requestid.go`:
```go
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const HeaderRequestID = "X-Request-Id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if id == "" {
			id = newID()
		}
		w.Header().Set(HeaderRequestID, id)
		next.ServeHTTP(w, r)
	})
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req-unknown"
	}
	return "req_" + hex.EncodeToString(b)
}
```

- [ ] **Step 5: Implement logger**

Create `services/api/internal/platform/logger/logger.go`:
```go
package logger

import (
	"log/slog"
	"os"
)

func New(appEnv string) *slog.Logger {
	var handler slog.Handler
	if appEnv == "local" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	return slog.New(handler)
}
```

- [ ] **Step 6: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/platform/... -v; cd ../..
```
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add services/api/internal/platform services/api/go.mod services/api/go.sum
git commit -m "feat(api): add slog logger and request-id middleware"
```

---

## Task 4: System module — health and readiness handlers

**Files:**
- Create: `services/api/internal/modules/system/handler.go`
- Create: `services/api/internal/modules/system/routes.go`
- Test: `services/api/internal/modules/system/handler_test.go`

The handler depends on an injected `Checker` interface so tests can fake DB/Redis state without real connections (spec: "test readiness with fake checker").

- [ ] **Step 1: Write the failing tests**

Create `services/api/internal/modules/system/handler_test.go`:
```go
package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeChecker struct{ err error }

func (f fakeChecker) Ping(context.Context) error { return f.err }

func TestHealthz(t *testing.T) {
	h := NewHandler(nil, nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h.Healthz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestReadyz_AllHealthy(t *testing.T) {
	h := NewHandler(fakeChecker{nil}, fakeChecker{nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readyz(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body readyResponse
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "ready" || body.Checks["postgres"] != "ok" || body.Checks["redis"] != "ok" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestReadyz_PostgresDown(t *testing.T) {
	h := NewHandler(fakeChecker{errors.New("conn refused")}, fakeChecker{nil})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)

	h.Readyz(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body readyResponse
	json.Unmarshal(rec.Body.Bytes(), &body)
	if body.Status != "not_ready" || body.Checks["postgres"] != "down" || body.Checks["redis"] != "ok" {
		t.Errorf("unexpected body: %+v", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:
```bash
cd services/api && go test ./internal/modules/system/ -v; cd ../..
```
Expected: FAIL — `undefined: NewHandler`, `undefined: readyResponse`.

- [ ] **Step 3: Implement the handler**

Create `services/api/internal/modules/system/handler.go`:
```go
package system

import (
	"context"
	"encoding/json"
	"net/http"
)

type Checker interface {
	Ping(ctx context.Context) error
}

type Handler struct {
	postgres Checker
	redis    Checker
}

func NewHandler(postgres, redis Checker) *Handler {
	return &Handler{postgres: postgres, redis: redis}
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	checks := map[string]string{
		"postgres": pingStatus(ctx, h.postgres),
		"redis":    pingStatus(ctx, h.redis),
	}
	status := http.StatusOK
	state := "ready"
	for _, v := range checks {
		if v != "ok" {
			status = http.StatusServiceUnavailable
			state = "not_ready"
			break
		}
	}
	writeJSON(w, status, readyResponse{Status: state, Checks: checks})
}

func pingStatus(ctx context.Context, c Checker) string {
	if c == nil {
		return "down"
	}
	if err := c.Ping(ctx); err != nil {
		return "down"
	}
	return "ok"
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run:
```bash
cd services/api && go test ./internal/modules/system/ -v; cd ../..
```
Expected: PASS (all three tests).

- [ ] **Step 5: Add route registration**

Create `services/api/internal/modules/system/routes.go`:
```go
package system

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/healthz", h.Healthz)
	r.Get("/readyz", h.Readyz)
}
```

- [ ] **Step 6: Commit**

```bash
git add services/api/internal/modules/system
git commit -m "feat(api): add system module with healthz and readyz handlers"
```

---

## Task 5: Postgres and Redis platform adapters

**Files:**
- Create: `services/api/internal/platform/database/postgres.go`
- Create: `services/api/internal/platform/redis/redis.go`

These provide concrete `Checker` implementations. No unit test (they wrap external clients); they are exercised by the manual smoke test in Task 9.

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd services/api && go get github.com/jackc/pgx/v5/pgxpool@latest && go get github.com/redis/go-redis/v9@latest && cd ../..
```
Expected: adds pgx and go-redis to `go.mod`.

- [ ] **Step 2: Implement Postgres adapter**

Create `services/api/internal/platform/database/postgres.go`:
```go
package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func Connect(ctx context.Context, url string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	return p.Pool.Ping(ctx)
}

func (p *Postgres) Close() {
	p.Pool.Close()
}
```

- [ ] **Step 3: Implement Redis adapter**

Create `services/api/internal/platform/redis/redis.go`:
```go
package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

func Connect(ctx context.Context, url string) (*Redis, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &Redis{Client: redis.NewClient(opt)}, nil
}

func (r *Redis) Ping(ctx context.Context) error {
	return r.Client.Ping(ctx).Err()
}

func (r *Redis) Close() error {
	return r.Client.Close()
}
```

- [ ] **Step 4: Verify it builds**

Run:
```bash
cd services/api && go build ./... && cd ../..
```
Expected: builds with no errors.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/platform/database services/api/internal/platform/redis services/api/go.mod services/api/go.sum
git commit -m "feat(api): add postgres and redis platform adapters"
```

---

## Task 6: Server assembly and wiring in main

**Files:**
- Create: `services/api/internal/app/server.go`
- Modify: `services/api/cmd/api/main.go`

- [ ] **Step 1: Add CORS dependency**

Run:
```bash
cd services/api && go get github.com/go-chi/cors@latest && cd ../..
```
Expected: adds chi/cors.

- [ ] **Step 2: Implement server assembly**

Create `services/api/internal/app/server.go`:
```go
package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/varin/ivyticketing/services/api/internal/modules/system"
	appmw "github.com/varin/ivyticketing/services/api/internal/platform/middleware"
)

func NewRouter(cfg Config, log *slog.Logger, pg, rdb system.Checker) http.Handler {
	r := chi.NewRouter()
	r.Use(appmw.RequestID)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.WebOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-Id"},
		AllowCredentials: false,
	}))

	sys := system.NewHandler(pg, rdb)
	sys.RegisterRoutes(r)

	log.Info("router assembled", "web_origin", cfg.WebOrigin)
	return r
}

func StartServer(ctx context.Context, cfg Config, log *slog.Logger, handler http.Handler) error {
	srv := &http.Server{Addr: ":" + cfg.APIPort, Handler: handler}
	log.Info("api listening", "port", cfg.APIPort)
	return srv.ListenAndServe()
}
```

- [ ] **Step 3: Wire everything in main**

Replace `services/api/cmd/api/main.go` entirely:
```go
package main

import (
	"context"
	"os"

	"github.com/varin/ivyticketing/services/api/internal/app"
	"github.com/varin/ivyticketing/services/api/internal/platform/database"
	"github.com/varin/ivyticketing/services/api/internal/platform/logger"
	"github.com/varin/ivyticketing/services/api/internal/platform/redis"
)

func main() {
	cfg, err := app.LoadConfig()
	log := logger.New(cfg.AppEnv)
	if err != nil {
		log.Error("config load failed", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pg, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	rdb, err := redis.Connect(ctx, cfg.RedisURL)
	if err != nil {
		log.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()

	handler := app.NewRouter(cfg, log, pg, rdb)
	if err := app.StartServer(ctx, cfg, log, handler); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
```

Note: `LoadConfig` is called before logger uses `cfg.AppEnv`; if it errors, `cfg` is the zero value so `AppEnv==""` → JSON handler, which is fine for an error log.

- [ ] **Step 4: Verify it builds and all tests pass**

Run:
```bash
cd services/api && go build ./... && go test ./... && cd ../..
```
Expected: build OK; all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/api/internal/app/server.go services/api/cmd/api/main.go services/api/go.mod services/api/go.sum
git commit -m "feat(api): assemble router and wire dependencies in main"
```

---

## Task 7: Database migration + sqlc setup

**Files:**
- Create: `database/migrations/00001_create_schema_health.sql`
- Create: `database/queries/system.sql`
- Create: `services/api/sqlc.yaml`
- Create: `services/api/.env.example`
- Create: `.env.example` (root)

- [ ] **Step 1: Write the goose migration**

Create `database/migrations/00001_create_schema_health.sql`:
```sql
-- +goose Up
CREATE TABLE schema_health (
    id          SMALLINT PRIMARY KEY DEFAULT 1,
    checked_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT schema_health_singleton CHECK (id = 1)
);
INSERT INTO schema_health (id) VALUES (1);

-- +goose Down
DROP TABLE schema_health;
```

- [ ] **Step 2: Write the sqlc query**

Create `database/queries/system.sql`:
```sql
-- name: HealthPing :one
SELECT checked_at FROM schema_health WHERE id = 1;
```

- [ ] **Step 3: Write sqlc config**

Create `services/api/sqlc.yaml`:
```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "../../database/queries"
    schema: "../../database/migrations"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
```

- [ ] **Step 4: Write env templates**

Create `.env.example` (root):
```bash
APP_ENV=local
APP_NAME=ivyticketing
API_PORT=8080
DATABASE_URL=postgres://localhost:5432/ivyticketing?sslmode=disable
REDIS_URL=redis://localhost:6379
WEB_ORIGIN=http://localhost:4321
```

Create `services/api/.env.example` (identical, for running api standalone):
```bash
APP_ENV=local
APP_NAME=ivyticketing
API_PORT=8080
DATABASE_URL=postgres://localhost:5432/ivyticketing?sslmode=disable
REDIS_URL=redis://localhost:6379
WEB_ORIGIN=http://localhost:4321
```

- [ ] **Step 5: Commit**

```bash
git add database services/api/sqlc.yaml services/api/.env.example .env.example
git commit -m "feat(db): add schema_health migration, sqlc config, env templates"
```

Note: running `sqlc generate` and `goose up` happens in Task 9 (workflow), after tools are installed. Generated code under `services/api/internal/db/` will be committed there.

---

## Task 8: Astro web app

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/astro.config.mjs`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/tailwind.config.mjs`
- Create: `apps/web/.env.example`
- Create: `apps/web/src/styles/global.css`
- Create: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/layouts/PublicLayout.astro`
- Create: `apps/web/src/pages/index.astro`

- [ ] **Step 1: Scaffold package.json**

Create `apps/web/package.json`:
```json
{
  "name": "@ivyticketing/web",
  "type": "module",
  "version": "0.1.0",
  "scripts": {
    "dev": "astro dev",
    "build": "astro build",
    "preview": "astro preview"
  },
  "dependencies": {
    "astro": "^4.15.0"
  },
  "devDependencies": {
    "@astrojs/tailwind": "^5.1.0",
    "tailwindcss": "^3.4.0"
  }
}
```

- [ ] **Step 2: Install dependencies**

Run:
```bash
cd apps/web && pnpm install && cd ../..
```
Expected: creates `node_modules` and `pnpm-lock.yaml` (ignored / lock committed). If exact versions resolve higher, that is fine.

- [ ] **Step 3: Astro + Tailwind config**

Create `apps/web/astro.config.mjs`:
```js
import { defineConfig } from "astro/config";
import tailwind from "@astrojs/tailwind";

export default defineConfig({
  integrations: [tailwind()],
  server: { port: 4321 },
});
```

Create `apps/web/tailwind.config.mjs`:
```js
/** @type {import('tailwindcss').Config} */
export default {
  content: ["./src/**/*.{astro,html,js,ts}"],
  theme: { extend: {} },
  plugins: [],
};
```

Create `apps/web/tsconfig.json`:
```json
{
  "extends": "astro/tsconfigs/strict",
  "compilerOptions": {
    "types": ["astro/client"]
  }
}
```

Create `apps/web/src/styles/global.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

Create `apps/web/.env.example`:
```bash
PUBLIC_API_URL=http://localhost:8080
```

- [ ] **Step 4: API client wrapper**

Create `apps/web/src/lib/api.ts`:
```ts
export interface ReadyResponse {
  status: "ready" | "not_ready";
  checks: Record<string, string>;
}

const API_URL = import.meta.env.PUBLIC_API_URL ?? "http://localhost:8080";

export async function fetchReadiness(): Promise<ReadyResponse | null> {
  try {
    const res = await fetch(`${API_URL}/readyz`);
    return (await res.json()) as ReadyResponse;
  } catch {
    return null;
  }
}
```

- [ ] **Step 5: Layout + index page**

Create `apps/web/src/layouts/PublicLayout.astro`:
```astro
---
import "../styles/global.css";
const { title = "ivyticketing" } = Astro.props;
---
<!doctype html>
<html lang="id">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{title}</title>
  </head>
  <body class="min-h-screen bg-slate-50 text-slate-900">
    <main class="mx-auto max-w-2xl p-8">
      <slot />
    </main>
  </body>
</html>
```

Create `apps/web/src/pages/index.astro`:
```astro
---
import PublicLayout from "../layouts/PublicLayout.astro";
import { fetchReadiness } from "../lib/api";

const ready = await fetchReadiness();
---
<PublicLayout title="ivyticketing — system status">
  <h1 class="text-2xl font-bold mb-6">ivyticketing</h1>
  <section class="rounded-lg border border-slate-200 bg-white p-6">
    <h2 class="text-lg font-semibold mb-4">System status</h2>
    {ready === null ? (
      <p class="text-red-600">API tidak dapat dihubungi.</p>
    ) : (
      <ul class="space-y-2">
        {Object.entries(ready.checks).map(([name, status]) => (
          <li class="flex items-center justify-between">
            <span class="capitalize">{name}</span>
            <span class={status === "ok" ? "text-green-600" : "text-red-600"}>
              {status === "ok" ? "✅ ok" : "❌ down"}
            </span>
          </li>
        ))}
      </ul>
    )}
  </section>
</PublicLayout>
```

- [ ] **Step 6: Verify build**

Run:
```bash
cd apps/web && pnpm build && cd ../..
```
Expected: Astro build succeeds (readiness fetch returns null during build with no API running — handled gracefully).

- [ ] **Step 7: Commit**

```bash
git add apps/web
git commit -m "feat(web): add astro app rendering live api readiness status"
```

---

## Task 9: Dev scripts, Makefile, README + full verification

**Files:**
- Create: `scripts/dev/setup-local.sh`
- Create: `scripts/dev/start-local.sh`
- Create: `Makefile`
- Create: `README.md`

- [ ] **Step 1: Write setup-local.sh**

Create `scripts/dev/setup-local.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

PG_PREFIX="$(brew --prefix)/opt/postgresql@16/bin"

echo "==> Installing Postgres 16 and Redis (if missing)"
brew list postgresql@16 >/dev/null 2>&1 || brew install postgresql@16
brew list redis >/dev/null 2>&1 || brew install redis

echo "==> Starting services"
brew services start postgresql@16
brew services start redis

echo "==> Waiting for Postgres to accept connections"
for i in {1..30}; do
  if "$PG_PREFIX/pg_isready" -q; then break; fi
  sleep 1
done

echo "==> Creating database (if missing)"
"$PG_PREFIX/createdb" ivyticketing 2>/dev/null || echo "database already exists"

echo "==> Installing Go tools (goose, sqlc)"
go install github.com/pressly/goose/v3/cmd/goose@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

echo "==> Generating sqlc code"
(cd services/api && "$(go env GOPATH)/bin/sqlc" generate)

echo "==> Running migrations"
"$(go env GOPATH)/bin/goose" -dir database/migrations postgres \
  "postgres://localhost:5432/ivyticketing?sslmode=disable" up

echo "==> Installing web dependencies"
(cd apps/web && pnpm install)

echo "==> Setup complete. Run 'make dev' to start."
```

- [ ] **Step 2: Write start-local.sh**

Create `scripts/dev/start-local.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

cleanup() { kill 0; }
trap cleanup EXIT

echo "==> Starting API on :8080 and web on :4321"
(cd services/api && go run ./cmd/api) &
(cd apps/web && pnpm dev) &
wait
```

- [ ] **Step 3: Write Makefile**

Create `Makefile`:
```make
GOPATH_BIN := $(shell go env GOPATH)/bin
DATABASE_URL ?= postgres://localhost:5432/ivyticketing?sslmode=disable

.PHONY: setup dev api web migrate-up migrate-down sqlc test lint fmt

setup:
	bash scripts/dev/setup-local.sh

dev:
	bash scripts/dev/start-local.sh

api:
	cd services/api && go run ./cmd/api

web:
	cd apps/web && pnpm dev

migrate-up:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	$(GOPATH_BIN)/goose -dir database/migrations postgres "$(DATABASE_URL)" down

sqlc:
	cd services/api && $(GOPATH_BIN)/sqlc generate

test:
	cd services/api && go test ./...

lint:
	cd services/api && go vet ./...

fmt:
	cd services/api && go fmt ./...
	cd apps/web && pnpm exec prettier --write . || true
```

- [ ] **Step 4: Make scripts executable**

Run:
```bash
chmod +x scripts/dev/setup-local.sh scripts/dev/start-local.sh
```

- [ ] **Step 5: Write README**

Create `README.md`:
```markdown
# ivyticketing

Race registration & event ticketing platform. Go modular monolith + Astro frontend.

## Phase 1 — Foundation

Thin-but-live monorepo proving `web → api → Postgres + Redis`.

## Prerequisites

- macOS with Homebrew
- Go 1.25+
- Node 20+ and pnpm

## Setup from zero

```bash
make setup    # install + start Postgres/Redis, create db, install tools, migrate, pnpm install
make dev      # API on :8080, web on :4321
```

Open http://localhost:4321 — you should see Postgres ✅ and Redis ✅.

## Smoke test (verify the chain is live)

```bash
curl -s localhost:8080/healthz          # {"status":"ok"}
curl -s localhost:8080/readyz | jq      # both checks "ok"

brew services stop redis
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/readyz   # 503
brew services start redis
```

## Project structure

- `apps/web` — Astro frontend (public site, participant UI)
- `services/api` — Go modular monolith (Chi, pgx, sqlc)
- `database/migrations` — goose migrations
- `database/queries` — sqlc query sources
- `scripts/dev` — local setup/run scripts
- `docs/` — PRD, struktur, masterplan, specs, plans

## Make targets

`setup`, `dev`, `api`, `web`, `migrate-up`, `migrate-down`, `sqlc`, `test`, `lint`, `fmt`

## Next phase

Phase 2 — Auth, RBAC & multi-tenant core. See `docs/masterplan.md`.
```

- [ ] **Step 6: Run full setup**

Run:
```bash
make setup
```
Expected: Postgres + Redis installed/started, db created, goose+sqlc installed, `sqlc generate` produces `services/api/internal/db/`, migration applies, web deps installed.

- [ ] **Step 7: Verify the API and readiness (healthy)**

In one terminal: `make api`. In another:
```bash
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
```
Expected: `{"status":"ok"}` and `{"status":"ready","checks":{"postgres":"ok","redis":"ok"}}`.

- [ ] **Step 8: Verify readiness degrades (Redis down → 503)**

Run:
```bash
brew services stop redis
curl -s -o /dev/null -w "%{http_code}\n" localhost:8080/readyz
brew services start redis
```
Expected: `503`.

- [ ] **Step 9: Verify full test suite and migrate down/up**

Run:
```bash
cd services/api && go test ./... && cd ../..
make migrate-down && make migrate-up
```
Expected: tests PASS; migration rolls back and re-applies cleanly.

- [ ] **Step 10: Verify web renders status**

With `make dev` running, open http://localhost:4321 and confirm Postgres ✅ / Redis ✅ render. Stop Postgres (`brew services stop postgresql@16`), reload, confirm it shows ❌, then restart it.

- [ ] **Step 11: Commit**

```bash
git add scripts Makefile README.md services/api/internal/db services/api/go.sum
git commit -m "feat(dev): add setup/start scripts, Makefile, README; generate sqlc code"
```

---

## Self-Review Notes

- **Spec coverage:** struktur (T1-T8 dirs), health/readiness (T4), config/env (T2,T7), CORS (T6), sqlc+goose (T5,T7,T9), Makefile/scripts (T9), unit tests healthz+readyz fake checker (T4), smoke test (T9 + README), DoD items 1-9 all mapped (git/gitignore already done; setup T9; migrate T9; healthz/readyz T4/T9; redis-down T9; web T8/T9; go test T4/T6/T9; README T9; no hardcoded secrets T2/T7).
- **Type consistency:** `Checker` interface + `Ping(ctx)` used identically across system handler (T4) and adapters (T5); `NewHandler(pg, rdb)` and `NewRouter(...)` signatures align (T4/T6); `readyResponse` defined once (T4) and asserted in tests.
- **No placeholders:** every code step shows full content.
