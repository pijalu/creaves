# Resync runbook — backfilling v2 payload fields on the console

Audience: console/creaves administrators. Required reading when a Creaves
center was connected to the console **before** payload v2 (animal age, type,
entry cause, outtake type/rating, translations) or whenever the console looks
out of date for one center.

## Symptom

On the console dashboard / reports, animals from one center show:

- `Unknown` buckets in the age / type / entry-cause / outtake-type tables,
- NULL `animal_age`, `animal_type`, `entry_cause`, `outtake_*`, `translations`
  columns in `consolidated_animals`,

while the Creaves source app has the data.

Cause: those rows were created from v1 events that did not carry the fields.
The console never guesses missing data — it needs the center to re-send its
full current state. That is what a **resync** does.

## What a resync does

1. Creaves walks **all** animals and enqueues one deterministic
   `animal_state` event per animal (full current snapshot, v2 payload with
   translations).
2. Events whose content hash is unchanged since the last known state are
   skipped (dedup), so resyncs are cheap to repeat.
3. The webhook pusher delivers pending events to the console every 5 s with
   retry; the console upserts one row per animal.

A resync never deletes console rows and never sends personal/discoverer data —
the payload contract is unchanged, only more complete.

## Procedure (per center)

On the **Creaves** app of the center (not the console):

1. Log in as admin.
2. Open `/webhook_resync` (menu: Administration → Webhook resync).
3. Click **Start resync**. The page polls status every 2 s.
4. Wait for status `completed` (`animals_processed = total_animals`).
   - `events_skipped_unchanged` high is normal on a re-run.
   - `errors` lists animals whose payload could not be built — investigate
     those animals individually (usually broken associations).
5. Within ~10 s of completion, the console reflects the new state. Verify on
   the console: filter `/consolidated_animals?instance_id=<center>` — the
   `Unknown` buckets in `/reports/annual` for that center should be gone.

No console-side action is needed; the receiver is idempotent.

## If a resync seems stuck

- Status stays `running` with `animals_processed = 0`: check the Creaves app
  log (`buffalo dev` console or your log file). A resync worker runs on a
  background goroutine; if the app was restarted mid-run, the run is marked
  `failed (interrupted by restart)` on boot — just start a new one.
- Status `failed`: the `errors` column tells you why. Fix and re-run; resyncs
  are idempotent.
- Events created but console unchanged: the pusher delivers every 5 s and
  retries forever. Check the console is reachable from the Creaves host and
  the webhook key matches (Administration → config on Creaves vs the center's
  API key on the console).

## After an app upgrade that changes the payload

When a Creaves/console upgrade adds fields to the payload (like v2 did),
**every connected center should run one resync** after upgrading. Until then
its old rows keep showing `Unknown` in the new report columns — this is
expected, not data loss.

## Automated verification

`e2e/run.sh` (see `.agents/skills/e2e-testing/SKILL.md`) exercises this whole
path: it wipes console instance A, runs a resync through the real UI + webhook
pipeline, and asserts all 9 fixture rows come back with v2 columns and
translations populated.
