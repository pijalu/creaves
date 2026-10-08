# Bug 2: Export view sort/filter freezes Firefox (slow-script error)

**Status**: fixed & verified (2026-09-08)
**Project touched**: creaves (`templates/export/view.plush.html` only)

## Symptom

`/export/view?query=register` renders ~10 000 rows × 14 columns. Clicking the
"Code postal découverte" column sort, or typing in the filter box, froze the
page for >10 s; Firefox showed:

```
This page is slowing down Firefox. To speed up your browser, stop this page.
```

## Reproduction (agent-browser, Chromium)

- `sortExportTable(7)` via CDP eval → renderer main thread blocked >150 s
  (CDP channel itself stalled: "Resource temporarily unavailable").
- Same freeze on the real header-link click.

## Root cause

Two compounding factors in the inline JS/CSS of the export view:

1. **`table-layout: auto` (default)** on a 10 018-row table: every DOM
   mutation forces the engine to re-measure all ~140 k cells to recompute
   column widths. Measured: a *single* forced layout after a mutation ≈ 0.9 s
   even when batched; with auto layout the per-mutation re-measure dominates.
2. **Per-row `tbody.appendChild(r)` reorder loop** (10 018 insertions): each
   insertion into the live table triggers table-structure bookkeeping/style
   invalidation, so the loop degenerated to minutes. Even a *detached*
   per-row appendChild loop remained pathological in Chromium (>150 s).

The sort comparator itself was **not** the problem: key extraction 8 ms,
key sort 20 ms, full comparator sort 34 ms.

## Fix (minimal diff, template only)

- `table` element: `style="table-layout: fixed;"` — column widths come from
  the header row, so mutations no longer re-measure every cell.
- `sortExportTable()`: replaced the per-row `appendChild` loop with one
  `tbody.replaceChildren.apply(tbody, rows)` call — a single DOM mutation
  for the whole reorder.

Filter needed no JS change (fixed layout already makes it fast).

## Validation (agent-browser, live page, 10 018 rows)

| Action | Before | After |
|---|---|---|
| Sort postal-code col (header click) | >150 s freeze | 281 ms sort + 587 ms layout |
| Sort desc (2nd click) | — | 235 ms + 692 ms, order reversed correctly |
| Filter "3000" | "extremely slow" | 89 ms + 205 ms, 3/10 018 visible |
| Filter clear | — | all 10 018 rows restored |

Correctness checked: ascending order verified across all 10 018 rows
(`localeCompare …, {numeric:true}` pairwise), empty cells first; descending
click reverses. No console errors. Screenshot: post-sort table rendered.

## Quality gates

- `go vet ./...` — clean
- `staticcheck ./...` — only pre-existing warnings (grifts dot imports,
  models log reimport) — unrelated
- `gocognit -over 15 .` / `gocyclo -over 12 .` — only pre-existing entries
  (event_producer, sqldump, animals, startup_seed) — unrelated
- `go test -count=1 -race -cover ./...` — all packages ok

## Residual notes

- `replaceChildren.apply(tbody, rows)` spreads ~10 k args — within engine
  limits (Firefox/Chrome ≥ 65 k). If exports grow well beyond that, switch
  to a DocumentFragment fill.
- Bug 3 (DataTables evaluation) may supersede this hand-rolled sort/filter;
  this fix keeps the current page usable meanwhile.
- e2e used a temporary `e2e_test` admin user, deleted after validation.
