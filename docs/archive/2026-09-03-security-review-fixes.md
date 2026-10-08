# 2026-09-03 — Security review fixes (creaves + creaves-console)

Cross-project security review (code review + UI review via agent-browser).
8 issues found, all fixed, regression-tested, live-verified and committed.
Quality gates run per project (go vet / staticcheck / gocognit -over 15 /
gocyclo -over 12 / go test -race -cover) — remaining warnings are
pre-existing and unrelated to these changes.

## Bug 1: [CRITICAL][SECURITY] creaves — anonymous privilege escalation via registration mass-assignment

**Observed**: `POST /registration/` binds the whole `models.User` struct from form params (`c.Bind(u)` in `actions/users.go:UsersCreate`) without resetting privileged fields. An anonymous attacker posting `Admin=true&Approved=true` becomes an approved administrator instantly (verified live: user `eviluser2` created with `admin=1, approved=1`, full admin access). The `users/_form.plush.html` template only *hides* the Admin/Approved/Shared checkboxes for non-admins (`if (current_user.Admin)`), which does not stop a crafted POST.

**Fix**: in `UsersCreate`, after `c.Bind`, force `u.Admin=false`, `u.Approved=false`, `u.Shared=false` unless `GetCurrentUser(c)` is an authenticated admin (admin-created-user flow preserved).

**Tests**: `actions/security_regression_test.go` — `TestUsersCreateRegistrationCannotSelfPromote` (crafted POST lands with all flags false), `TestUsersCreateAdminCanStillGrantFlags` (legitimate admin flow intact). MySQL-backed via `searchTestDB`.

**Live validation**: crafted anonymous POST with `Admin=true&Approved=true&Shared=true` → 302, DB row `admin=0 approved=0 shared=0` (pre-fix: all 1).

**Commit**: `6099112` (creaves).

## Bug 2: [CRITICAL][SECURITY] creaves-console — any user can self-promote to admin via profile update

**Observed**: `UsersResource.Update` allows a non-admin user to update their own record and calls `c.Bind(user)` on the full struct; `models.User` has `form:"admin"`/`form:"active"` tags, so `admin=true` in the PUT body promotes the caller (verified live: non-admin `viewer` became `admin=1`). The edit template also rendered Admin/Active checkboxes to non-admins.

**Fix**: capture `wasAdmin`/`wasActive` before bind; `protectPrivilegedFlags` restores them for non-admin callers and prevents an admin from removing their own admin flag (self-lockout protection). Edit templates (html/fr/de/nl) wrap Admin/Active checkboxes in `if (current_user.Admin)`.

**Tests**: `actions/security_regression_test.go` — `TestUserUpdateNonAdminCannotPromoteSelf`, `TestUserUpdateAdminCannotRemoveOwnAdmin` (sqlite).

**Live validation**: logged in as non-admin `viewer`, PUT own `/users/{id}` with `admin=true` → 303 but DB `admin=0`; edit form renders no admin checkbox (pre-fix: `admin=1`).

**Commit**: `c94d73c` (creaves-console).

## Bug 3: [HIGH][SECURITY] password_hash / key_hash exposed in JSON API responses

**Observed**: `models.User.PasswordHash` had `json:"password_hash"` in both projects; console `models.WebhookAPIKey.KeyHash` had `json:"key_hash"`. JSON endpoints (`r.JSON(user)`, `r.JSON(keys)`) returned bcrypt hashes / key hashes to callers.

**Fix**: `json:"-"` on `PasswordHash` (both projects) and `WebhookAPIKey.KeyHash` (console).

**Tests**: `TestUserJSONNeverExposesPasswordHash`, `TestWebhookAPIKeyJSONNeverExposesKeyHash` (console); creaves covered by live check + console-side marshal tests.

**Live validation**: `/users/` JSON (both apps) contains no `password_hash`; console `/webhook_api_keys/` JSON contains no `key_hash`.

**Commits**: `ca54a37` (creaves), `e3f6d1f` (creaves-console).

## Bug 4: [MEDIUM][SECURITY] creaves — webhook API key (plaintext) leaked in config JSON responses

**Observed**: `ConfigsResource` JSON responses include `Config.Settings` raw blob with `webhook_api_key` in plaintext — the shared secret authorizing pushes to the console.

**Fix**: `Config.MarshalJSON` (models/config.go) redacts `Settings.WebhookAPIKey` to `""` before marshaling (type-alias to avoid recursion). DB storage unchanged (pusher still reads the real key).

**Tests**: `models/config_redact_test.go` — redaction, preservation of other settings, empty settings.

**Live validation**: `GET /config/` with Accept: application/json → `"webhook_api_key":""` while DB still holds the real key (`creaves_e157db6c-…`) and webhook delivery still works.

**Commit**: `850dd05` (creaves).

## Bug 5: [MEDIUM] creaves — debug `/crash` route enabled in all environments

**Observed**: `GET /crash` panicked the server on demand, registered unconditionally in actions/app.go.

**Fix**: route deleted (plus now-unused `fmt` import).

**Live validation**: `GET /crash` → 404.

**Commit**: `7d15027` (creaves).

## Bug 6: [MEDIUM] creaves-console — webhook endpoint reads unbounded request body (DoS)

**Observed**: `WebhookEventsHandler` did `io.ReadAll(c.Request().Body)` with no size cap and no batch-size cap.

**Fix**: body wrapped in `http.MaxBytesReader` (10 MB) → 413 on overflow; batches over 1000 events → 413.

**Tests**: `TestWebhookRejectsOversizedBody`, `TestWebhookRejectsTooManyEvents` (console regression file).

**Live validation**: 11 MB body → 413; 1001-event batch → 413; normal 1-event POST → 200 with standard validation response.

**Commit**: `f583eda` (creaves-console). Note: `WebhookEventsHandler` gocognit 50→53 / gocyclo 31→33 — pre-existing hotspot, +3/+2 from these security checks.

## Bug 7: [LOW] creaves — webhook batch size not validated (0 / negative / huge)

**Observed**: `ConfigsResource.Create/Update` parsed `WebhookBatchSize`/`WebhookMaxPerMin` via `fmt.Sscanf` with defaults but no bounds checks; values flowed into SQL `LIMIT` and request sizing.

**Fix**: `parseWebhookLimits` clamps batch to 1-100 (default 1) and max-per-min to 1-10000 (default 60); used by both Create and Update.

**Tests**: `TestParseWebhookLimitsClampsValues` (table-driven: empty/zero/negative/huge/garbage).

**Commit**: `b399035` (creaves).

## Bug 8: [LOW][SECURITY] creaves — guest rate limiter trusts X-Forwarded-For unconditionally

**Observed**: guest phone-verification rate limiting keyed on the `X-Forwarded-For` header when present, letting attackers rotate spoofed XFF values to bypass the 30 attempts/15 min limit.

**Fix**: `guestClientIP` honours XFF only when the direct peer IP is listed in the `TRUSTED_PROXIES` env var (comma-separated); default is to trust none and use `RemoteAddr`. Both rate-limit call sites (GuestNew token path, GuestCreate) use it.

**Tests**: `TestGuestClientIP*` (4 cases: no proxies configured, untrusted peer, trusted peer, trusted peer without XFF).

**Commit**: `20f67d0` (creaves).

## UI review notes (no code issues found)

- creaves-console: all pages loaded clean as admin (`/consolidated_animals`, `/reports*`, `/instances`, `/sync_management`, `/users`, `/webhook_api_keys`); webhook returns 401 without/with bad Bearer key.
- creaves: all pages loaded clean (`/`, `/dashboard`, `/animals`, `/feeding`, `/registertable`, `/registersnapshot`, `/reports/annual`, `/users`, `/config`, `/event_streams`, `/translations`, `/maintenance`, `/webhook_resync`, `/guest`, `/paths`) — no template errors.
