# 2026-09-05 — Ordinary animal edits now emit animal_state webhook events

## Bug (bugs.md item 3, CRITICAL)

Editing an ordinary animal field in creaves (e.g. cage VE12→VE99 on animal 2058)
saved successfully but appended **no event** to `event_streams` — only
`animal_discovered`, `animal_status_changed`, `animal_died`/`animal_released`
transitions were published. The console therefore never saw ordinary edits and
its consolidated view stayed stale.

## Root cause

`animal_state` events existed only in the resync path
(`actions/webhook_resync.go`); the CRUD actions had no state-event hook. Two
related defects in the state-hash chain:

1. No `animal_state` publication on the update path.
2. `payload.state_hash` was never populated (the hash only lived in the
   `event_streams.content_hash` column), so the console's content-addressed
   no-op check (`event_processor.go`: skip `ApplyEvent` when
   `payload.state_hash == consolidated.state_hash`) never fired — every state
   event was re-applied in full.

## Fix

### creaves

- `actions/event_producer.go`: new `PublishAnimalStateEvent(tx, animalID, user)` —
  reloads the animal with the full eager-association list, builds the payload
  with the same canonical builder the resync uses, derives `current_status`
  exactly like `processResyncAnimal` (`released` iff an outtake exists), hashes
  via `StateContentHashPayload`, sets `payload.state_hash`, and creates the
  deterministic `StateEventUUID(instance, animal, hash)` event. An existing
  (instance, animal, hash) row suppresses the insert — a no-op save emits
  nothing. A lost insert race against a concurrent resync (duplicate
  deterministic UUID) is treated as "already published".
  Helpers: `createAnimalStateEvent`, `publishAnimalStateEventWarn`,
  `isDuplicateKeyError`.
- Hook sites (warn-only on failure, same contract as the other publish calls):
  - `actions/animals.go`: `Create`, `Update`
  - `actions/intakes.go`: `Create` (when already linked), `Update`, `Destroy`
  - `actions/outtakes.go`: `Create`, `Update`, `Destroy`
  - `actions/discoveries.go`: `Create`, `Update`, `Destroy`
  - Care entries are deliberately out of scope: care fields are not part of
    `CanonicalStateContent` and the console does not consolidate care data.
- `actions/webhook_resync.go`: `enqueueResyncStateEvent` now also sets
  `payload.state_hash` so resync and update-path events share one identity and
  the console no-op dedupe works for both.

### creaves-console

- No production code change needed: `event_processor.go` already implements the
  `payload.state_hash` no-op and the deterministic-UUID upsert; they become
  active now that the producer supplies the hash.

## Tests

- creaves `actions/animal_state_event_test.go` (sqlite suite):
  deterministic event with hash in column + payload; no-op update dedupes to
  one event; cage change publishes a second event with a new hash and the new
  cage in the payload; outtake ⇒ `current_status=released`; event-stream kill
  switch respected; audit-only payload differences hash identically.
- creaves-console `TestEventProcessor_StateEventCageChangeUpdatesSnapshot`:
  two state events (VE12→VE99) produce a snapshot with the new cage and latest
  state hash; `TestEventProcessor_AnimalStateSameHashIsIdempotent` (pre-existing)
  covers the same-hash no-op.

## E2E validation (dev servers + agent-browser)

1. Edited animal 1766 cage S10→S99 in creaves UI: animal_state event
   `374c5cbd-…` created and `delivered_at` set within seconds.
2. Console drill-down page showed `Zone/Cage: / S99`;
   `consolidated_animals.cage = S99`, state hash stored.
3. Reverted S99→S10: new event `761b0d45-…` delivered; console cage back to S10.
4. Re-saved the form unchanged (row `updated_at` bumped): **no** new event
   (count stayed 3) — content-hash dedupe verified end-to-end.

## Residual notes

- Existing consolidated rows keep `state_hash IS NULL` until they receive one
  new-format state event (next resync or edit backfills it) — relevant to the
  sync-checksum item (bugs item 4), not to this fix.
- Full suites green per guideline (vet, staticcheck, gocognit/gocyclo with only
  pre-existing findings noted, `go test -count=1 -race -cover ./...`, and the
  sqlite-tagged suites on both repos).
