# Care plan / animal page UX — Round 7 bugs

Live list for the round-7 feedback (2026-10-02). Status is marked per item;
everything marked **Done** is implemented, unit/render-tested and green —
what is still open is listed as **Open** with its remaining step.

Companion fix plan: `docs/care-plan-round-7-fix-plan.md`.

---

## R4-7.1 — Apply modal on the animal Protocol tab greys out the whole screen (**Done**)

**Where**: `/animals/10317?back=…#nav-plan` (any animal, any tab but the
Treatment one).

Clicking the green check of a feeding row opened the shared apply modal —
but the modal markup was rendered **inside the `#nav-treatment` tab pane**
(`templates/animals/show.plush.html`, `partial("care_plan/apply_toggle…")`).
From the Protocol tab that pane is `display:none`, so Bootstrap appended its
backdrop to `<body>` while the modal itself stayed invisible and unfocusable:
the screen greyed out and **nothing could be clicked** — no cancel, no
escape, no close.

**Fix**: both shared partials (`care_plan/plan_med_toggle`,
`care_plan/apply_toggle`) now render at **page level**, next to the plan
editor, outside every tab pane. Their listeners are delegated on
`document`, so DOM position is irrelevant. Applied in all four locale forks.

**Test**: `TestAnimalShowModalsOutsideTabPanes` walks the `<div>`/`</div>`
nesting of every locale fork and asserts both partials fall outside **every**
tab pane, not merely after the last one — this page legitimately has panes
(media/outtake/audit) after the modals, so "after the last pane" was the
wrong invariant.

---

## R4-7.2 — Dead "missed" rows flood the day; the count belongs on the last row (**Done**)

**Where**: `/animals/…#nav-plan`, day cards.

A day with several feeding times showed every occurrence: the ones already
past due with nothing recorded rendered as dead rows with no apply button —
"only one apply" visible for a day that had four. The caregiver lost the
count of what was missed.

**Fix** (`actions/care_plan_animal_page.go`): an occurrence in status
`missing` **inside the last 24 h** leaves `day.Items` and only raises
`day.MissingCount` (capped at 5, `missingCountCap`). Older misses stay as
history. The template renders ONE pill beside the **last** remaining row's
description (`care_plan.animal_plans.missing`, `…missing_hint`, 4 locales).

---

## R4-7.3 — "Applicable protocol" and "Details" as two collapsed cards at the bottom (**Done**)

**Where**: `/animals/…#nav-plan`.

Two stacked collapsibles at the bottom of the tab answered one question
("why does this animal get this?").

**Fix**: ONE collapsed card **on top** of the tab merges the R4-6
traceability table (`#planTraceTable`, kept byte-identical for the a11y and
test surface) and the R3-7 protocol definitions table
(`#careAnimalPlansTable`). Collapsed by default; header keeps the source
count badge.

---

## R4-7.4 — The kind pill is glued to the red "late" border (**Done**)

**Where**: `/animals/…#nav-plan`, late rows.

`.plan-item-late` draws a 3px red left border; the row's first child (the
`nourriture` / kind pill) touched it and read as part of the border.

**Fix**: `assets/css/care-plan.scss` — `padding-left: 0.65rem` on
`.plan-item-late`.

---

## R4-7.5 — Per-animal check is noise for a lone animal; counter pill unreadable (**Done**)

**Where**: `/care_plan` (feeding cards, cage × diet).

With one animal in the cage the per-animal check duplicates the group
check; the corner counter badge (`badge-light`) was nearly invisible on the
row background; removing/adding the per-animal button shifted the row.

**Fix**: per-chip check buttons render only when `len(fcard.Chips) > 1`;
otherwise an invisible `.plan-apply-space` spacer keeps the footprint, so
the table columns stay aligned either way. The counter badge is
`badge-secondary` (darker) and renders only when the card holds several
animals. `aria-label` still carries the count (R4-4.2 a11y guarantee kept).
**Test**: `TestFeedingSingleAnimalGroupOnly` (new) + `TestFeedingSectionRender`
(extended) over all four forks.

---

## R4-7.6 — "Late" without a time: the caregiver cannot judge urgency (**Done**)

**Where**: `/care_plan`, feeding chips.

**Fix**: `FeedingChip` carries `DueHM` / `DueDayKey` / `DueShortDate`
(`feedingChipOf`), rendered as a muted `.plan-chip-due` span next to the
animal label.

---

## R4-7.11 — A green apply button reads as "already done" (**Done**)

Green (`btn-success` / `btn-outline-success`) is the **applied** colour in
this UI, so an apply affordance painted green looks recorded.

**Fix**: every apply control is now `btn-outline-secondary` (white
background, grey text) — `plan-apply-btn`, `plan-feeding-one`,
`plan-feeding-apply`, `plan-cage-apply`, in all forks. Terminal *applied*
states keep the green (`plan-med-*` slots, R4-1.1) and the modal **confirm**
buttons keep it too: confirming an action is not a state.

---

## R4-7.10 — `care_plan.status.done` rendered raw (**Done**)

`templates/animals/show.plush.*` uses `care_plan.status.done`, which was
never defined in any locale file, so the literal key was displayed.

**Fix**: `care_plan.status.done` added to all four `locales/care_plan.*.yaml`
(audit: every `care_plan.status.*` key used by a template now exists in all
four locales).

---

## R4-7.7 — Compact medication shows tomorrow all day long (**Done**)

**Where**: `/care_plan?view=compact&kind=medication`.

The work screen listed every slot of the schedule, tomorrow included, so it
was never empty and tomorrow's buttons competed with today's.

**Fix**: `scopeSeriesToToday` (`actions/care_plan_viewmodel.go`) keeps every
slot due up to the end of today — open ones and the terminal ones that are
the day's record — plus **at most one** later open slot, the nearest, and
only when it beats the pending late entry ("if the duration from now to the
entry is shorter than the current one"). The survivor is rebucketed into a
dedicated `tomorrow` group, so it follows morning/noon/evening like any
other bucket instead of masquerading as part of today. With the day's work
done at 22:00 the medication view is empty.

`care_plan.slot.tomorrow` was translated in en-us/fr only — de/nl rendered
the raw key; both were added ("Morgen").

**Test**: `TestScopeSeriesToToday{DropsTomorrow,KeepsNearerNext,EmptyInEvening,KeepsAtMostOneLaterSlot}`,
`TestFillMedTiersDropsScopedOutSeries`, `TestTomorrowSlotLabelInAllLocales`
(`actions/care_plan_round7_med_scope_test.go`). The "exactly equal" case is a
real boundary: a 1 s shift in the comparison flips it.

---

## R4-7.8 — Button alignment in the medication table (**Done**)

**Where**: the medication series table, every slot row.

**Fix** (`assets/css/care-plan.scss`): `.plan-med-cell` gets a fixed
`min-width` and right-aligns its content, so every button's left edge lands
on the same x and the times read as a column instead of a ragged wrap.
`.plan-med-bucket` reserves a `min-height`, so the buttons after every
bucket caption start at the same y and the day groups read as aligned
bands.

`public/` is gitignored and rebuilt by webpack, so the `.scss` is the
artefact of record.

**Test**: `TestCarePlanStylesheetAlignment` pins the rules on the source, and
the stylesheet was compiled with `sass` to confirm the output.

---

## R4-7.9 — Hamburger shown while there is still room (**Done**)

**Where**: the application navbar, every authenticated page.

`navbar-expand-lg` collapsed the bar below 992px although the icon-only bar
fits comfortably there — the space was simply wasted.

**Fix**: the breakpoint moved to `md`, and the **two thresholds that mirror
it** moved with it, or the bar would lie about its own state: the CSS query
that keeps labels visible while the menu is collapsed (767.98px) and the JS
overflow guard that only probes an expanded bar (768). Below md the labels
stay force-visible (in the collapsed vertical menu they cost no width);
between md and xl the bar is icon-only; at xl+ the pre-existing *measured*
overflow probe still hides the labels in a verbose locale. All four locale
forks.

**Test**: `TestNavbarBreakpointsAgree` asserts the Bootstrap class, both CSS
thresholds and the JS guard agree, per fork — changing the Bootstrap class
alone now fails it.

---

## R4-7.11b — Link an activity to its logged item (**Done**)

**Where**: `/animals/{id}#nav-plan`, every applied row.

`CardView.FulfillmentLink` was already computed on the animal page but never
rendered, so an applied feeding/observation was a dead end.

**Fix**: the green "done" badge doubles as the door to the record that was
actually logged — same badge shape and colour, only clickable, never a dead
link (a deleted or absent fulfillment keeps the plain badge). The link
carries `back=/animals/{id}#nav-plan`, so the record's own back button
lands on the source.

That exposed an asymmetry: `Treatments.Show` honoured `?back=` (sanitized,
with a label naming the real destination) but `Cares.Show` ignored it and
its template read `params["back"]` **raw** — an unsanitized redirect plus a
hardcoded "Back" that lied about where the button went. `Cares.Show` now
mirrors `Treatments.Show`.

**Test**: `TestAnimalAppliedRowLinksToFulfillment`,
`TestAnimalDayCardBackTargetsPlanTab`, `TestCareShowUsesSanitizedBack`,
`TestViewRecordLabelInAllLocales` (`actions/care_plan_round7_fulfillment_link_test.go`).

---

## Open items

### R4-7.12 — Protocol trace repeats what the row already shows (**Done**)

The `{count} occurrence(s)` column added nothing once the schedule and
content were visible; it now carries edit/delete per row. Plan:
§R4-7.12 of `docs/care-plan-round-7-fix-plan.md`.

### R4-7.13 — Per-animal exception to a global protocol (**Open**)

An animal-specific "skip this protocol" exception, listed as an exception on
the animal and suppressed-but-still-traceable in the plan. Semantics agreed:
settable by anyone who can already edit the animal, permanent until removed,
logging who set it and why. Needs an additive table + engine rule (no
destructive migration — see the session constraint).

### R4-7.14 — Compact feeding rows (**Done** — 5/5)

`/care_plan?view=compact&kind=feeding`:

- late rows are **white**, not red — `.plan-tier-body { background: #fff }`
  paints the body, so the section's red only reaches the header strip.
- rows are ordered by **cage name**, not by what must be executed first.
- the global checkmark is left-aligned in its cell, not centred.
- the expected **time repeats on every animal chip** (`10100 16:00`,
  `10101 16:00`, …) instead of once per group.
- with more than one animal there is no collapse — the buttons are always
  expanded.

Measured before: `rowBg=rgba(0,0,0,0)` inside `bodyBg=rgb(255,255,255)`;
`applyBtn offsetInCell=10.5` of a 63px cell; chips `[2003/26 16:00 |
2004/26 16:00]`. Plan: §R4-7.14.

**Parts 1-3 fixed.** The other two findings were re-measured rather than
assumed:

- **1 (red late rows)** — `.plan-tier-body { background: #fff }` forced the
  body white, so the tier's red reached only the border. The body and its
  cells are now transparent and the tint shows: `effectiveRowBg`
  `rgb(255,255,255)` → `rgb(248,215,218)`. The **late group dot was also
  amber** `#e0a800` — the same colour as the "due now" tier next to it — so it
  is now red `#dc3545`, matching `.plan-dot-missing`.
- **2 (execution order)** — `fillFeedTiers` appended rows in cage order and
  never sorted. Now sorted by the group's EARLIEST open occurrence, cage label
  as tie-break so the auto-refresh does not reshuffle equal times. After:
  `ASCENDING_BY_EARLIEST_DUE=true unsortedKeys=0`.
- **3 (centred check)** — the check was already horizontally centred
  (10.5 px / 10.5 px in a 63 px cell); what was wrong is **vertical**: the cell
  grows with the animal list, so on a cage with 47 chips the button sat 10.5 px
  from the top of a 321 px cell and 280.5 px from the bottom — reading as if it
  belonged to the first animal. `.plan-feed-check` centres it: `vTop=145.5
  vBot=145.5`.

**Parts 4-5 fixed.** Both were re-measured in the live DOM before any edit:

- **4 (repeated time)** — the time was per-*chip* markup, so a cage of six
  animals fed at `08:00` printed `08:00` six times: **351 chips for 167 distinct
  (day, time) pairs**, the worst row carrying `WASTED_TIME_LABELS=5`. The chip's
  `plan-chip-due` span is gone; the time is now the **sub-group heading**.
  `foldChipsByTime` keys on `day|short-date|time` — so `16:00 today` and `16:00
  tomorrow` stay two sub-groups rather than merging — and keeps first-seen
  order, which makes group 0 the earliest by construction. After:
  `chipsStillCarryingOwnTime=0`, one `plan-time-label` per sub-group
  (`timeLabels=167`), and the page shrank `21476 → 12784 px`.
- **5 (no collapse)** — **29 of 162** rows hold several animals (133 hold one);
  all were expanded, the largest **321 px** tall. A row is collapsible only when
  `AnimalCount > 1`, so a single-animal row never gains a header to open. It
  renders **closed by default** (`aria-expanded="false"`, `class="collapse"`),
  keyboard-operable like the tier headers, and the collapsed header still
  carries the count badge plus `from 08:00` — so collapsing tidies the screen
  without hiding what is due. `COLLAPSED=29 EXPANDED=0`, `noHeader=133`, and
  `GROUPCHECK vTop=10.5 vBot=10.5 cellH=51` (was 321) — the check is centred
  again for the same reason as part 3.

R4-7.6's per-chip `plan-chip-due` assertion in `TestFeedingSectionRender` was
**retargeted, not deleted**: its intent (the time is visible) is now asserted as
one label per sub-group, plus a `NotContains` that no animal repeats a time.

### R4-7.15 — Compact medication animal column is ragged (**Open**)

`/care_plan?view=compact&kind=medication`: the animal cell has no width and no
background, so labels stop reading as a column. Measured: distinct widths
`167.9 … 180.8px`, `bg=rgba(0,0,0,0)`. Plan: §R4-7.15.

### R4-7.16 — Compact observation does not follow the medication format (**Open**)

`kind=observation` (and `care`, `weighing`) still render a 4-cell table while
medication renders a line, and the row tint exists only on late items. Plan:
§R4-7.16 — default taken: convert all three, tint every item.

### R4-7.17 — Raw HTML entities in the confirm modal (**Open**)

The observation confirmation shows `La réponse à l&#39;observation…`. Every
translation interpolated into a `<script>` block is HTML-escaped by Plush, and
script text never decodes entities. Also a **script-injection risk**: a
translation containing `"` terminates the JS string literal. Plan: §R4-7.17.

### R4-7.18 — compact and detailed look identical (**Done**)

Measured across every kind: compact and detailed differ **only** for
`observation` (13 vs 23 rows); care, weighing, cleanup, medication and feeding
differ only in the toggle's own active state. The toggle is removed and the
handler pins a single density; `?view=detailed` still resolves and renders that
same view. Verified identical row counts for both parameters on every kind.
Plan: §R4-7.18.

### R4-7.19 — Protocol trace duplicates the definitions table (**Done**)

On the animal Protocol tab the same protocol is listed **twice, stacked** —
trace row and definitions row — with the same name, content and schedule; and
the name itself repeats its own detail. Also: the "Protocole de l'animal" badge
should say the protocol is set at animal level (with a tooltip), and the trace
should carry the action kind. Removing the second table needs one confirmation
first (it is the only place inactive/expired protocols are listed). Plan:
§R4-7.19.

**Resolved** (commit R4-7.22). The single confirmation asked for was measured
first, and it changed the fix: on animals/10221 **two** protocols existed ONLY
in the definitions table (one of them inactive), so deleting it outright would
have made them unreachable. Three things therefore moved into the trace row
before the table went: the protocol TYPE (`src.Kind`), the Active/Expired/…
status, and the ISO window. `ProtocolSourceView` gained `Active` for the status
(rendered for **every** animal protocol, not just the exceptions — now that
idle protocols are listed, "no badge" would be ambiguous between
active-but-not-due and switched-off). The badge reads "Dedicated" / "Spécifique"
/ "Spezifisch" / "Specifiek" with a tooltip explaining it is set at this
animal's level. The delegated CRUD script moved its `data-url`/`data-animal-id`
anchor to `#planDetails` — NOT to `#planTraceTable`, which renders only when
the trace is non-empty and would have killed the "New protocol" button on an
animal with no protocol.

### R4-7.20 — Protocol names truncated mid-word (**Open**)

`care_plan_convert_data.go:230` slices the name by **bytes** at 60, so
`…à côté de la nourriture…` is stored as `…à cô` — cut inside a word (and able
to split a multi-byte rune). Generator fixed to truncate on a rune and word
boundary; existing rows left untouched (no destructive DB change). Plan:
§R4-7.20.

### R4-7.21 — Counts do not all mean the same thing (**Open**)

On the compact feeding screen the summary strip counts **occurrences** while
the tier badge counts **groups**, and both saturate at `99+`, so `162` and
`351` render identically. Measured: `headerBadge=99+ ROWS=162
CHIP_OCCURRENCES=351`. Direction given: a badge counts **what is visible in
its section** — groups everywhere, one unit, one meaning. Plan: §R4-7.21.

### R4-7.22 — The same protocol sentence is rendered four times (**Done**)

`/animals/{id}#nav-plan`, measured on animal 10312 (fr): the diet sentence
appears **3–4×** on one screen — trace name cell, trace content cell,
definitions-table name cell, definitions-table content cell.

Root cause chain, each step independently sufficient:

1. `care_plan_convert_data.go:230` truncates the stored name by **bytes**
   (`title[:60]`) → `…à cô`, cut mid-word (R4-7.20);
2. `care_plan_humanize.go:147` then asks
   `strings.Contains(name, content)` to decide whether to append the content
   — a truncated name can never contain the full content, so the check fails
   and the content is appended **again**, producing
   `Alimentation — …à cô — …retirer le soir`;
3. two surfaces render name **and** content, so 2 code paths × 2 cells = the
   four copies.

Fixing the truncation alone does not repair the 3 existing rows, so the display
must also become tolerant of a name that merely *starts with* the content —
that removes the duplication on existing data with **no data migration**.

Target format (given): `<type>  <complete description>  [<buttons for the
applicable hours>]`, following the care_plan medication rules; two protocols
with the same description but different hours are two lines, each with its own
hours. Plan: §R4-7.22.

**Resolved** (same commit). The root cause is fixed at step 3 — the
definitions table is gone, so there is one surface instead of two, and the
trace row renders the description in exactly one cell (`richPlanName`'s
re-append is no longer used there). Steps 1–2 were already fixed by R4-7.24,
which stores whole words; no data migration was needed either way.

Measured on animals/10312 (all four locales), after the merge:

```
definitionsTableStillPresent=false   traceTablePresent=true
rowsMissingType=0   rowsMissingStatusAndWindow=0
rowsWithContentTwice=0   REPEATED_CELLS={}
scopeTooltips=2   hasDedicatedLabel=true
crudAnchor=true url=/animals/10312/care_animal_plans   newBtnWired=true
```

On animals/10221 the trace now lists **9** rows including the two that were
previously unreachable ("Traitement — Baycox 5 % (PER OS)", "Traitement —
Contrôle prolongation TRT"), each with its own type and window.

A measurement trap worth recording: the duplication count was first read off
`innerText`, which also returns the text of **collapsed** accordions — 21 hidden
copies of the same feeding sentence (7 days × 3 times) inflated the figure. Every
`#acp-*` day card on that animal is collapsed, so `VISIBLE_OCCURRENCES_OF_DESC=0`.
Duplication must be counted on **visible** text (`offsetParent !== null`) only.

### R4-7.23 — Treatment tab: day badge over an empty body (**Open**)

`/animals/10312?back=#nav-treatment`: six day cards, each header showing an
open-count badge, each body **empty** — a count of "3" over nothing. The
caregiver is told there is work and given none.

Cause: the day badge counts medication slots **and** the non-medication items
(`care_plan_animal_page.go:377-386`), but the card body renders only
`partial("care_plan/med_series.plush.html")` from `day.Group.Series`. For an
animal whose work is feeding/observation/care the count is non-zero while the
series list is empty. The animal's own Protocol tab (`#nav-plan`) *does* render
those items — the two tabs disagree about the same days.

Measured (10312): 6 collapses, every body `<div class="med-series
flex-grow-1"></div>`, headers `07/10·3 06/10·3 05/10·3 04/10·3 03/10·3
02/10·2`.

Fix: render the non-medication items in the Treatment tab too (one line per
item, same language as the medication series), so the badge and the body always
describe the same set. Plan: §R4-7.23.

### R4-7.24 — A description must never be cut (cross-cutting, critical) (**Done**)

Stated as a hard rule after the `…à cô` truncation (R4-7.20) was found a
third time in the UI. No page may render a **cut description**: not mid-word,
not mid-sentence, not with a silent truncation.

Audited truncation sites:
- `care_plan_convert_data.go:220` `%.60s`, `:231` and `:310` `title[:60]` — the
  real offenders (R4-7.20). The column is `varchar(200)`, so the cap is not a
  storage necessity.
- `guest.go:621` `number[:20]` — a **lookup key**, never displayed; not a
  description. Out of scope, documented so it is not "fixed" by mistake.
- `configs.go:393` `uuid[:8]`, `guest.go:115/145` `hits[:0]` — not truncation.
- CSS: only `.autocomplete-suggestion` uses `text-overflow: ellipsis`
  (a type-ahead dropdown — expected).

Fix: a shared, rune-safe, word-boundary truncator used by every name/content
writer; and where a description is too long for a cell it must **wrap or be
reachable in full**, never silently cut.

**Fixed.** `truncateWords(s, max)` in `actions/text_truncate.go` — counts RUNES
(to match `varchar(200)`, which MySQL counts in characters), cuts back to the
last space, trims the dangling space, marks the cut with `…`, and falls back to
a hard rune cut when there is no word boundary (returning it over-long would
overflow the column, which is the original defect). All three offending sites now
route through `convertedFeedingName` / `convertedMatcherName`.

Proven end-to-end against the test DB: reverting just the per-animal plan site
to the byte slice stores `Alimentation — nourriture 6ba7da04 spéciale pour
animaux malades avec un r (conversion)` — cut mid-word, and silently, with no
marker at all.

Two things worth recording:
- `richPlanName` was **already** tolerant (t21 flagged it as a risk): a truncated
  name no longer `Contains` the payload content, so the full text is appended
  and the caregiver still reads the complete description. Pinned by
  `TestTruncatedNameStillShowsTheCompleteContent`.
- **Existing rows are untouched.** Two names in the dev DB were already cut by
  the old code (`…40 souri`, `…poussin moulu et`); rewriting stored data is
  outside the no-destructive-change rule, so the fix prevents new cuts only.

Audit re-run: no CSS `text-overflow` on any description-bearing element, and the
only other Go slices are a lookup key (`guest.go` `number[:20]`, never displayed)
and uuid/marker prefixes.

### R4-7.25 — A ✓ marks an item that is still **to do** (**Done**)

> "instead of using a check mark for item to do - use the same as treatment
> `treatments/062b233c-…?back=%2Fcare_plan%3Fkind%3Dmedication`"

Measured on the live pages:

| page | marker for an open (to-do) item |
|---|---|
| `/treatments/:id` | `<span class="badge badge-warning"><i class="far fa-clock"></i> To do</span>` — a **clock**, amber |
| `/treatments/:id`, already done | `<span class="badge badge-success"><i class="fas fa-check"></i> Done</span>` |
| `/care_plan` (every kind) | `<button class="… plan-apply-btn"><i class="fas fa-check"></i></button>` — a **solid check** |

Both pages end up applying the very same treatment, so the same object is called
"Done" on one page and "to do" on the other. The ✓ is the app-wide **completed**
glyph: it is the dashboard's "Done" submit, the todos' "Fait / Erledigt /
Klaar", `drugs`/`cares` checkboxes, and the treatment page's own *done* badge.
Putting it on a button whose job is to *perform* the item means the user reads
the button's face as the item's state, and gets it backwards — the row looks
finished before anything was recorded.

FontAwesome is genuinely loaded (`/assets/application.*.css` + the i2svg
runtime, which swaps the `<i>` for an inline `<svg class="svg-inline--fa
fa-check">`), so this is a wrong-glyph bug, not a missing-icon bug: the check is
drawn at 12.45×16 px inside a 32 px button.

Fix: adopt the treatment page's language verbatim — **open item = clock**
(`far fa-clock`), **done item = check** (`fas fa-check`). Applied to every
care-plan "record this" control, in all four locales:

- `templates/care_plan/index.plush*.html` — 6 × `fas fa-check`
  (`.plan-apply-btn` ×3, `.plan-feeding-one`, `.plan-feeding-apply`,
  `.plan-cage-apply`)
- `templates/animals/show.plush*.html` — 1 × `fas fa-check` (`.plan-apply-btn`)

Deliberately **not** touched:
- `_med_series.plush.html` — its `○ HH:MM` (open) / `✓ HH:MM` (green, applied)
  already uses check = done. It becomes *more* consistent with the change.
- `animals/show.plush.html:777` — `fa-check`/`fa-times` answer "does this
  protocol replace the kind?", a yes/no fact, not a to-do.
- the green `btn-success` treatment modal/slot buttons — green = applied, and
  `TestSlotTierClass` pins it.

Plan: §R4-7.25.

Fixed: `fas fa-check` → `far fa-clock` on the 7 to-do controls per locale fork
(28 replacements, whole-line and count-asserted). The language is now one
glyph: **clock = to do, check = done**, on both pages.

The swap also removed a latent defect: R4-7.5's invisible `plan-apply-space`
spacer is 34 px while the control it stands in for was 32 px, so a cage gaining
or losing a co-diner shifted the row by 2 px. `min-width: 2.1rem` on the four
to-do controls makes them agree — and pins the width against the clock being
0.89 px narrower than the check it replaced.

Verified in en-US/fr/de/nl on `/care_plan?kind=feeding|observation|medication`
and `/animals/10312?back=#nav-plan`: `ANY_CHECK_ON_TODO=0` everywhere, widths
`{"34":218,"42":162}` on feeding, and the treatment page's localized pair
intact (`To do`/`À faire`/`Zu erledigen`/`Te doen` = clock, `Done`/`Fait`/
`Erledigt`/`Gedaan` = check). Gates clean; the 7 `actions` failures are the
identical pre-existing set. Evidence:
`tmp/browser_evidence/round7/e2e-round7-part3-todo-glyph.md`.

### TEST-1 — `TestAnimalSearchFiltersANDCombined` flakes on fixture collision (**Open**, test-infra, pre-existing)

Not a product defect. The animal-search fixture derives its unique
`(year, yearNumber)` from `MAX(yearNumber)+1` plus a hash (`animals_test.go:93`).
`sweepStaleFixtureRows` only deletes animals with `cage LIKE 'CP-%'/'OTHER-%'`,
but the search fixtures are written with `cage = NULL`, so they accumulate. As
`MAX(yearNumber)` grows each run, the hashed offset can land on an already-taken
`(year, yearNumber)` and the fixture insert fails with
`Error 1062 Duplicate entry … animals.animals_year_yearNumber_idx`.

Observed during R4-7.14b/c gating: the **same** feature code passed the test in
two consecutive full `-race` runs and failed in a third; it also passes 3/3 in
isolation and 3/3 in its `-race` group, and passed on HEAD in a full run. So it
is a flaky isolation gap, not a regression — but it intermittently adds a 8th
failure and pollutes the gate diff.

Fix (separate, test-infra): extend the sweep to drop `NULL`-cage search fixtures
(e.g. `species LIKE 'Testsp %'`), or make the yearNumber derivation provably
collision-free. Not fixed here to keep this commit to the feeding UI change.

### TEST-2 — `TestMigrateTreatmentTimesIdempotent` fails on a stale test-DB FK (**Done**, test-infra)

Not a product defect and not caused by R4-7.22. The test inserts a `treatments`
row for an animal that no longer exists in `creaves_test`:

```
Error 1452 (23000): Cannot add or update a child row: a foreign key constraint
fails (`creaves_test`.`treatments`, CONSTRAINT `treatments_animals_id_fk`
FOREIGN KEY (`animal_id`) REFERENCES `animals` (`id`))
```

Proven pre-existing during R4-7.22 gating: it fails identically on stashed HEAD
(`6723739`), in isolation 3/3, and as part of the whole `models` package. The
R4-7.22 diff touches no file under `models/`. Unlike TEST-1 this one is
deterministic, not flaky — it is a permanently red gate on this machine.

**Fixed.** `migrationTreatment` hardcoded `AnimalID: 42`; a new
`migrationHostAnimalID(t)` helper resolves a legal animal instead. It reuses an
existing animal when the test database has one (the normal case — `creaves_test`
is never reset between runs), and otherwise builds the minimum chain an animal
requires, cleaning it up afterwards. Two schema facts the create branch has to
respect, both found by forcing that branch and watching it fail:

- `animals.IntakeDate` is NOT NULL with no default — unset, it sends the zero
  time and MySQL rejects `'0000-00-00'`;
- `animals` carries `UNIQUE(year, yearNumber)` and the zero pair is already
  taken by the seed data (`Duplicate entry '0-0'`), so both are derived from
  the random uuid bytes — the same collision class as TEST-1.

Verified both branches: the reuse path passes 3/3 under `-race`, and the create
path was proven by temporarily short-circuiting the reuse branch to `&& false`
so the fixture chain actually executed. Pure unit tests (`migrationEntries`)
short-circuit on `DB == nil` and never touch the database.
