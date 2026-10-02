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

### R4-7.14 — Compact feeding rows (**Open**)

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

### R4-7.19 — Protocol trace duplicates the definitions table (**Open**)

On the animal Protocol tab the same protocol is listed **twice, stacked** —
trace row and definitions row — with the same name, content and schedule; and
the name itself repeats its own detail. Also: the "Protocole de l'animal" badge
should say the protocol is set at animal level (with a tooltip), and the trace
should carry the action kind. Removing the second table needs one confirmation
first (it is the only place inactive/expired protocols are listed). Plan:
§R4-7.19.

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

### R4-7.22 — The same protocol sentence is rendered four times (**Open**)

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

### R4-7.24 — A description must never be cut (cross-cutting, critical) (**Open**)

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
