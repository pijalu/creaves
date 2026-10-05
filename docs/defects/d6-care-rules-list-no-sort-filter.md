# D6 — `/care_rules` list: no sort / filter / search / paging

**Date**: 2026-10 (review round). **URL**: `/care_rules`. **Severity**: low
(admin ergonomics; correctness unaffected at current row counts, degrades as
rule library grows). **Guideline**: — (admin-list UX; the care-rules surface is
the only major admin index without any list controls).

## Observed (measured)

`CareRulesResource.List` (`actions/care_rules.go:110-130`) loads the whole
table with a bare `tx.All(rules)` and renders every row in one table: no
`ORDER BY`, no pagination (`PaginateFromParams`), no kind/active/matcher
filters, no name/description search. The HTML template consequently offers no
sortable headers, filter controls, search box, or pager — the admin scans an
unsorted wall of rows. (The seeded §7.4 library plus conversion-era rules
already produce a long single page; every future rule lengthens it.)

## Expected

- Sortable columns on a whitelist: `sort=name|kind|priority|active`,
  `dir=asc|desc`.
- Filters: `kind=`, `active=`, `matcher_id=`; case-insensitive `q=` substring
  search over name + description.
- Pagination via `PaginateFromParams` (project-standard pager markup).
- JSON branch unchanged: full set, API compatibility preserved (the care-plan
  engine and any API consumers read the unsorted full list today).
- All controls localized in the four template forks.

## Reproduction steps

1. Log in as admin; open `/care_rules`.
2. Observe: one unsorted table, every rule on one page; no sort headers, no
   filter/search inputs, no pager.
3. Append `?sort=name&dir=desc` (or `?page=2`, `?q=pesée`) to the URL — the
   rendered list is identical: the params are ignored.
4. `curl -H 'Accept: application/json' /care_rules` — full set (must remain
   so).

## Root cause

`actions/care_rules.go:110-130` — the List handler uses `tx.All(rules)` with
no query modifiers and no parameter handling; `templates/care_rules/index.plush.html`
(×4) renders the single static table with no controls.

## Mapped test IDs

TM-7 (sort asc/desc per whitelisted column, kind/active/matcher filters, `q=`
search, pagination; JSON branch byte-stable) — all four locales.

## Status

**Open.** Fix owner: Phase 6 (`/care_rules` sorting/filtering/search/paging).
