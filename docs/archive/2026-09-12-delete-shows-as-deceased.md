# Fix archive — 2026-09-12 — Delete shows as deceased in console

## Original report (bugs.md)
> ## Delete show as deceased in console
> Delete show remove the animal from the console and not show it dead

## Fix

Creaves "Destroy" attaches an **error outtake** (type *Doublon*: rating -1, dead 0,
error 1) — it is a data-entry removal, **not** a death. Two sides:

### creaves (producer)
- `models/event_stream.go`: new `EventTypeAnimalDeleted = "animal_deleted"`.
- `actions/event_producer.go`: `PublishAnimalDeletedEvent` publishes the new type with
  a payload marking `current_status: "deleted"`.
- `actions/animals.go` (`Destroy`): publishes `animal_deleted` instead of
  `PublishAnimalDiedEvent`.
- `actions/webhook_resync.go`: chunk loader (`loadResyncAnimalChunk`) and expected-set
  counter (`countAnimals`) exclude animals whose outtake is an error outtake
  (`COALESCE(ot.error, 0) = 0`), so resync totals/checksums stay aligned with the event
  history (destroyed animals are never re-announced).

### creaves-console (receiver)
- `models/event_stream.go`: `animal_deleted` added to the known event types.
- `actions/event_processor.go`: `processEvent` parses the payload first; for
  `animal_deleted` **or** legacy events carrying an error outtake
  (`payload.Outtake.Error`) it calls `deleteConsolidatedAnimal`, which removes the
  `consolidated_animals` row **and all `event_streams` rows** of that instance/animal —
  the `existing()` disaster-recovery path would otherwise resurrect the animal from its
  old events. Redelivery is idempotent.
- `actions/event_display.go`: localized `animal_deleted` labels (en/fr/de/nl) + badge.

## Tests
- creaves: `resync_excludes_destroyed_test.go` (error-outtake animal excluded from the
  chunk loader and from `countAnimals`), `event_producer_integration_test.go::
  TestPublishAnimalDeletedEvent_TypeAndStatus`.
- creaves-console: `event_processor_delete_test.go` — animal_deleted removes the
  consolidated row; legacy error-outtake state deletes it; redelivery idempotent.
- Full suites green: `go test -count=1 -race -cover ./...` (both repos; console with
  `-tags sqlite`).

## Verification (e2e, agent-browser + DB)
```
creaves  :3000  create reception animal -> /animals/980108 created
console  :3001  consolidated_animals total 10142 -> 10143 (row present)
creaves  :3000  /animals/980108 -> Destroy (confirm "Are you sure?") -> redirected
creaves  DB     outtake Doublon (rating -1, error 1) attached;
                event_streams: animal_discovered, animal_state, animal_deleted all delivered
console  DB     consolidated_animals row 980108 gone, its event_streams rows gone,
                total back to 10142; /consolidated_animals/?search=980108 -> empty
```

## Commits
- creaves `a1fb99d` feat(events): publish animal_deleted on destroy; keep destroyed
  animals out of resync
- creaves-console `473303f` feat(webhook): process animal_deleted events by removing
  the consolidated animal
