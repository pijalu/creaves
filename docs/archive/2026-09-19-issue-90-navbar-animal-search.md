## #90 — Barre de recherche d'animaux dans le menu — DONE ✅ commit e087bad
**Source:** https://github.com/pijalu/creaves/issues/90
**Observed:** Animal search bar lives on dedicated page only; users want it in top menu (PC + mobile).
**Expected:** Search input in navbar, desktop + mobile responsive, submits to existing search.
**Plan:**
1. `templates/application.plush*.html`: add search form in navbar pointing at existing animals search route (`actions/animals_search.go`).
2. Responsive: visible field on desktop (input-group), collapsible on mobile (navbar collapse section).
3. All 4 locales unaffected (no text besides placeholder — reuse i18n key if exists, else add to all locale packs).
**Test:** e2e agent-browser: search from navbar on desktop + mobile viewport widths; result page identical to dedicated search page.
**Validation:** screenshot desktop + mobile widths; search returns same results as /animals search.
