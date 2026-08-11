# TODO - Implementation Tracking

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

## References

- [PLAN.md](./PLAN.md) - Detailed implementation plan
- [AGENTS.md](./AGENTS.md) - Project setup and conventions
- `~/.config/opencode/skills/chrome-devtools-testing.md` - Testing procedures
