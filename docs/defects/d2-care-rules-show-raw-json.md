# D2 — `/care_rules/{id}` returns raw JSON for a browser GET

**Date**: 2026-10 (review round). **URL**: `/care_rules/{id}` (any rule id).
**Severity**: medium (a browser navigation to a rule detail URL renders a raw
JSON blob instead of a page — dead-end screen for admins). **Guideline**: —
(UX routing defect; the care-rules admin surface must be usable in a browser).

## Observed (measured)

`GET /care_rules/{id}` from a browser (`Accept: text/html`) returns
`Content-Type: application/json` with the serialized rule — the Show handler
renders JSON unconditionally:

```go
// actions/care_rules.go:133-143
func (v CareRulesResource) Show(c buffalo.Context) error {
    ...
    return c.Render(http.StatusOK, renderJSON(rule))
}
```

There is no `templates/care_rules/show.plush.html` at all — every other admin
resource (zones, matchers, species…) serves an HTML detail page; the List
handler right above (`care_rules.go:110-130`) is properly content-negotiated
via `responder.Wants("html", …).Wants("json", …)`, so Show is the outlier.

## Expected

- Browser GET → HTML detail page (read-only: name, kind badge, matcher name,
  priority, active flag, validity window, humanized payload + schedule, admin
  edit/delete buttons, back link honouring `?back=`), in all four locales.
- `Accept: application/json` → the existing JSON payload, unchanged (API
  compatibility).

## Reproduction steps

1. Log in as admin; open `/care_rules` and copy any rule id.
2. Navigate the browser to `/care_rules/{id}` (or click a hand-built link).
3. Observe a raw JSON document in the viewport instead of an HTML page.
4. `curl -H 'Accept: application/json' …/care_rules/{id}` — same payload
   (this branch must be preserved).

## Root cause

`actions/care_rules.go:133-143` — `CareRulesResource.Show` ends in an
unconditional `c.Render(http.StatusOK, renderJSON(rule))` with no
`responder.Wants` negotiation and no HTML template fork
(`templates/care_rules/` has only `index`/`new`/`edit`).

## Mapped test IDs

TM-3 (HTML page on browser GET; JSON branch unchanged) — all four locales.

## Status

**Verified** (Phase 2). Fix owner: Phase 2 (care rule show page HTML).

### Fix

- `actions/care_rules.go` — `CareRulesResource.Show` now forks through
  `responder.Wants("html", …).Wants("json", …)`; the JSON branch renders the
  identical payload as before (API contract preserved). The HTML branch sets
  the rule, its matcher name and a sanitized back target
  (`localBackParam` + `unwrapBackChain`, default `/care_rules`) and renders
  the new detail template.
- `templates/care_rules/show.plush.html` (+ `.fr/.de/.nl` forks) — read-only
  detail page: name (`richPlanName`+`tname`), description, kind badge,
  matcher name (or the localized "no matcher" text), priority, active badge,
  validity window, humanized payload content + raw-JSON disclosure,
  humanized schedule + raw-JSON disclosure, stop/latch flags, admin
  edit/delete buttons and a back link honouring `?back=`.
- `locales/care_plan.{en-us,fr,de,nl}.yaml` — new keys
  `care_plan.rules.show_title`, `care_plan.action.back`,
  `care_plan.field.validity_window`, `care_plan.validity.unbounded`.

### Verification (direct execution)

- `go test ./actions -run 'TestCareRuleShow' -count=1 -v` — all 6 new tests
  PASS (`actions/care_rules_show_test.go`):
  - 2-T1 `TestCareRuleShowHTMLBrowserGet` — browser GET with
    `?back=/care_plan?kind=cleanup` → 200 HTML, every field rendered (name,
    description, matcher, priority, kind badge, humanized payload + schedule,
    validity dates, stop/latch labels), back link
    `href="/care_plan?kind=cleanup"`, edit + DELETE buttons; body carries no
    raw JSON document.
  - 2-T2 `TestCareRuleShowJSONUnchanged` — `Accept: application/json` → 200,
    same payload shape as before (id/name/action_kind/matcher_id/active/
    priority + nested payload document).
  - 2-T3 `TestCareRuleShowClickThrough` — `/care_plan?kind=cleanup` and
    `?kind=medication` each render `/care_rules/{id}?back=…` source links;
    following them returns the show page whose back link returns to the same
    work screen.
  - 2-T4 `TestCareRuleShowNonAdminGate` — regular user → 403 on the HTML
    branch.
  - 2-T5 `TestCareRuleShowAllLocales` — fr/en-US/de/nl render localized
    title, back label, validity label and kind badge; no
    "translation missing" marker.
  - 2-T6 `TestCareRuleShowBackSanitized` — external/protocol-relative/
    backslash `?back=` values never reach the back link (falls back to
    `/care_rules`).
- `go test ./actions -count=1` — full suite `ok 12.581s` (no regressions;
  JSON API consumers unaffected).
