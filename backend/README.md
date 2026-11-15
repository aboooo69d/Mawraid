# Mawarid Backend (Sprint 1)

Go + GORM + PostgreSQL + Redis REST API for the Mawarid gas-distribution
platform (authentication, accounts, documents, provider profiles, admin
review). Mobile-first frontend is Next.js/React and talks to this API at
`http://localhost:8080/api/v1` (see `api/openapi.yaml` for the auth contract).

This guide targets **Windows + PowerShell** development.

---

## 1. Required tools

| Tool | Version | Notes |
|------|---------|-------|
| Go | **1.24+** (module `go 1.24`; developed with 1.27.x) | `go version` to check |
| PostgreSQL | **16** recommended (compose uses `postgres:16-alpine`; 14+ should work) | Docker **or** native install |
| Redis | **7** (optional — see §6) | Only used for rate-limit counters |
| Docker Desktop | any recent (optional) | Easiest way to run PG + Redis |
| PowerShell | 5.1 or 7+ | Commands below are PowerShell |

No other dependencies are required; Go modules are resolved automatically on
first build.

> **Note for this machine:** if `go` is not on `PATH`, either add
> `%LOCALAPPDATA%\Programs\go-toolchain\go\bin` to your user `PATH` or call the
> binary by full path (`& "$env:LOCALAPPDATA\Programs\go-toolchain\go\bin\go.exe"`
> in place of `go` below).

---

## 2. Local PostgreSQL

### Option A — Docker Compose (preferred when Docker is available)

From the `backend/` directory:

```powershell
docker compose up -d      # starts PostgreSQL 16 + Redis 7
docker compose ps         # both should be "running (healthy)"
```

This creates a PostgreSQL instance that matches the default `.env.example`
exactly:

- host/port: `localhost:5432`
- user: `mawarid`, password: `mawarid_dev_password`, database: `mawarid`
- data persists in the named volume `mawarid_pgdata`

Stop with `docker compose down` (data persists) or `docker compose down -v`
(**destructive** — deletes the data volumes).

### Option B — native PostgreSQL on Windows (no Docker)

1. Install PostgreSQL 16 (EDB installer, or `winget install PostgreSQL.PostgreSQL.16`
   where winget is available).
2. Remember the superuser (`postgres`) password you set during install.
3. Create the role and database to match `.env.example` (via `psql` or pgAdmin):

```powershell
psql -U postgres -c "CREATE ROLE mawarid LOGIN PASSWORD 'mawarid_dev_password';"
psql -U postgres -c "CREATE DATABASE mawarid OWNER mawarid;"
```

4. If your PostgreSQL listens on another port or host, adjust `DATABASE_URL`
   in `.env` accordingly.

---

## 3. Environment configuration

The server reads configuration **only from environment variables** — it does
**not** auto-load `.env`. Copy the template and edit as needed:

```powershell
Copy-Item .env.example .env      # .env is git-ignored — never commit it
notepad .env
```

Important variables (full annotated list in `.env.example`):

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | PostgreSQL DSN (**required** — the server refuses to start without it) |
| `DATABASE_AUTO_MIGRATE` | `true` applies the embedded versioned migrations on boot (local dev only) |
| `JWT_SECRET` | ≥ 32 chars; production refuses the dev default |
| `ADMIN_BOOTSTRAP_PHONE` / `ADMIN_BOOTSTRAP_PASSWORD` | Optional: create the first admin on an **empty** DB (see §4) |
| `REDIS_DISABLED` | `true` = run without Redis using the in-memory limiter (see §6) |
| `SMS_PROVIDER` | `dev` (default) = OTP codes rendered into the server log, **no real SMS** |
| `DEMO_PHONE_VERIFICATION` | `true` = the freshly generated OTP is echoed as `dev_otp_code` in the registration / OTP-request responses so a **local demo** needs no SMS provider. Random single-use codes only (never universal); the server **refuses to start** in production with it |

**Migrations** are embedded (`internal/database/migrations`, versioned
`.up.sql` files) and run automatically at startup when
`DATABASE_AUTO_MIGRATE=true`. With it set to `false`, apply them manually with
your own tooling — the server only warns.

### First admin account (bootstrap)

On a fresh database, set both variables for the first start only:

```powershell
$env:ADMIN_BOOTSTRAP_PHONE = "7XXXXXXXX"        # valid Yemeni number
$env:ADMIN_BOOTSTRAP_PASSWORD = "<strong-password>"
```

Rules enforced by the server: skipped when an admin already exists; refuses an
empty password; validates the phone format; **never logs** the phone or
password. Remove the variables after the first successful start.

---

## 4. Running the backend (PowerShell)

```powershell
cd C:\path\to\backend

# Load the .env file into the current shell session:
Get-Content .env | ForEach-Object {
  if ($_ -match '^\s*([^#][^=]*)=(.*)$') {
    [Environment]::SetEnvironmentVariable($Matches[1].Trim(), $Matches[2].Trim())
  }
}

# Run (with automatic migrations when DATABASE_AUTO_MIGRATE=true):
go run ./cmd/server
```

Expected log lines confirm the startup order: database connect → migrations →
limiter (Redis or in-memory fallback) → SMS adapter → admin bootstrap →
`http server listening on :8080`.

Smoke checks:

```powershell
Invoke-RestMethod http://localhost:8080/health     # db/redis/sms status
Invoke-RestMethod http://localhost:8080/api/v1/auth/me   # 401 envelope = alive
```

Stop the server with `Ctrl+C` (graceful shutdown, 10 s drain).

---

## 5. Build, vet, test

```powershell
go build ./...     # compile every package
go vet ./...       # static analysis
go test ./...      # unit + HTTP-layer tests (no database needed)
```

`go test ./...` must end with all packages `ok`. The DB-dependent integration
suite is included but **skips itself** unless enabled (see §7).

---

## 6. Redis is optional in local development

Redis is used **only** for shared rate-limit counters (OTP resend windows,
login lockout). For local development it is optional in two ways:

1. **Explicit:** set `REDIS_DISABLED=true` — the server uses a built-in
   in-process limiter and logs a warning.
2. **Automatic:** leave Redis enabled but unreachable — if the connection
   fails at startup, the server logs a warning and falls back to the in-memory
   limiter by itself.

The in-memory limiter is **per process**: perfectly fine for a single local
instance, but it does **not** share counters across processes — never run
production multi-instance deployments without Redis.

PostgreSQL, by contrast, is **required** — the server exits if it cannot
connect.

---

## 7. Running the database integration tests

The end-to-end suite (`tests/integration`) needs a real PostgreSQL. It is
gated and behaves honestly:

- Without the variables → tests **SKIP** (reported as skipped, never fake-pass).
- With `RUN_INTEGRATION=1` and an unreachable `DATABASE_URL` → tests **FAIL
  loudly** (so real breakage is never hidden).

```powershell
$env:RUN_INTEGRATION = "1"
$env:DATABASE_URL = "postgres://mawarid:mawarid_dev_password@localhost:5432/mawarid?sslmode=disable"
go test ./tests/integration -v
```

The suite runs its own migrations against the target database; use a
**disposable** database — it writes users, OTPs, documents and audit rows.
SMS note: the suite uses the dev adapter and reads OTP codes from the captured
server log; no real SMS is claimed or sent.

---

## 8. Honesty notes

- **SMS:** the default `dev` adapter renders OTP codes into the server log —
  it does **not** deliver real messages. The generic HTTP gateway adapter is
  unverified against any real provider.
- **Secrets:** everything sensitive comes from the environment; nothing is
  hardcoded, and phone numbers/passwords are never logged.
- **API contract:** success responses are plain JSON; errors use the
  `{error:{code,message,message_ar,request_id,details}}` envelope with an
  `x-request-id` header. Arabic messages are provided per SRS NFR-USA-02.

## 9. Project layout (short)

```
backend/
├── api/openapi.yaml            # OpenAPI 3.0.3 — auth endpoints
├── cmd/server/                 # entry point (startup order, admin bootstrap)
├── internal/
│   ├── config/                 # env-based configuration + validation
│   ├── database/               # connect + embedded versioned migrations
│   ├── http/                   # router, middleware, handlers
│   ├── service/                # business logic
│   ├── auth|cache|sms|...      # argon2id/JWT, limiter, SMS adapters
│   └── models/                 # GORM models
├── tests/httpapi/              # HTTP-layer tests (no DB)
├── tests/integration/          # E2E suite (DB-gated, §7)
├── docker-compose.yml          # dev PostgreSQL 16 + Redis 7
└── .env.example                # configuration template (placeholders only)
```
