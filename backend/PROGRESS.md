# PROGRESS — Sprint 1 Backend (checkpoint 2026-10-24, sixth session)

## Sixth session — MVP citizen-auth contract alignment (2026-10-24)

Resolved the mismatch between the approved Sprint 1 MVP requirements and the
implemented auth contract. Backend-only changes; no frontend files touched, no
commits, no deployment, no production database touched.

### What changed

1. **Citizen registration is now exactly three fields** — `full_name`,
   `phone_number`, `password` (`RegisterConsumerInput`). The national-ID and
   `usage_types` requirements are gone from the citizen contract; legacy keys
   sent by old clients are ignored by the JSON decoder (never validated,
   never stored). Duplicate detection is by phone only (still 409).
   - **No schema migration was needed** (verified against
     `0001_init.up.sql`): `users.id_number` is already nullable, `users.status`
     stays `pending_approval` by default (explicitly written as `active`),
     and the default profile rows fit existing CHECK constraints
     (`customers.type IN ('home','business')`, nullable
     `homes.family_count`). Existing legacy pending consumers are left
     untouched — they still go through the admin review flow.
2. **No admin approval for citizens**: `RegisterConsumer` now creates the
   account with `status = active` plus the default ERD profile
   (`customers` home + `homes` row, `id_number` NULL). Login remains gated by
   phone verification (403 `PHONE_NOT_VERIFIED` until the OTP is confirmed).
   Providers/admins keep `pending_approval`; the admin review, suspend,
   reinstate and reject endpoints are unchanged and still work on consumers.
3. **Gated SMS-free demo phone verification**: new env
   `DEMO_PHONE_VERIFICATION` (`cfg.OTP.DemoExposeCode`, default false). When
   enabled, the freshly generated OTP is echoed as `dev_otp_code` in
   `POST /auth/register/*` and `POST /auth/otp/request` responses so a local
   demo completes register → verify → login with no SMS provider. Codes stay
   random, single-use, hashed at rest, with unchanged TTL/cooldown/attempt
   limits — no universal code, no weaker verification. `config.Validate()`
   refuses to start in production while the flag is on (unit-tested).
   `RequestOTP` now returns `(code string, err error)`; the code is discarded
   on SMS failure and in the password-recovery path.
4. **Docs**: `api/openapi.yaml` (3-field `RegisterConsumerRequest`, consumer
   status `active`, optional `dev_otp_code` on registration/OTP-request
   responses, login gate clarification, MVP/demo notes), `.env.example`
   (`DEMO_PHONE_VERIFICATION`), `README.md` (env table row).

### Tests

- New: `internal/config/config_test.go` (production refusal + dev allowance
  + default-off + Load parsing), `TestRegisterIgnoresLegacyFields` (httpapi),
  `TestDemoPhoneVerificationWithoutSMS` (integration, full flow via the
  echoed code, wrong-code still rejected).
- Updated: consumer registration steps in
  `tests/integration/auth_flow_test.go` (3 fields, status `active`, login
  right after OTP verify — the consumer approve step and the
  pending-approval login expectations were removed; provider approval and
  consumer reject flows retained), `TestRegistrationSurvivesOTPDispatchFailure`
  (3 fields, `active`), httpapi validation payload, `registrationPayload`
  unit test (dev-code branches).
- `go build ./...`, `go vet ./...`, `go test ./... -count=1` — ALL PASS.

---

# PREVIOUS CHECKPOINT (2026-10-24, fifth session)

## Fifth session — integration suite run against real PostgreSQL (2026-10-24)

The gating env vars (`RUN_INTEGRATION=1`, `DATABASE_URL` → `mawarid_test`) ARE
visible in this session's shell (Go toolchain at
`%LOCALAPPDATA%\Programs\go-toolchain\go\bin`, Go 1.27.3). The full suite was
run against the real database — no skips. Two registration bugs (both HTTP 500
on what must be 201) were reproduced, root-caused and fixed:

### Bug 1 — GORM model columns diverged from the SQL schema (SQLSTATE 42703)

- Reproduced: `TestRegistrationSurvivesOTPDispatchFailure` — provider register
  → 500; `TestSprint1EndToEnd` — consumer 2 (usage `business`) → 500 (consumer
  1 with usage `home` passed, which is the tell).
- Root cause (confirmed via `\d businesses` / `\d providers` on `mawarid_test`):
  GORM derives column names from Go field names, so
  `Business.CommercialImageURL` → `commercial_image_url` and
  `Provider.Description` → `description`, but `0001_init.up.sql` defines
  `cmmercial_image_url` (deliberate ERD spelling) and `provider_description`.
  Every INSERT into `businesses`/`providers` failed with
  `column ... does not exist (SQLSTATE 42703)`; the whole registration
  transaction rolled back → generic 500. All other 14 tables match GORM
  naming (checked column-by-column against `information_schema.columns`).
- Fix (models only — no schema change, no auth-flow change):
  `internal/models/models.go` — explicit `column:` tags:
  `Business.CommercialImageURL` → `column:cmmercial_image_url`,
  `Provider.Description` → `column:provider_description`. JSON field names
  (public API) are unchanged. No raw SQL references the old generated names.

### Bug 2 — stored provider logo_url missing the API base path (404)

- Reproduced after Bug 1 was fixed: `TestSprint1EndToEnd` step "serve logo"
  → `GET {logo_url}` returned the plain-text JSON-catch-all 404.
- Root cause: `UploadLogo` stored `logo_url = "/providers/{id}/logo"`, but
  the serving route is registered at `{APIBasePath}/providers/{id}/logo`
  (router.go, default `/api/v1`). The generated URL never matched a route.
- Fix: `internal/service/provider.go` — build the URL from
  `Cfg.HTTP.APIBasePath` (trailing-slash trimmed), so the stored URL always
  matches the registered route.

### Diagnosability improvement

- `internal/service/auth.go` — both registration transaction failures were
  swallowed without a log line (that is why the root cause was invisible in
  the server logs). They now log the DB error via
  `Log.ErrorContext(..., "err", txErr)` (SQL fragments + SQLSTATE only, no
  credentials, no PII) before returning the generic 500.

### Actual verification results (this session, all real, nothing skipped)

- `go build ./...` → exit 0
- `go vet ./...` → exit 0
- `gofmt -l .` → clean
- `go test ./... -count=1` → exit 0 — every package ok, INCLUDING
  `tests/integration` (RUN_INTEGRATION visible in this shell):
  - `internal/apierr`, `internal/auth`, `internal/cache`,
    `internal/database` (10.2s incl. embedded-migration regression test),
    `internal/http`, `internal/validation` — unit tests pass.
  - `tests/httpapi` — HTTP-layer tests pass.
  - `tests/integration -v -count=1` → **3/3 PASS**:
    - `TestSprint1EndToEnd` — **PASS** (was: 500 at consumer-2 registration,
      then 404 at serve-logo)
    - `TestRegistrationSurvivesOTPDispatchFailure` — **PASS** (was: 500 at
      provider registration)
    - `TestLogoutBodyValidation` — **PASS**
- `-race` not run: requires cgo/gcc, unavailable on this Windows shell
  (disclosed, not silently skipped).
- SMS stayed optional throughout: the dispatch-failure test proves a failing
  SMS gateway still yields 201 + `next_step: "request_otp"`; the happy-path
  tests use the in-process dev adapter (no real SMS).
- Nothing committed or pushed; the test database was used as-is (no reset);
  no production database touched.

---

# PREVIOUS CHECKPOINT (2026-10-23, retained for history)

Emergency stop requested. This file records the state at stop time.

## Verification status (last full run — after the migration-runner fix)

PostgreSQL 16.15 is installed and running locally; the `mawarid_test` database
exists (role `mawarid`). The integration suite is gated on `RUN_INTEGRATION` +
`DATABASE_URL`, which are set only in the user's interactive shell and are not
visible to the agent's shells. The agent did not guess credentials or modify
persistent environment settings, so the DB-gated tests **skip** here.

- `go build ./...` → **exit 0**
- `go vet ./...` → **exit 0**
- `go test ./...` → **exit 0** — all packages pass:
  - `internal/database` — includes the new DB-free regression test
    `TestEmbeddedMigrationsDiscovered` (guards the `.up.sql` discovery filter).
  - `internal/validation`, `internal/auth`, `internal/cache`, `internal/apierr` — unit tests pass.
  - `internal/http` — in-package tests pass (incl. `decodeJSONOptional` table,
    logout-400-before-revocation with nil service, `registrationPayload` OTP outcome).
  - `tests/httpapi` — HTTP-layer tests pass (envelope, x-request-id, 401/403/400/413,
    CORS allow+deny, validation-before-DB, panic recovery, no stack-trace leak,
    logout auth-precedence).
  - `tests/integration` → **SKIP** (correct behaviour in the agent's shell): `RUN_INTEGRATION`
    / `DATABASE_URL` not visible — includes `TestRegistrationSurvivesOTPDispatchFailure`
    and `TestLogoutBodyValidation`. On the user's shell (where the vars ARE set) the suite
    reaches the database; the migration fix is what unblocks it — see the
    migration-fix section for the exact SQLSTATE observed and the change made.

### Integration test build errors — RESOLVED (not pending)

Earlier build failures in `tests/integration` and `tests/httpapi` were fixed this session:

- `multipart` helper name collided with `mime/multipart` import → renamed to `buildMultipart`.
- Fabricated-response helpers (`mustResp`/`mustStatus`) in the integration test were dishonest
  (asserted against fake statuses) → removed; tests now assert on real responses.
- `http.StatusPayloadTooLarge` does not exist → `http.StatusRequestEntityTooLarge`.
- Unchecked `http.Post` errors flagged by vet → error checks added.
- `apierr` test assumed `ErrFieldRequired` had no details → rewritten against `BadRequest`.
- `stdLogWriter` missing `Printf` (gorm `logger.Writer`) → method added.
- Unused imports/vars in `sms/http.go`, `service/otp.go` → removed.

**No build errors are currently outstanding.**

## What exists now (this session's changes)

- `internal/service/admin.go` — **rewritten** to match the real package conventions
  (`apierr.*`, `Service.audit(ctx, actor, target, action, details, ip)`, `sms.Sender`):
  ListAccounts/GetAccount (decorate: pending-doc counts + provider names), ReviewAccount
  (approve→active + pending docs approved; reject→rejected + sessions revoked; audit +
  best-effort SMS; self/admin target forbidden), Suspend/Reinstate, ReviewDocument
  (+ `recomputeVerification`), ListPendingVerifications, ListAuditLogs.
- `internal/service/provider.go` — **rewritten** to the real models (Provider PK = user_id,
  `ProviderLocation`, `ProviderWorkingHour` with `StartTime/EndTime/IsActive`):
  GetProfile/UpdateProfile (end>start, lat/lng bounds, primary-location upsert),
  List/SetWorkingHours (7-slot cap, per-day uniqueness, HH:MM + close>open, location ownership),
  List/CreateLocation, UploadLogo/LogoFilePath, AdminListProviders, ProviderStats.
- `internal/service/otp.go` — removed limiter `Reset` on cooldown hit (it defeated the
  per-window resend cap; DB cooldown remains authoritative).
- `internal/http/` — **new package**: `server.go` (writeJSON, envelope errors, decodeJSON
  ≤1 MB → 413, pagination, pathID), `middleware.go` (x-request-id, CORS allow-list,
  panic recovery, body-free access log, JWT auth + DB account-state gate, RBAC, session meta),
  `router.go` (all routes under `/api/v1`, JSON 404 catch-all, `/health`), handlers for
  auth / account+documents+notifications / admin / provider.
- `cmd/server/main.go` — config → logger → PG connect + versioned migrations → Redis
  limiter with documented in-memory fallback → SMS adapter → services → admin bootstrap
  (`ADMIN_BOOTSTRAP_PHONE` + `ADMIN_BOOTSTRAP_PASSWORD`, refuses empty password, skips when
  an admin exists, logs no phone/password) → HTTP server with timeouts + graceful shutdown.
- `internal/database/database.go` — `Printf` added to `stdLogWriter`.
- `internal/sms/http.go` — unused `time` import removed.
- Tests written: `internal/validation`, `internal/auth` (password+jwt), `internal/cache`,
  `internal/apierr`, `tests/httpapi`, `tests/integration` (full E2E, DB-gated).
- `api/openapi.yaml` — **new**: OpenAPI 3.0.3 documentation of the 10 auth
  endpoints (register consumer/provider, OTP request/verify, login, refresh,
  logout, forgot/reset password, me): request/response schemas, per-status
  error codes, bearer auth, x-request-id, OTP/lockout rate limits.
  Validated with a throwaway yaml.v3 parser (outside the repo, no new project
  dependency): clean parse, no duplicate keys, all `$ref` targets resolve.
- `go mod tidy` run (go.sum complete).

### Bugs found & fixed while writing tests

- Logo upload wrongly required `document_type` → `parseUpload(..., requireDocType)` flag.
- `LogoFilePath` probed by URL suffix (stored URL has no extension) → probe `.png` then `.jpg`.
- `PAYLOAD_TOO_LARGE` was returned as 400 → now 413 (`httpapi` test enforces).

## Auth reliability fixes (second session — completed)

### Fix 1 — registration no longer fails when OTP dispatch fails

- `internal/service/auth.go`: `RegisterConsumer`/`RegisterProvider` now return
  `RegistrationResult{User, OTPSent}`. An OTP dispatch failure AFTER the account
  commit is logged and reported as `OTPSent=false` instead of a 500 (the old
  behaviour made clients retry into 409 `ACCOUNT_ALREADY_EXISTS`). Dead `now`
  variable removed.
- `internal/service/otp.go`: `RequestOTP` deletes the undelivered code when SMS
  dispatch fails, so the documented recovery call (`POST /auth/otp/request`) is
  not blocked by the cooldown of a code nobody received. Security preserved:
  the per-window limiter counter still applies and a deleted code cannot be
  verified.
- `internal/http/handlers_auth.go`: shared `registrationPayload(...)` helper —
  dispatch failure → 201 with `next_step: "request_otp"` + bilingual message;
  happy path unchanged (`verify_otp`).

### Fix 2 — logout rejects malformed JSON

- `internal/http/server.go`: `decodeJSON` refactored onto `decodeJSONBody`;
  new `decodeJSONOptional` accepts an EMPTY body only — malformed JSON,
  trailing content and >1 MB bodies are still rejected.
- `internal/http/handlers_auth.go`: `handleLogout` uses `decodeJSONOptional`:
  malformed JSON → 400 `INVALID_JSON`; empty body / `{}` → logout-all
  (unchanged); valid `refresh_token` → single-session logout (unchanged).

### Tests added

- `internal/http/handlers_auth_test.go` (new file): `decodeJSONOptional` table
  (empty / whitespace / `{}` / valid token / malformed / trailing), oversized
  body → 413, handler-level logout 400 with nil service (proves no revocation
  attempt), `registrationPayload` both branches.
- `tests/httpapi`: `TestLogoutUnauthenticatedPrecedesBodyParsing` (401 before
  body parsing, no DB needed).
- `tests/integration` (DB-gated): `newEnvWithSMSSender` harness +
  `failFirstSender` (fails N sends, then delegates); both currently SKIP
  locally (no PostgreSQL) — see Unresolved 1:
  - `TestRegistrationSurvivesOTPDispatchFailure` — provider+consumer register
    during a simulated outage → 201 `request_otp`; re-register → 409; the
    account exists; `otp/request` after recovery → 200; verify → 200.
  - `TestLogoutBodyValidation` — malformed → 400 and the session survives;
    trailing content → 400; `{}` and no-body → logout-all (refresh dies);
    `refresh_token` → exactly that session dies, the other survives.

### Note — gofmt normalization (deviation, disclosed)

`gofmt -w` was run over the module while fixing auth.go: files that were never
gofmt-clean were reformatted (whitespace/alignment only, no semantic change).
Beyond the fix files this touched: `internal/auth/jwt.go`,
`internal/config/config.go`, `internal/http/{handlers_account,handlers_admin,router}.go`,
`internal/models/models.go`, `internal/service/{account,admin,provider}.go`,
`internal/validation/validation_test.go`, `tests/httpapi`, `tests/integration`.
Build/vet/tests were re-verified after.

### API doc updates (api/openapi.yaml, re-validated)

- Registration 201 + `next_step` enum `[verify_otp, request_otp]` with the
  recovery instruction; register endpoint descriptions updated.
- `otp/request` documents that a failed dispatch discards the undelivered code
  (immediate retry allowed; per-window limit still applies).
- Logout documents 400 `INVALID_JSON` on malformed bodies.

## Setup documentation (third session — completed)

Docs and `.env.example` only — no code, tests, or migrations touched.

- `README.md` — **new**: Windows/PowerShell setup guide (tools + versions with
  Go 1.24 requirement from go.mod; PostgreSQL via Docker Compose **or** native
  Windows install incl. psql role/db creation; environment configuration with
  the `.env`-loading snippet — verified the server does NOT auto-load `.env`,
  no godotenv in go.mod; `go run ./cmd/server` + smoke checks; build/vet/test;
  honest integration-test gating; Redis-optional explanation; project layout).
- `.env.example` — added the variables `config.Load()` reads but the template
  was missing: `ADMIN_BOOTSTRAP_PHONE`, `ADMIN_BOOTSTRAP_NAME`,
  `ADMIN_BOOTSTRAP_PASSWORD` (commented out, placeholder phone `7XXXXXXXX`,
  empty password, with the server-enforced rules documented), plus
  `HTTP_READ_TIMEOUT_SEC`, `HTTP_WRITE_TIMEOUT_SEC`,
  `DATABASE_MAX_OPEN_CONNS`. Every variable in `config.go` is now represented.
- `docker-compose.yml` — reviewed, unchanged (PG16 + Redis7, matches the
  default `DATABASE_URL`).
- Redis optionality documented both ways: explicit `REDIS_DISABLED=true`
  (in-memory fallback) and automatic fallback when Redis is unreachable at
  startup; per-process caveat (never production multi-instance). PostgreSQL
  remains required (server exits without it).

## Migration-runner fixes (fourth session — completed)

The integration suite surfaced two defects in the existing embedded-migration
runner (`internal/database/database.go`). Both were fixed in place, reusing the
same runner and the same `0001_init.up.sql` file — no second migration system
was introduced, the SQL file's schema was not changed, and no new dependency was
added.

### Fix 1 — file discovery matched nothing (off-by-three)

The `.up.sql` filter sliced the last **11** characters of each filename and
compared them to the **8**-character string `".up.sql"`, so no file could ever
match. `Migrate` created only `schema_migrations`, reported success, and applied
nothing — every DB-dependent test then failed with
`ERROR: relation "roles" does not exist (SQLSTATE 42P01)`.

- Fix: the check now uses `strings.HasSuffix(e.Name(), ".up.sql")`, extracted
  into an `upMigrations([]fs.DirEntry) []string` helper.
- Regression test (no database needed): `internal/database/migrate_test.go` →
  `TestEmbeddedMigrationsDiscovered` runs the real `upMigrations` helper over the
  embedded FS and asserts `0001_init.up.sql` is discovered.

### Fix 2 — multi-statement body rejected (SQLSTATE 42601)

Once discovery worked, applying the file failed with
`ERROR: cannot insert multiple commands into a prepared statement (SQLSTATE 42601)`.

- Root cause: GORM's PostgreSQL driver (pgx) defaults to the **extended query
  protocol**, where a prepared statement may contain only ONE command. The
  migration body is a multi-statement file (`BEGIN; ... COMMIT;`), so the whole
  body as a single prepared statement is rejected.
- Fix (smallest safe change): `Migrate` still uses the normal GORM pool for the
  `schema_migrations` bookkeeping, but each pending migration body is executed by
  a new `execMigration` helper over a **dedicated connection** whose
  `DefaultQueryExecMode` is `pgx.QueryExecModeSimpleProtocol`. The simple query
  protocol accepts multiple commands in one message, and PostgreSQL runs them as
  a single implicit transaction — preserving atomicity for files without their
  own `BEGIN/COMMIT`, while files that do open a transaction keep their explicit
  control. No client-side SQL splitting, no global protocol change (the rest of
  the app keeps using prepared statements), version tracking unchanged.
- pgx (`github.com/jackc/pgx/v5`) was already in the module graph (indirect via
  the GORM driver); importing it directly adds no new dependency.
- The DSN is obtained from the dialector, so `Migrate(db, log)`'s signature and
  both call sites (`cmd/server/main.go`, the integration harness) are unchanged.

### Verification after the fixes

- `go build ./...` → exit 0; `go vet ./...` → exit 0.
- `go test ./...` → exit 0 (integration tests skip in the agent's shell for lack
  of visible DB env vars).
- Exact integration results could not be produced by the agent: `DATABASE_URL` /
  `RUN_INTEGRATION` are set only in the user's shell, and the agent did not guess
  credentials or edit persistent env settings. The 42P01 and 42601 failures were
  reproduced and are fixed; on the user's shell the same suite is what exercises
  the now-working migration path.

## Unresolved

1. ~~Integration suite has not been run by the agent against the real PostgreSQL,~~
   **RESOLVED (fifth session)**: run against `mawarid_test` with `RUN_INTEGRATION=1`
   — 3/3 tests pass (see the fifth-session section at the top). The two registration
   500s it exposed (GORM↔SQL column mismatch, `logo_url` base-path) are fixed.
2. `api/openapi.yaml` written for the **auth endpoints only** (scope of the
   documentation task just completed). Account/documents/notifications,
   provider-profile, admin and health endpoints are still undocumented.
   Inconsistencies found while documenting: (a) `Refresh` returns
   `ACCOUNT_PENDING_APPROVAL` for rejected/suspended accounts too (still open);
   (b) registration returned 500 when the OTP SMS failed although the account
   was already created (retry → 409) — **FIXED** this session (see the
   reliability-fixes section above); (c) `models.OTPPurposeLogin` ("login") is
   rejected by the OTP endpoints (dead constant, still open); (d) Go mux 405
   responses are plain text, outside the JSON envelope (still open);
   (e) `handleLogout` swallowed malformed JSON — **FIXED** this session;
   (f) gofmt misalignment — resolved by the module-wide gofmt normalization.
3. ~~`README.md` / Windows startup instructions~~ — **DONE** (see the setup-documentation
   section below).
4. ~~`.env.example` completeness~~ — **DONE**: added the missing variables
   (`ADMIN_BOOTSTRAP_PHONE`/`NAME`/`PASSWORD`, `HTTP_READ_TIMEOUT_SEC`,
   `HTTP_WRITE_TIMEOUT_SEC`, `DATABASE_MAX_OPEN_CONNS`); all placeholders only.
5. Nothing committed to git (repo `backend/`, branch `feature/backend-sprint1`, all files untracked).
6. Final 10-section delivery report not produced.

## Next steps (in order)

1. ~~Run the integration suite against the real PostgreSQL~~ — **DONE (fifth session)**:
   `go test ./tests/integration -v -count=1` → 3/3 PASS on `mawarid_test`. The bugs it
   exposed (GORM↔SQL column mismatch on `businesses`/`providers`, `logo_url` missing the
   `/api/v1` base path) are fixed; see the fifth-session section at the top.
2. Extend `api/openapi.yaml` with the remaining registered routes (account,
   documents, notifications, provider, admin, health — source of truth =
   `internal/http/router.go`); the auth section is done.
3. ~~Write `README.md`~~ — DONE (Windows/PowerShell setup: tools+versions, PG via
   compose or native install, `.env` loading (no auto-load — verified: no
   godotenv), run/build/vet/test, integration-test gating, Redis-optional).
4. ~~Verify `.env.example` completeness~~ — DONE (6 missing variables added);
   `docker-compose.yml` unchanged as the dev dependency set.
5. `git add` + commit on `feature/backend-sprint1` (only with user permission per original rules).
6. Produce the required 10-item delivery report with honest test results and documented
   SRS conflicts (URS 1.3 pending-login block vs document upload order; URS 1.2 auto-approve
   vs flows-doc admin approval; SRS single-form registration vs JSON+separate upload API;
   password policy is an implementation decision; SMS=dev adapter only, no OCR available).

No existing code was modified outside the backend repo during this stop; no databases were
touched; nothing was pushed or merged.
