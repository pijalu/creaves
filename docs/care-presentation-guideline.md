# Care Presentation Guideline

Status: **normative**. Every care-plan surface (day plan, dashboard medication cell,
animal Treatment/Protocol tabs, history) MUST follow this contract. A deviation is a
defect — log it under `docs/defects/` with a reference to the violated section.

Origin: user review round of 2026-10 (defect log D1–D7). The medication line is the
reference implementation; this document generalizes it to every action kind
(feeding, medication, care, cleanup, weighing, observation).

---

## §1 Tier sections (Late / Now / Later / Done / History)

1. Open work is grouped into urgency tiers, always in this order:
   **Late → Now → Later**. Fully recorded work goes to **Done**; terminal and
   superseded records (applied, skipped, deferred, overridden, missed) go to
   **History**, last on the page.
2. Every tier — History included — is a bordered panel (`.plan-tier`) with:
   - a header carrying the tier name + a count pill + a collapse chevron;
   - a matching background tint (Bootstrap alert palette):
     Late `#f8d7da` red · Now `#fff3cd` yellow · Later `#d4edda` light green ·
     Done `#b7dfc2` darker green · History neutral grey;
   - a collapse body; Late/Now/Later default open, Done/History default closed.
3. A tier that has nothing to render does not render at all — no empty header
   under a "0" badge, and the summary strip shows no pill for it.
4. One unit, one number: every badge on a section counts the same thing the
   summary strip counts (open occurrences), so header and strip can never
   disagree.

## §2 Toggle-button colour semantics

1. Every actionable occurrence is a **toggle button** whose colour is its state:
   - **red** (`btn-danger`) — late / missed, still recordable;
   - **yellow** (`btn-warning`) — due now;
   - **white with border** (`btn-light border`) — scheduled / to do later;
   - **green** (`btn-success`) — done.
2. The colour change MUST be immediate on click — the button flips in place,
   no page reload, no waiting for the auto-refresh. Undo restores the colour
   the occurrence has *now* (its current tier), which may differ from the
   colour it had before applying.
3. Colour is additive, never the only signal: the glyph
   (`○` todo · `✓` done · `⊘` skipped · `⏸` deferred · `–` out of window) and
   the localized `title` stay, so state survives colour-blindness and
   screen readers (`sr-only` status text).
4. The tier→class mapping is defined ONCE, server-side
   (`slotTierClass` in `actions/care_plan_viewmodel.go`). Templates never
   pick a Bootstrap colour class for a toggle; they render the provided
   `TierClass`. The original tier class rides on the button
   (`data-tier-class`) so client-side undo can restore it.

## §3 One line per item — time grouping

1. Repeating occurrences of the same (animal-or-cage × item) merge into
   **one line**: `<ℹ> <animal/cage> | <item label> | <toggle 1><toggle 2>…`
   with toggles in due-time order.
2. The expected time is stated once per time sub-group (sub-group label),
   not repeated on every occurrence.
3. The `ℹ` detail button is the first element of the line, in a fixed-width
   leading column, so every row's `ℹ` aligns vertically.
4. One click = one state change. Kinds that need input (weighing → weight,
   observation → answer, feeding entry → ration confirmation) open the
   shared input modal; everything else toggles instantly.

## §4 Grouping: cage level with animal-level toggle

1. Feeding and cleanup default to **cage-level** grouping (one line per
   cage × diet/cleanup, animals as occurrences inside).
2. The view offers a **cage ⇄ animal toggle** (`group=cage|animal`, default
   `cage`, persisted like view preferences): animal level renders one line
   per animal with the same toggle buttons.
3. Animal identity in a line: on multi-animal surfaces the animal cell leads
   (after `ℹ`); on the animal's own page the cell is omitted (identity is
   the page context).

## §5 Responsive fall-back ladder

When a line's content does not fit, the following degradations apply **in
order** — never skip a level, never overflow horizontally, never clip a
button:

1. **Level 1 — shrink the animal**: the animal cell shows only the
   year/number (`YearNumberFormatted`); the full label stays available in
   the cell's `title` and in the `ℹ` detail modal.
2. **Level 2 — buttons stack under the item**: the toggle group takes its
   own full-width row under `<ℹ> <label>`, wrapping, right-aligned.
3. **Level 3 — full stack**:
   ```
   animal number
   <ℹ> item details (multiline, no ellipsis)
   <toggle 1><toggle 2><…>   (multiline, left-aligned)
   ```

At every level: no horizontal scrollbar, no `text-overflow: ellipsis` on
content (R4-7.24 — never cut a description), all buttons fully visible.

## §6 Bucket captions (morning / noon / evening)

1. The time-of-day grouping of a series (morning → noon → evening) is
   labelled: a full-width caption band (`.plan-med-bucket`,
   `t("care_plan.slot.*")`) separates the button rows of one stacked series.
2. The caption appears **between button rows**, never inline between two
   buttons of the same row — when the whole series fits on one row, the
   caption is omitted (the slot `title` already carries the bucket name).
3. On space-constrained read-only surfaces (dashboard cell) captions are
   suppressed entirely (`medSeriesCompact`).

## §7 Shared components (implementation contract)

The guideline is enforced by construction, not by copy-editing:

| Component | Role |
|---|---|
| `care_plan/_plan_tier.plush.html` | ONE tier panel (§1) — every section incl. History |
| `care_plan/_plan_med_row.plush.html` | ONE medication row shell (animal cell + series) |
| `care_plan/_plan_item_line.plush.html` | ONE occurrence line for row kinds (params: `showAnimal`, `showKind`) |
| `care_plan/_plan_slot_toggle.plush.html` | ONE toggle button (§2) for every kind |
| `slotTierClass()` (Go) | ONE tier→colour policy (§2.4) |
| `setToggleState()` (JS) | ONE in-place flip implementation (§2.2) |

Localized template forks (`*.plush.fr/de/nl.html`) are byte-identical copies
of the base file; all text goes through `t()` keys present in all four
locale files. A UI change is not complete until all four forks and all four
locales are updated (project rule).

## §8 Test data prerequisites

See `docs/care-plan-test-data.md` for the seeded-data checklist the test
matrix assumes (multi-animal cage, feeding 2×/day, cleanup rule, medication
3×/day crossing bucket boundaries, observation protocol, one late and one
done occurrence).
