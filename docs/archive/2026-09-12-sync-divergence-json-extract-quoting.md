# Sync status divergence after a completed full resync — FIXED

**Date**: 2026-09-12
**Symptom**: Console `/sync_management` (instance `dev`): Confirmés 0 / Non confirmés 10140,
stored checksum `sha256:a51ae571c72176256a437bf71cc0c8e645c4cdbb75f5be2a3312c7b1b193e689`
vs producer-announced `sha256:5f7d9b25b665309d1561cb4edcda689e19f4e2897bcb5995b45363e960d069a8`
("checksum mismatch"), while the producer `/webhook_resync` reported 10140 delivered AND
acknowledged (Non confirmés 0).

## Root cause (console-side, one line of SQL)

`latestEventStateHashes` in `creaves-console/actions/sync_checksum.go` extracted the
per-animal state hash from the stored event payload with
`json_extract(payload, '$.state_hash')`.

- **MySQL 8** (dev/prod): `json_extract` returns a **JSON** value — string members come
  back as **quoted JSON text**: `"<hash>"`.
- **SQLite** (unit tests): `json_extract` returns an **SQL TEXT** value — unquoted: `<hash>`.

Consequences on MySQL only (tests stayed green):

1. `latest[hash]` (`"h1"`, quoted) never equalled `consolidated_animals.state_hash`
   (`h1`, raw) → **every animal unconfirmed** (Confirmés 0 / Non confirmés 10140).
2. The event-log fingerprint was built from `"<animal_id>|\"<hash>\""` lines →
   stored-set checksum diverged from the producer's expected checksum
   (reproduced byte-exact: the quoted-line digest of the real 10140-line set is
   exactly `sha256:a51ae571…`).
3. The producer-side ack path was unaffected (the webhook response echoes
   `payload.state_hash` parsed by Go's `encoding/json`, which unquotes), so the
   producer legitimately showed everything delivered & acknowledged — the two
   admin UIs disagreed about the same data.

## Fix

Replace the plain extraction with the unquoting `->>` operator
(`JSON_UNQUOTE(JSON_EXTRACT(...))` semantics), supported by **MySQL ≥5.7.13/8** and
**SQLite ≥3.38** (console bundles SQLite ≥3.45 via mattn/go-sqlite3 v1.14.42), so both
dialects return the raw hash:

```sql
SELECT animal_id, state_hash FROM (
    SELECT animal_id,
        payload ->> '$.state_hash' AS state_hash,
        ROW_NUMBER() OVER (PARTITION BY animal_id ORDER BY created_at DESC) AS rn
    FROM event_streams
    WHERE instance_id = ? AND event_type = ?
        AND json_valid(payload)
        AND payload ->> '$.state_hash' IS NOT NULL
        AND payload ->> '$.state_hash' <> ''
) t WHERE rn = 1
```

Legacy events without `state_hash` (pre-backfill rows) are explicitly excluded from
the fingerprint; a hash-less event can neither confirm an animal nor shadow its latest
hashed state event. Producer side needed **no change** (its bookkeeping was correct).

## Tests

`creaves-console/actions/sync_checksum_test.go`:

- `TestLatestEventStateHashes_YieldsUnquotedHashes` — pins the raw-extraction contract
  (the regression that shipped the incident) and the resulting confirmation count.
- `TestInstanceSyncStatus_LegacyEventsWithoutStateHash` — legacy hash-less state events
  are excluded from the fingerprint, do not shadow hashed events, and animals known
  only through them stay expected-but-unconfirmed.
- Existing golden vectors (`TestStateSetChecksum_GoldenAndDeterminism`), ack-path and
  legacy-payload-refresh webhook tests unchanged and green.

```
CGO_ENABLED=1 go test -tags sqlite -count=1 -race ./...
→ ok creaves-console/actions 65.5s · ok creaves-console/excel · ok creaves-console/models
go vet ./... → clean · staticcheck ./actions/ → clean
```

## E2E evidence (dev pair, agent-browser)

Before the fix, live `/sync_management` reproduced the incident exactly
(Confirmés 0, `sha256:a51ae571…` vs `sha256:5f7d9b25…`, "checksum mismatch").

After rebuilding with the fix (buffalo dev auto-rebuild) and running a **force full
resync** on the producer (10141 animals — one new since the previous run):

- Producer `http://127.0.0.1:3000/webhook_resync`:
  `Expected animals: 10141 · Delivered & current: 10141 · Unconfirmed: 0 (0 never synced)`
  `Expected checksum: sha256:037bf499a452bf21c47327167d4e1cdc522c4c273f3eb727ec7e90fa21985571`
- Console `http://127.0.0.1:3001/sync_management` (instance dev):
  `Stored 10141 · Expected 10141 · Received 10141 · Confirmed 10141 · Unconfirmed 0`
  stored checksum `sha256:037bf499a452bf21c47327167d4e1cdc522c4c273f3eb727ec7e90fa21985571`
  = announced → **"checksum match · matches producer"**.

Resync run: `status.json` → `completed 10141/10141, events_delivered 10141, events_failed 0`.
