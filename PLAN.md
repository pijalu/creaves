# Event Stream Implementation Plan

## Overview
This document tracks the implementation of the event stream system for multi-instance consolidation. It will be updated at each key milestone to ensure work can be resumed across sessions.

## Current Status: IN PROGRESS

## Implementation Checklist

### Phase 1: Foundation ✅ COMPLETED
- [x] Database migration to rename `instance_configs` to `config`
- [x] Add `settings` JSON column to config table
- [x] Create `models/config.go` with Config struct and ConfigSettings
- [x] Create `actions/configs.go` with CRUD operations
- [x] Create `actions/event_streams.go` with List/Show/Destroy
- [x] Create event stream table migration
- [x] Create `models/event_stream.go` model
- [x] Add admin menu links for Configuration and Event Stream
- [x] Create all templates (config CRUD, event_streams list/show)
- [x] Fix form binding issues for boolean checkboxes

### Phase 2: Feature Flag Consolidation ✅ COMPLETED
- [x] Merge all feature flags into single `eventstream` flag
  - [x] Remove EnableConsolidation, EnableNotifications, EnableReporting from ConfigSettings
  - [x] Update DefaultSettings() to only have EnableEventStream
  - [x] Update templates to show only eventstream flag
  - [x] Update Create/Update handlers in configs.go
  - [x] Remove unused helper functions
- [x] Update documentation

### Phase 3: Event Producer Implementation ✅ COMPLETED
- [x] Verify event producer is hooked into:
  - [x] Animal creation via intake (discoveries.go)
  - [x] Animal status updates (animals.go)
  - [x] Animal outtake (outtakes.go)
- [x] Test event creation via browser for each flow
- [x] Fix any issues found during testing
  - [x] Fixed EventType type from custom type to string
  - [x] Updated all references to use string type

### Phase 4: Snapshot Task ✅ COMPLETED
- [x] Create grift task for complete animal snapshot
  - [x] `buffalo task event:snapshot` - Creates events for all animals
  - [x] `buffalo task event:snapshot:force` - Force creates (may duplicate)
  - [x] `buffalo task event:snapshot:stats` - Shows statistics
  - [x] Skips animals that already have events
- [x] Test snapshot task
  - [x] Successfully created 11,595 events for 5,846 animals
  - [x] Verified via browser - events display correctly

### Phase 5: Consolidation Database ⏳ IN PROGRESS
- [ ] Create `regional-consolidation` database
- [ ] Create consolidated_animals table schema
- [ ] Create event processor grift task
- [ ] Test event processing from multiple instances
- [ ] Test consolidation via browser

### Phase 6: Documentation & Cleanup ✅ COMPLETED
- [x] Separate TODOs from AGENTS.md into dedicated TODO.md
- [x] Mark implemented vs not implemented clearly
- [x] Update all documentation with current status
- [x] Final end-to-end testing
  - [x] Event stream list page working
  - [x] Event stream show page working
  - [x] Configuration page working with single eventstream flag

## Current Implementation Details

### Database Schema
```sql
-- config table (renamed from instance_configs)
- id (uuid, PK)
- instance_id (string)
- name (string)
- description (text)
- active (bool)
- settings (json)
- created_at/updated_at (timestamp)

-- event_streams table
- id (uuid, PK)
- instance_id (string)
- animal_id (uuid)
- event_type (string)
- payload (json)
- created_at (timestamp)
- processed_at (timestamp, nullable)
```

### Feature Flags (CURRENT - TO BE SIMPLIFIED)
```go
type ConfigSettings struct {
    EnableEventStream    bool `json:"enable_event_stream"`     // KEEP
    EnableConsolidation  bool `json:"enable_consolidation"`    // REMOVE
    EnableNotifications bool `json:"enable_notifications"`    // REMOVE
    EnableReporting     bool `json:"enable_reporting"`        // REMOVE
}
```

### Event Types
- `animal_discovered` - New animal intake
- `animal_status_changed` - Status update
- `animal_released` - Successful outtake
- `animal_died` - Death outtake

## Testing Status

### Browser Testing Required
1. **Intake Flow**: Create new animal → verify event created
2. **Status Update**: Edit animal status → verify event created
3. **Outtake Flow**: Release animal → verify event created
4. **Config Management**: Toggle eventstream flag → verify persistence
5. **Event Stream View**: View events list → verify display
6. **Consolidation**: Process events → verify consolidated view

### Databases
- **creaves** (main application database)
- **regional-consolidation** (consolidated view - TO BE CREATED)

## Next Steps
1. Merge feature flags into single eventstream flag
2. Test event creation via browser for all animal flows
3. Create snapshot task for initial export
4. Set up consolidation database and processor

## Session Notes

### Session 2026-04-23
- Created PLAN.md document
- Started Phase 2: Feature flag consolidation
- Fixed form binding issues in configs.go (manual param binding for booleans)
- Need to: simplify feature flags, test event creation, create snapshot task

## Blockers/Issues
- None currently

## References
- AGENTS.md - Project setup and conventions
- TODO.md - Performance and feature todos (TO BE CREATED)
- ~/.config/opencode/skills/chrome-devtools-testing.md - Testing procedures
