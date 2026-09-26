# TODO - Implementation Tracking

## Sync v2 rollout status

Sync v2 producer envelope, state identity primitives, resync lifecycle, and admin status endpoints validated through T7.3; deployment-specific manual checks are documented in the console Sync v2 plan.

This document tracks all pending TODOs and their implementation status. Items are organized by category and marked as ✅ COMPLETED or ⏳ PENDING.

## Event Stream Implementation (Phase 1-3)

### Phase 1: Foundation ✅ COMPLETED
- [x] **TODO-FUNC-001**: Create event stream table
  - Migration: `event_streams` table
  - Model: `models/event_stream.go`
  - Indexes: `(instance_id, animal_id, created_at)`

- [x] **TODO-FUNC-004**: Design event document schema
  - JSON structure with instance_id, animal_id, event_type, timestamp, payload
  - Payload includes: discovery_location, initial_status, current_status, timestamps

- [x] **TODO-FUNC-005**: Add instance identifier to all events
  - Config stores instance_id
  - Events tagged with instance_id
  - Default to hostname or UUID

### Phase 2: Event Production ✅ COMPLETED
- [x] **TODO-FUNC-002**: Implement event producer
  - `PublishEvent()` function in `actions/event_producer.go`
  - Hooks in discoveries.go for animal_discovered events
  - Hooks in animals.go for animal_status_changed events
  - Hooks in outtakes.go for animal_released/animal_died events

### Phase 3: Consolidation ⏳ IN PROGRESS
- [ ] **TODO-FUNC-003**: Build event processor
  - Grift/task: `buffalo task consolidation:process`
  - Reads unprocessed events
  - Upsert into `consolidated_animals` table
  - Mark events as processed
  - Support reprocessing

### Phase 4: Snapshot Task ✅ COMPLETED
- [x] Create snapshot task for complete animal export
  - `buffalo task event:snapshot` - Creates events for all animals
  - `buffalo task event:snapshot:force` - Force creates (may duplicate)
  - `buffalo task event:snapshot:stats` - Shows statistics
  - Skips animals that already have events

## Performance Optimization (Phase 4)

### Performance Todos ⏳ PENDING
- [ ] **TODO-PERF-001**: Cache reference data (animaltypes, caretypes, zones, etc.)
  - Location: `actions/typehelper.go`
  - Impact: HIGH - Every form render hits DB 5-6 times

- [ ] **TODO-PERF-002**: Cache current user in session/middleware
  - Location: `actions/users.go:SetCurrentUser`
  - Impact: HIGH - DB hit on every authenticated request

- [ ] **TODO-PERF-003**: Add pagination to LandingIndex
  - Location: `actions/landing.go`
  - Impact: HIGH - Loads ALL in-care animals unconditionally

- [ ] **TODO-PERF-004**: Migrate EnrichAnimals to EnrichAnimalsOptimized
  - Location: `actions/registertable.go`, `actions/registersnapshot.go`
  - Impact: MEDIUM - N+1 queries on register pages

- [ ] **TODO-PERF-005**: Add connection pool configuration
  - Location: `models/models.go`
  - Impact: MEDIUM - Default pool may not suit multi-user load

- [ ] **TODO-PERF-006**: Add HTTP cache headers for static assets
  - Location: `actions/app.go` (ServeFiles)
  - Impact: LOW - Reduces asset re-download

- [ ] **TODO-PERF-007**: Review tx.Eager() usage for N+1 elimination
  - Location: Multiple resource files
  - Impact: MEDIUM - Pop Eager can generate unexpected queries

- [ ] **TODO-PERF-008**: Add request timing/logging middleware
  - Impact: LOW - Helps identify slow endpoints in production

## Known Bottlenecks

- **Reference data fetched from DB on every request**: `actions/typehelper.go` loads animal types, care types, zones, etc. without caching
- **User re-fetched from DB every request**: `actions/users.go:SetCurrentUser` does `tx.Find(u, uid)` on every authenticated request
- **Landing page loads ALL animals**: `actions/landing.go` loads every in-care animal without pagination
- **N+1 queries**: Some handlers still use `EnrichAnimals` (older) instead of `EnrichAnimalsOptimized`
- **No connection pool tuning**: `models/models.go` uses default Pop connection settings
- **Eager() overuse**: Many handlers use `tx.Eager()` which can generate unexpected queries

## Implementation Status Summary

| Phase | Status | Completion |
|-------|--------|------------|
| Phase 1: Foundation | ✅ COMPLETED | 100% |
| Phase 2: Event Production | ✅ COMPLETED | 100% |
| Phase 3: Consolidation | 🔄 IN PROGRESS | 50% |
| Phase 4: Performance | ⏳ PENDING | 0% |

## Next Steps

1. Complete Phase 3: Consolidation
   - Create `regional-consolidation` database
   - Create `consolidated_animals` table
   - Implement event processor grift task
   - Test with browser

2. Begin Phase 4: Performance Optimization
   - Start with TODO-PERF-001 (cache reference data)
   - Then TODO-PERF-002 (cache current user)
   - Then TODO-PERF-003 (pagination)

## Care Expert System Engine (`models/careplan`)

Pure-Go engine per [docs/care-expert.md](./docs/care-expert.md). Key research/decision record (2026-09):

- [x] **CARE-PLAN-001**: Matcher field registry + AnimalContext (§5.1) — COMPLETED
  - `models/careplan/matcher_registry.go` — 25 default fields, append-only `FieldProvider` registration, op/type contract (`opsByType`)
  - `models/careplan/context.go` — DB-free DIP abstraction (`AnimalContext`), `days_in_care`, in-care flag
  - Tests: `registry_test.go`

- [x] **CARE-PLAN-002**: Matcher DSL parser via codegen (§5.2) — COMPLETED
  - Grammar `models/careplan/dsl.y` (source of truth) → **goyacc** (`golang.org/x/tools/cmd/goyacc`, x/tools extension)
  - Generated `dsl_yacc.go` is a **build artifact, NOT committed** — regenerate with `go generate ./models/careplan` (goyacc on PATH; easy build-phase generation ⇒ stays out of VCS)
  - Lexer = thin adapter over stdlib **`text/scanner`** (no bespoke scanning engine): two-char ops (`<=`, `>=`, `!=`, `!~`), case-insensitive keywords, negative-number folding, regex-safe unquote (`\"`/`\\` are the only escapes; other backslashes pass through so RE2 patterns like `"\d{3}"` need no double-escaping)
  - AST with 1-based byte-column token positions; `ParseError{Msg, Column}` for inline admin UI
  - Save-time semantic validation (`ValidateNode`): unknown field, undeclared op, literal/type mismatch, empty `IN ()`, BETWEEN bounds order, RE2 compile cached on node
  - Tests: `dsl_test.go` — grammar, precedence NOT>AND>OR, BETWEEN-AND ownership, syntax errors, semantic errors with exact columns, escapes

- [ ] **CARE-PLAN-003**: Evaluator with why-trace + Preview (§5.3–5.5) — PENDING
- [ ] **CARE-PLAN-004**: Schedule value object + occurrence generator (§4.3/§6.1: day-1 intake anchoring, `[intake, outtake)` clamp, `duration_days` counts generated days, DST boundary test) — PENDING
- [ ] **CARE-PLAN-005**: Action payload validation per kind (§4.2: feeding/medication/care/cleanup/weighing/observation + `instructions`) — PENDING
- [ ] **CARE-PLAN-006**: PlanSource interface + override resolver + status computation (§4.7/§6.1: slot-level & `replaces_kind` overrides, 8 statuses, apply window §10-A1, defer expiry §10-H1, per-kind grouping §6.2a, course latch §10-A4) — PENDING

## References

- [PLAN.md](./PLAN.md) - Detailed implementation plan
- [AGENTS.md](./AGENTS.md) - Project setup and conventions
- `~/.config/opencode/skills/chrome-devtools-testing.md` - Testing procedures