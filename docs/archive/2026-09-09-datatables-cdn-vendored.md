# bugs.md — DataTables (and other JS/CSS) assets loaded from external CDN (creaves-console)

**Status**: fixed, tested, committed (creaves-console `5376aa4`), archived 2026-09-09.

## Bug report (from bugs.md)

> **Observed**: `creaves-console/templates/application.plush.*.html` loaded
> DataTables CSS/JS from `https://cdn.datatables.net/3.0.3/...` (stylesheet in
> `<head>`, scripts at end of body). External CDN dependency: breaks offline /
> air-gapped deployments, adds supply-chain + privacy exposure, and the version
> is not locked by the Go build. (creaves already bundles DataTables via
> webpack `application.js` — console must match.)
>
> **Expected**: all datatable content (JS + CSS + any DataTables images/fonts)
> packaged with the build and minified — served from the app's own assets
> (`public/assets/...`), **zero external CDN** for datatables (audit the whole
> layout for other CDN assets while at it).

## Fix

Approach: **vendored minified dist files** (explicitly allowed by the bug's
fix direction) instead of adding a full webpack/npm pipeline — the console has
no webpack build at all (no `package.json`, no `assets/` dir), and its
`public/` tree is served via `go:embed` (`public/embed.go`), so plain vendored
files are packaged into the binary with zero build changes.

- Downloaded the exact previously-used versions from the CDNs into
  `public/assets/vendor/`:
  - jQuery 3.6.0 (`jquery/jquery-3.6.0.min.js`)
  - Bootstrap 4.6.2 (`bootstrap/bootstrap.min.css`,
    `bootstrap/bootstrap.bundle.min.js`)
  - DataTables 3.0.3 core + Bootstrap 4 integration
    (`datatables/dataTables.min.js`, `datatables/dataTables.bootstrap4.min.js`,
    `datatables/dataTables.bootstrap4.min.css`)
  - Font Awesome 5.15.4 (`fontawesome/css/all.min.css` + all 15 referenced
    `fontawesome/webfonts/fa-*.{eot,svg,ttf,woff,woff2}`)
- Rewrote the asset URLs in all 4 locale layouts
  (`templates/application.plush{,.fr,.de,.nl}.html`) from the CDN URLs to the
  local `/assets/vendor/...` paths.
- Updated the (now stale) "CDN assets loaded" comment in the 4 export view
  templates.
- Verified DataTables' CSS has no external `url()` references (sorting icons
  are inline SVG data URIs), and Font Awesome's CSS only references the
  vendored `../webfonts/` files — so the bundles are fully self-contained.

CDN audit of the whole layout/app: besides the three DataTables/Bootstrap/FA
URLs, the layout also loaded jQuery from `code.jquery.com` — all four CDNs
removed. No other CDN references exist in `templates/`, `actions/` or
`public/`. (Buffalo's own dev-mode 404 error page still loads its logo from
`gobuffalo.io` — framework-internal, dev-only, out of scope.)

## Tests

No Go code changed; templates + static files only.

**Unit/regression**: full console suite green before e2e:
```
CGO_ENABLED=1 go test -tags sqlite ./actions/... ./models/...  →  ok actions 6.716s, ok models
```

**E2E (agent-browser, dev binary on :3001, admin/admin123 login)**:

- `GET /export/reports` — renders with all assets loaded from
  `/assets/vendor/...` (verified in network log).
- `GET /export/reports/view?query=register` — DataTable initialises:
  entries-per-page selector (25/50/100/250/500), search box, sortable
  column headers, paginator.
- Interactions (all client-side, verified live):
  - sort by `N°` column → first cell flips to `2026`-sorted value;
  - type `2024` in search → `Showing 1 to 50 of 2,146 entries (filtered
    from 10,046 total entries)`;
  - click paginator *Next* → `Showing 51 to 100 of 2,146 entries`.
- **Network log for every page load** (login page, dashboard,
  `/export/reports`, `/export/reports/view?query=register`): all 8 JS/CSS
  files + the FA woff2 font served from
  `http://localhost:3001/assets/vendor/...` with status 200; a final grep
  of the full request list for `cdn.`, `jsdelivr`, `code.jquery`, `cdnjs`,
  `datatables.net` matched **zero** external requests (only the local
  `/assets/vendor/datatables/` paths). The only remaining external request
  in the log was `gobuffalo.io` logo on the framework's own dev 404 page
  (triggered by an intentionally-wrong query name — not part of the app
  layout).

## Code quality

- `go vet ./...` — clean.
- `staticcheck ./...` — clean.
- `gocognit -over 15 .` — only pre-existing warnings in unrelated files
  (`models.(*ConsolidatedAnimal).UpdateFromPayload` 53, etc.); none in
  changed files (no Go changes).
- `gocyclo -over 12 .` — same, all pre-existing and unrelated.
- `CGO_ENABLED=1 go test -count=1 -race -cover -tags sqlite ./...` —
  all packages ok (actions 56.3%, excel 64.5%, models 57.8% coverage).

## Commit

```
5376aa4 fix(assets): vendor jQuery/Bootstrap/DataTables/Font Awesome — drop all CDN dependencies (bugs.md datatables item)
```

Files: `templates/application.plush{,.fr,.de,.nl}.html`,
`templates/export/view.plush{,.fr,.de,.nl}.html` (comment),
`public/assets/vendor/**` (23 vendored files).
