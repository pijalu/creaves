# Fix archive — 2026-09-05 — event_stream "Locally Processed" column/row removal

## Original report (bugs.md)
> ## creaves: event_stream
> table show "Traité localement" but this does not bring much value -> remove from all language

## Fix
The local consolidation processed state (`ProcessedAt`) brings no value to the
event_streams UI. Removed:

- **Index table** (`creaves/templates/event_streams/index.plush.{html,fr,de,nl}.html`):
  the `event-streams.index.processed` `<th>` header and the matching
  `event.ProcessedAt` badge `<td>` cell.
- **Show page** (`creaves/templates/event_streams/show.plush.{html,fr,de,nl}.html`):
  the full "Locally processed" row including its hint.
- **Locale keys** (`creaves/locales/event_streams.{fr,en-us,de,nl}.yaml`):
  - `event-streams.index.processed`
  - `event-streams.show.locally-processed`
  - `event-streams.show.locally-processed-hint`

The `ProcessedAt` field remains in the model/database; only the UI display and
its locale strings were removed.

## Validation
- `go vet ./...` — clean.
- `staticcheck ./...` — only pre-existing warnings (grifts dot imports,
  `models/models.go` duplicate log import); none related to this change.
- `gocognit -over 15 .` / `gocyclo -over 12 .` — pre-existing entries only;
  no Go code changed by this fix.
- `buffalo test` — all packages pass (MySQL test DB).
- Browser verification (agent-browser skill) with the dev server on
  http://127.0.0.1:3000:
  - French `/event_streams/` — headers: ID, ID d'instance, ID d'animal,
    Type d'événement, Livré, Créé à. No "Traité localement" column; "Livré"
    still present.
  - French `/event_streams/<id>` — rows: ID, ID d'instance, ID d'animal,
    Type d'événement, Livré, Créé à. No "Traité localement" row.
  - English `/event_streams/` — headers: ID, Instance ID, Animal ID,
    Event Type, Delivered, Created At. No "Locally Processed" column.
  - English `/event_streams/<id>` — no "Locally Processed" row.

## Commit
- creaves `f98051a` — fix(event-streams): remove the 'Locally Processed' column
  and row from event_streams UI
