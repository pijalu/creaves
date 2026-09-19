# 2026-10-08 — Critical review of `feature/open-issues-2026-10` (BUG-R1..R7)

Archived from `bugs.md` after all items were fixed/closed. Process: per-commit
review of the 28 commits after `fix/review-findings`, fixes with unit tests,
e2e evidence via agent-browser, and quality gates per repo.

## Critical review — all fixes since `fix/review-findings` (creaves) — DONE 2026-10-08
Reviewed 28 commits (`git log fix/review-findings..HEAD`) per-commit + cross-cutting
(schema/migration safety, webhook contract, i18n, race/txn, authz). Findings → BUG-R1..R6 below.
Verified OK (no action): additive migrations, #100 `<=>` guard, f93557c Exists-cache fix,
guest RoleGuard passthrough, cage batch partial-failure semantics, attachment Serve ACL
consistent with app model (all roles view all animals). Baseline grifts failure unrelated.

---

## BUG-R1 — Attachment upload authorization gap (HIGH)
**Observed:** `creaves/actions/attachments.go:68-128` `AttachmentsCreate` comment claims
"Any authenticated user may upload on an animal still in care (admins on outtaken animals)"
but **no code checks `animal.OuttakeID` or admin** — only the template hides the form
(`templates/animals/show.plush.html:95`). `AttachmentsDestroy` (`attachments.go:181-245`)
has the same gap. (Correction during fix: Destroy DOES enforce owner/admin at
`attachments.go:209-212`; the multipart size is capped before reading — `f.Size` check at
line 142 + `io.LimitReader(max+1)` at line 187. Only Create's missing in-care check is real.)
**Expected:** server-side enforcement matching the comment.
**Fix plan:**
1. In Create after loading the animal: if `animal.OuttakeID.Valid` and current user not admin → danger flash + redirect to animal page.
2. Tests: new `actions/attachments_test.go` — non-admin create on outtaken animal → blocked (redirect+flash, no row); admin → allowed.
3. e2e (agent-browser, 4 locales where UI-visible): admin upload on in-care animal works; outtaken-animal form hidden.
4. Gates (each separately): go vet, staticcheck, gocognit -over 15, gocyclo -over 12, go test -count=1 -race -cover.
5. Commit: `fix: enforce attachment authorization on outtaken animals`.

## BUG-R2 — Orphaned attachments when animal is destroyed (HIGH)
**Observed:** `creaves/actions/animals.go:1017-1072` `AnimalsResource.Destroy` converts the
animal to an error-outtake (intakes/cares/attachments all survive) but **never deletes
`attachments`/`attachment_blobs`** (no FK attachments.animal_id → animals). Media stays in
DB forever and remains servable via `AttachmentsServe` (`attachments.go:191-200`).
**Expected:** destroying an animal removes its attachments + blobs.
**Fix plan:**
1. In Destroy txn: delete blobs for the animal's attachments, then attachments (raw SQL, codebase style).
2. Test: animal + attachment + blob → destroy → both tables empty for that animal.
3. e2e (agent-browser): upload attachment, destroy animal, attachment URL → 404.
4. Gates + commit: `fix: delete attachments and blobs when animal is destroyed`.

## BUG-R3 — Webhook payload gaps: new outtake fields never reach console (MEDIUM) — FIXED 2026-10-08
**Observed:** new fields `precise_location` (#197-4), `stay_duration` (#175),
`ready_for_release` (#197-8), `corpse_destination*` (#149) are absent from
`models/event_stream.go:147-160` `OuttakePayload` and `actions/event_producer.go:296-318`;
console `models/consolidated_animal.go` has no columns for them. Violates the workspace
"Keep Payload Format In Sync" rule; console silently loses data.
**Expected:** contract extended additively in both repos (old receivers ignore unknown fields).
**Scope refinement (during fix):** ReadyForRelease lives on Animal (not Outtake);
CorpseDestinationByID (internal user reference) excluded from the payload — console is an
anonymous consolidated read view; the mark handler is session-user based anyway.
**Fix plan:**
1. creaves: AnimalPayload += `ReadyForRelease *bool`; OuttakePayload += `PreciseLocation string` (omitempty), `StayDuration *int` (omitempty), `CorpseDestination string` (omitempty), `CorpseDestinationAt string` (omitempty); populate in buildAnimalPayload.
2. creaves-console: additive nullable columns on `consolidated_animals` (no prod data — free redesign, still additive); map in upsert; show on animal detail.
3. Tests: creaves event_producer payload test; console webhook handler test with new fields.
4. e2e: both apps running; create outtake with precise location + ready flag in creaves → visible in console.
5. Commit per repo: `feat: sync new outtake fields into webhook contract`.

**Resolution (2026-10-08):** contract extended additively both repos. creaves
`ba2ac5b` (payload structs + producer + test TestBuildEventPayloadExtendedOuttakeFieldsR3);
creaves-console commit (migration `20261008000000_add_extended_outtake_fields`, 5 nullable
columns, payload mirror, applyExtendedOuttakeFields helper, applyState clearing, 3 model
tests, SQLite test schema update, AGENTS.md both repos). Console dashboard display of the
new fields deliberately out of scope (storage + event payload only).
**e2e evidence:** creaves :3000 + console :3001, animal 4086 (2024/376) edited →
ready_for_release=1; outtake created via UI (Relacher, "Forêt de Robertsau", precise
"R3-E2E: 48.6200, 7.7900 clairière nord", date 2026/09/19 12:00 — form "now" fails the
future-date validation in dev because the picker value is compared as UTC vs local +0200,
pre-existing TZ quirk, unrelated). Events animal_released + animal_state delivered
(delivered_at set); console consolidated_animals row: status=released,
outtake_precise_location/stay_duration/ready_for_release stored; console event show page
`/events/6f32fe6f-...` renders `"ready_for_release": true`, `"stay_duration": 20667`,
`"precise_location": "R3-E2E: ..."` in the payload JSON.
**Gates (console):** go vet clean; staticcheck 2 pre-existing U1000; gocognit/gocyclo no
new hotspots (applyOuttake kept ≤12 via helper; UpdateFromPayload +1 on already-hot
baseline 59/58); CGO_ENABLED=1 go test -tags sqlite -count=1 -race -cover ./... all ok.

## BUG-R4 — SPW role inconsistencies in RoleGuard (MEDIUM)
**Observed:** `creaves/actions/role_guard.go:65-84` — SPW whitelist lacks `/attachments*`
and `/todos*` → SPW gets 403 on attachment media shown on animal pages they CAN view;
lecteur (GET-everywhere) is less restricted than SPW here — inconsistent.
**Expected:** SPW can GET attachment media and todos (read-only).
**Fix plan:**
1. Add `/attachments` and `/todos` prefixes to SPW GET whitelist.
2. Tests in `actions/role_guard_test.go`: SPW GET /attachments/1 and /todos allowed; SPW POST still blocked.
3. e2e (agent-browser): SPW user opens animal with attachment → image loads.
4. Gates + commit: `fix: allow SPW read access to attachments and todos`.

## BUG-R5 — Open redirect via unvalidated `back` param (LOW)
**Observed:** `c.Param("back")` used directly in redirects: `actions/duplicate_guard.go:61`,
`actions/attachments.go:231`, plus pre-existing uses in `cares.go`, `treatments.go`,
`veterinaryvisits.go`, `feeding.go`. Crafted `?back=https://evil.example` redirects after POST.
**Expected:** only local paths honored.
**Fix plan:**
1. Add `actions/safe_redirect.go`: helper returns param only if it starts with `/` and not `//`, else `/`.
2. Replace all `c.Param("back")` redirect uses.
3. Unit test for helper + one action test with malicious back param.
4. Gates + commit: `fix: validate back redirect parameter`.

## BUG-R6 — Missing tests for attachments lifecycle (LOW) — NOT A BUG (closed 2026-10-08)
**Observed (review claim):** #34 feature shipped with no tests — no `actions/attachments_test.go`, no
`models/attachment*_test.go`.
**Verification:** claim wrong — review grep was misinformed. Coverage exists:
`models/attachment_test.go` (TestAttachmentKind matrix, TestAttachmentValidate, TestSanitizeFilename),
`actions/attachments_test.go` (upload/serve/delete roundtrip, type+size rejection, delete ownership,
missing-blob 404, blob store/load/delete upsert) — extended by the R1/R2/R5 fixes with
TestAttachmentsCreateOuttakenAnimalAuthR1, TestAnimalDestroyRemovesAttachmentsR2,
TestAttachmentsRedirectBackParamSafeR5. **No fix required.**

## BUG-R7 — Dashboard force-feed preview per-animal queries (LOW, accepted)
`dashboard.go:141-166` runs per-animal treatment+care queries, capped at 5 animals.
Accepted as-is at current scale; no code change.

---

## Summary — priority order
| Order | Issue | Why |
|---|---|---|
| 1 | Critical review since fix/review-findings | 28 commits (15 issue fixes) never reviewed as a whole — regression/security risk |
| postponed | #141 | open on GitHub, assigned crealan — awaiting legal template spec; postponed per maintainer |
| removed | #117, #151 | cancelled per maintainer — not to be implemented |
| ignore | #123, #187, #195, #196 | instance provisioning (excluded per instruction) |

Archived (fixed + ticket closed/reassigned, see docs/archive/2026-09-19-issue-*.md):
#100, #88, #90, #96, #73, #175, #170, #158, #145, #107, #149, #150, #147, #197.
(#34 re-fixed 2026-10-08 — binaries moved to DB, see docs/archive/2026-10-08-issue-34-attachments-db-storage.md.)

Notes:
- TestLoadConfigMultipleConfigsCoexist (pre-existing failing test entry from the #100 session) passes again on `feature/open-issues-2026-10` — fixed en passant; details in the #197 archive note.
- Known baseline failure (pre-existing, unrelated to closed items): grifts TestMigrationsReplayOnEmptyDatabase — outtaketype seed name-collision ('Relacher' vs 'Relaché') under accent-insensitive collation.
