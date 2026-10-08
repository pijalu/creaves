# Bug Fix and Feature Implementation Plan

**Date**: 2025-01-24  
**Session**: Review and plan for bugs.md items

---

## Overview

This document outlines the fix/implementation plan for all items in `bugs.md`. Each item includes analysis, implementation approach, test strategy, and validation steps.

---

## Bug 1: creaves-console — Collapsible Search State Preservation

### Problem
The collapsible "More filters" section on `/consolidated_animals` resets to open state when switching pages or changing parameters, losing user's collapsed/expanded preference.

### Root Cause
The `moreFilters` div uses Bootstrap's `collapse` class with server-side conditional rendering for `show` class, but doesn't persist state across page navigations.

### Solution
Use `localStorage` to persist collapse state globally for the page.

### Implementation Steps

1. **Add JavaScript to preserve state** in `templates/consolidated_animals/index.plush.html`:
   - On page load, read `localStorage.getItem('consolidated_animals_filters_collapsed')`
   - Apply state to `#moreFilters` div and toggle button
   - Listen for Bootstrap collapse events to save state changes

2. **Modify template** to not use server-side conditional for `show` class (let JS handle it)

### Code Changes

**File**: `creaves-console/templates/consolidated_animals/index.plush.html`

```javascript
// Add at bottom of template
<script>
document.addEventListener('DOMContentLoaded', function() {
    const collapseEl = document.getElementById('moreFilters');
    const toggleBtn = document.querySelector('[data-target="#moreFilters"]');
    const storageKey = 'consolidated_animals_filters_collapsed';
    
    // Restore state
    const isCollapsed = localStorage.getItem(storageKey) === 'true';
    if (isCollapsed) {
        collapseEl.classList.remove('show');
        toggleBtn.setAttribute('aria-expanded', 'false');
    } else {
        collapseEl.classList.add('show');
        toggleBtn.setAttribute('aria-expanded', 'true');
    }
    
    // Save state on toggle
    collapseEl.addEventListener('hidden.bs.collapse', function() {
        localStorage.setItem(storageKey, 'true');
    });
    collapseEl.addEventListener('shown.bs.collapse', function() {
        localStorage.setItem(storageKey, 'false');
    });
});
</script>
```

### Test Approach
1. Navigate to `/consolidated_animals`
2. Click "More filters" to collapse
3. Change a filter and submit
4. Verify filters remain collapsed after page reload
5. Expand filters, change filter, verify they stay expanded

### Validation
- [ ] State persists across page navigation
- [ ] State persists across filter changes
- [ ] State persists across sessions (localStorage)
- [ ] No JavaScript errors in console

---

## Bug 2: creaves — Menu Too Long (Admin Dropdown)

### Problem
Administration menu has 15+ items, making it hard to navigate.

### Solution
Reorganize into logical groups with separators:
- **Configuration**: All reference data tables + translations
- **System**: Users, maintenance, event stream, webhook resync

### Implementation Steps

1. **Restructure** `templates/application.plush.html` admin dropdown
2. **Group items**:
   - Configuration header/divider
   - Drugs, Animal age, Animal types, Outcome types, Care types, Travel types, Localities, Zones
   - Translations
   - System header/divider
   - All Routes, Users, Maintenance, Configuration, Event Stream, Webhook resync

### Code Changes

**File**: `creaves/templates/application.plush.html` (lines 38-60)

```html
<div class="dropdown-menu" aria-labelledby="adminDropdown">
    <!-- Configuration Section -->
    <h6 class="dropdown-header">Configuration</h6>
    <a class="dropdown-item" href="<%= drugsPath() %>"><i class="fas fa-pills"></i> Drugs</a>
    <a class="dropdown-item" href="<%= animalagesPath() %>"><i class="fas fa-hourglass-half"></i> Animal age</a>
    <a class="dropdown-item" href="<%= animaltypesPath() %>"><i class="fas fa-tags"></i> Animal types</a>
    <a class="dropdown-item" href="<%= outtaketypesPath() %>"><i class="fas fa-sign-out-alt"></i> Outcome types</a>
    <a class="dropdown-item" href="<%= caretypesPath() %>"><i class="fas fa-hand-holding-heart"></i> Care types</a>
    <a class="dropdown-item" href="<%= traveltypesPath() %>"><i class="fas fa-car"></i> Travel types</a>
    <a class="dropdown-item" href="<%= localitiesPath() %>"><i class="fas fa-map-marker-alt"></i> Localities</a>
    <a class="dropdown-item" href="<%= zonesPath() %>"><i class="fas fa-map"></i> Zones</a>
    <a class="dropdown-item" href="/translations"><i class="fas fa-language"></i> Translations</a>
    
    <div class="dropdown-divider"></div>
    
    <!-- System Section -->
    <h6 class="dropdown-header">System</h6>
    <a class="dropdown-item" href="<%= pathsPath() %>"><i class="fas fa-route"></i> All Routes</a>
    <a class="dropdown-item" href="<%= usersPath() %>"><i class="fas fa-users"></i> Users</a>
    <a class="dropdown-item" href="/maintenance"><i class="fas fa-wrench"></i> Maintenance</a>
    <a class="dropdown-item" href="/config"><i class="fas fa-sliders-h"></i> Configuration</a>
    <a class="dropdown-item" href="/event_streams"><i class="fas fa-stream"></i> Event Stream</a>
    <a class="dropdown-item" href="/webhook_resync"><i class="fas fa-sync-alt"></i> Webhook resync</a>
</div>
```

### Test Approach
1. Login as admin
2. Open Administration dropdown
3. Verify items are grouped with headers
4. Verify all links work correctly

### Validation
- [ ] Menu shows Configuration section with 9 items
- [ ] Menu shows System section with 6 items
- [ ] All links navigate correctly
- [ ] Visual grouping is clear

---

## Bug 3: creaves-console — CSV Reports Section

### Problem
CSV reports are scattered; need a dedicated section listing all available exports.

### Solution
Create a new "CSV Reports" page under Reports menu listing all available exports with download links.

### Implementation Steps

1. **Add route** in `creaves-console/actions/app.go`:
   ```go
   app.GET("/reports/csv", ReportsCSVIndex)
   ```

2. **Create handler** `ReportsCSVIndex` in `creaves-console/actions/reports_csv.go`:
   - List all available CSV exports
   - Show description and link for each

3. **Create template** `creaves-console/templates/reports/csv.plush.html`:
   - Table with export name, description, parameters, download link

4. **Add navigation** in `templates/application.plush.html` Reports dropdown

### Available CSV Exports
| Export | Endpoint | Parameters |
|--------|----------|------------|
| Consolidated Animals | `/consolidated_animals/export.csv` | All filters |
| Annual Statistics | `/reports/annual/export.csv` | year, instance_id |
| Register | `/reports/register/export.csv` | year, instance_id |
| Snapshot | `/reports/snapshot/export.csv` | snapshotDate, instance_id |

### Test Approach
1. Navigate to Reports → CSV Reports
2. Verify all 4 exports are listed
3. Click each download link
4. Verify CSV downloads correctly

### Validation
- [ ] Page lists all CSV exports
- [ ] Links work with current filter context
- [ ] Downloads are valid CSV files

---

## Bug 4: creaves — CSV Export Reports (Online Version)

### Problem
CSV exports only download files; need an "online" tabular view.

### Solution
Restructure the Reports menu with a new "Export" submenu containing:
- **View** — online tabular view of query results (renamed from current "CSV exports")
- **CSV** — download CSV files (existing functionality, unchanged)

### Implementation Steps

1. **Restructure Reports menu** in `templates/application.plush.html`:
   ```
   Reports
   ├── Register
   ├── Snapshot  
   ├── Annual statistics
   ├── ──────────
   ├── Export ▸
   │   ├── View (online)     ← NEW: renamed from "CSV exports"
   │   └── CSV (download)    ← existing /export/csv
   └── ──────────
       Excel exports         ← existing /export/excel
   ```

2. **Add route** for online view:
   ```go
   app.GET("/export/view", ExportView)         // query list
   app.GET("/export/view/:query", ExportViewQuery)  // results as HTML table
   ```

3. **Create handler** `ExportView` in `creaves/actions/export_view.go`:
   - List all queries from `export.GetQueries()`
   - Execute query and render as HTML table
   - Add sorting/filtering via query params

4. **Create templates**:
   - `creaves/templates/export/view.plush.html` — query selector page
   - `creaves/templates/export/view_results.plush.html` — results table

5. **CSV download** remains available at existing `/export/csv` endpoint

### Code Changes

**File**: `creaves/templates/application.plush.html` (Reports dropdown)

Replace:
```html
<a class="dropdown-item" href="<%= exportCsvPath() %>"><i class="fas fa-file-csv"></i> CSV exports</a>
```

With:
```html
<div class="dropdown-submenu">
    <a class="dropdown-item dropdown-toggle" href="#"><i class="fas fa-download"></i> Export</a>
    <div class="dropdown-menu">
        <a class="dropdown-item" href="/export/view"><i class="fas fa-table"></i> View (online)</a>
        <a class="dropdown-item" href="<%= exportCsvPath() %>"><i class="fas fa-file-csv"></i> CSV</a>
    </div>
</div>
```

**Note**: Bootstrap 4 doesn't support nested dropdowns natively. Alternative: use two separate dropdown items or a flat list:
```html
<div class="dropdown-divider"></div>
<h6 class="dropdown-header">Export</h6>
<a class="dropdown-item" href="/export/view"><i class="fas fa-table"></i> View (online)</a>
<a class="dropdown-item" href="<%= exportCsvPath() %>"><i class="fas fa-file-csv"></i> CSV</a>
```

### Test Approach
1. Navigate to Reports → Export → View
2. Select a query
3. Verify results display in table
4. Navigate to Reports → Export → CSV
5. Verify CSV download works

### Validation
- [ ] Export submenu shows View and CSV options
- [ ] Online view shows tabular data
- [ ] CSV export still available and functional
- [ ] All queries work in both modes

---

## Bug 5: creaves — Excel Reports Error

### Problem
Excel returns error: *"We found a problem with some content in 'stat_communes.xlsx'. Do you want us to try to recover as much as we can?"*

### Root Cause Analysis

The XLSX template files contain **pivot tables, pivot caches, charts, and slicers** with hardcoded data range references. When excelize writes new data to the template, these references become stale:

**`stats_communes.xlsx`** (more problematic):
- Pivot cache references `bdd!$A$1:$N$3696` but data may have fewer/more rows
- 9 pivot tables + 5 charts + 4 slicers all referencing the pivot cache
- `_FilterDatabase` defined name: `bdd!$A$1:$N$3696`
- `recordCount="3695"` hardcoded in pivot cache
- Shared items in pivot cache contain cached values from old data

**`registre.xlsx`**:
- Pivot cache references `animals!$A$1:$AG1368`
- 1 pivot table with cached shared items
- `recordCount="1367"` hardcoded

### The Flow Problem

```
1. Excelize opens template (preserves all pivot/chart/slicer definitions)
2. New data written to sheet (different row count than template)
3. excelize saves file — pivot cache still references old range + old cached values
4. Excel opens file → detects mismatch between pivot cache and actual data
5. Excel shows "recover" error
```

### Solution: Option D — Update Pivot Cache

Since excelize v2.8.0 doesn't expose a direct API to update existing pivot caches, we need to manipulate the internal XML via `f.Pkg` (exported sync.Map containing all XLSX parts).

### Approach

1. **After writing data**, update the pivot cache definition to reference the new data range
2. **Clear pivot cache records** (cached shared items) so Excel rebuilds them on open
3. **Set `refreshOnLoad="1"`** on pivot cache to force refresh when opened
4. **Update `_FilterDatabase`** defined name if present (stats_communes only)

### Implementation Steps

1. **Create `updatePivotCache` function** in `excel/excel.go`:
   ```go
   // updatePivotCache updates all pivot caches to reference the new data range
   // and clears cached records so Excel refreshes on open.
   func updatePivotCache(f *excelize.File, sheetName string, lastRow int, lastCol string) error
   ```

2. **Update pivotCacheDefinition XML**:
   - Change `worksheetSource/@ref` from old range to `A1:{lastCol}{lastRow}`
   - Set `recordCount` to actual data row count
   - Set `refreshOnLoad="1"` to force Excel refresh
   - Clear `pivotCacheRecords` (delete or empty the records file)

3. **Update defined names** (stats_communes only):
   - Fix `_xlnm._FilterDatabase` to reference new range

4. **Handle both templates**:
   - `registre.xlsx`: 1 pivot cache, no definedNames issue
   - `stats_communes.xlsx`: 1 pivot cache + `_FilterDatabase` defined name

### Code Changes

**File**: `creaves/excel/excel.go`

Add new function:
```go
import (
    "encoding/xml"
    "fmt"
    "regexp"
    "strconv"
    "strings"
    
    "github.com/xuri/excelize/v2"
)

// pivotCacheDefinition represents the pivotCacheDefinition XML structure
// we need to modify.
type pivotCacheDefinition struct {
    XMLName          xml.Name `xml:"pivotCacheDefinition"`
    RefreshedVersion string   `xml:"refreshedVersion,attr"`
    RefreshOnLoad    string   `xml:"refreshOnLoad,attr"`
    RecordCount      string   `xml:"recordCount,attr"`
    CacheSource      struct {
        Type            string `xml:"type,attr"`
        WorksheetSource struct {
            Ref   string `xml:"ref,attr"`
            Sheet string `xml:"sheet,attr"`
        } `xml:"worksheetSource"`
    } `xml:"cacheSource"`
}

// updatePivotCaches updates all pivot caches to reference the current data
// range and forces Excel to refresh pivot tables on open.
func updatePivotCaches(f *excelize.File, dataSheet string, lastRow int, lastCol string) error {
    newRef := fmt.Sprintf("A1:%s%d", lastCol, lastRow)
    
    // Iterate over all files in the package to find pivot cache definitions
    var updateErr error
    f.Pkg.Range(func(k, v interface{}) bool {
        path, ok := k.(string)
        if !ok {
            return true
        }
        
        // Look for pivot cache definitions
        if strings.Contains(path, "xl/pivotCache/pivotCacheDefinition") && strings.HasSuffix(path, ".xml") {
            content, ok := v.([]byte)
            if !ok {
                return true
            }
            
            // Parse and update the pivot cache definition
            var pcd pivotCacheDefinition
            if err := xml.Unmarshal(content, &pcd); err != nil {
                updateErr = fmt.Errorf("failed to parse %s: %w", path, err)
                return false
            }
            
            // Update the data range reference
            pcd.CacheSource.WorksheetSource.Ref = newRef
            pcd.CacheSource.WorksheetSource.Sheet = dataSheet
            pcd.RecordCount = strconv.Itoa(lastRow - 1) // Exclude header
            pcd.RefreshOnLoad = "1" // Force refresh on open
            
            // Marshal back to XML
            updated, err := xml.Marshal(pcd)
            if err != nil {
                updateErr = fmt.Errorf("failed to marshal %s: %w", path, err)
                return false
            }
            
            // Save back to package
            f.Pkg.Store(path, append([]byte(xml.Header), updated...))
            
            // Clear the corresponding pivot cache records file
            recordsPath := strings.Replace(path, "pivotCacheDefinition", "pivotCacheRecords", 1)
            if _, exists := f.Pkg.Load(recordsPath); exists {
                // Replace with empty records - Excel will rebuild from source data
                emptyRecords := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<pivotCacheRecords xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" count="0"></pivotCacheRecords>`)
                f.Pkg.Store(recordsPath, emptyRecords)
            }
        }
        return true
    })
    
    return updateErr
}

// updateFilterDatabase updates the _FilterDatabase defined name to match
// the current data range (fixes stats_communes.xlsx).
func updateFilterDatabase(f *excelize.File, sheetName string, lastRow int, lastCol string) error {
    // Read workbook.xml
    content, ok := f.Pkg.Load("xl/workbook.xml")
    if !ok {
        return fmt.Errorf("workbook.xml not found")
    }
    
    wb, ok := content.([]byte)
    if !ok {
        return fmt.Errorf("invalid workbook.xml content")
    }
    
    newRef := fmt.Sprintf("%s!$A$1:$%s$%d", sheetName, lastCol, lastRow)
    
    // Replace _FilterDatabase reference
    // Pattern: <definedName name="_xlnm._FilterDatabase" localSheetId="N" hidden="1">SHEET!$A$1:$X$N</definedName>
    pattern := regexp.MustCompile(`(<definedName name="_xlnm\._FilterDatabase"[^>]*>)[^<]*(</definedName>)`)
    updated := pattern.ReplaceAll(wb, []byte("${1}"+newRef+"${2}"))
    
    f.Pkg.Store("xl/workbook.xml", updated)
    return nil
}
```

**Update `RunQuery` function** (after data write, before `UpdateLinkedValue`):

```go
// After all data is written:
// Update pivot caches to reference new data range and force refresh
lastCol := sheetPosition(1, len(cols)) // Get column letter for last column
if err := updatePivotCaches(f, sqlQuery.Sheet, line, lastCol); err != nil {
    c.Logger().Debugf("warning: failed to update pivot caches: %v", err)
}

// Update _FilterDatabase defined name if present
if err := updateFilterDatabase(f, sqlQuery.Sheet, line, lastCol); err != nil {
    c.Logger().Debugf("warning: failed to update filter database: %v", err)
}

// Remove or replace UpdateLinkedValue - it may cause issues with pivot tables
// if err := f.UpdateLinkedValue(); err != nil {
//     c.Logger().Debugf("Failed updated linked value: %v", err)
// }
```

### Why This Works

1. **Pivot cache range updated** → Excel knows where the actual data is
2. **`refreshOnLoad="1"`** → Excel ignores stale cached values and rebuilds from source
3. **Empty `pivotCacheRecords`** → Excel doesn't try to use old cached values
4. **`recordCount` updated** → Matches actual data size

### Test Approach

1. **Unit test**: Test `updatePivotCaches` modifies XML correctly
2. **Integration test**: Generate export with known data
3. **Excel validation**: Open in Microsoft Excel, verify:
   - No recovery error
   - Pivot tables show correct data
   - Can refresh pivot tables manually

### Validation

- [x] `stat_communes` export opens without recovery error
- [x] `registre_detail` export opens without recovery error  
- [x] Pivot tables show updated data
- [ ] Pivot tables can be refreshed manually in Excel
- [x] Data range in pivot cache matches actual data

### Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| excelize internal API changes | Pin excelize version, add tests |
| Complex XML manipulation | Extensive unit tests for XML parsing |
| Excel still shows warnings | Test with multiple Excel versions |
| Performance with large datasets | Benchmark with production-like data |

### Alternative: If Option D Proves Unreliable

Fall back to **Option A+B**: Strip pivot tables and provide separate "pivot-ready" CSV export for users who need pivot functionality.

---

## Bug 6: creaves-console — Excel Reports (All / Per Instance)

**Added by user 2025-01-24:** "The excel solution should be available to creaves-console as well (all/per instance)."

### Problem

The Excel export (Bug 5's pivot-preserving fix) exists only in creaves. The console — which already consolidates data from all instances and has all/per-instance scoping (`actions/report_scope.go`) — has no Excel export.

### Solution

Port the Excel export to creaves-console, scoped via the existing `ReportScope` pattern (same "Center" `<select name="instance_id">` selector used by all console reports pages).

### Design

- **New package** `creaves-console/excel/` (projects share no code — duplicate + adapt):
  - Copy `creaves/excel/excel.go` structure (embed templates, `RunQuery`, `sheetPosition`).
  - **Include the Bug 5 pivot-cache fix from day one** (`updatePivotCaches` + `updateFilterDatabase`, `refreshOnLoad=1`) — the console never ships the broken version.
  - Copy templates `registre.xlsx` + `stats_communes.xlsx` into `creaves-console/excel/templates/`.
  - `excel/config/config.yaml` with console-adapted queries (see below).
- **Queries** — console schema is denormalized (`consolidated_animals`), so queries are simpler; instance scoping via `ScopedWhere(scope, ...)`:
  - `registre_detail`: map consolidated_animals columns to the register columns (species, gender, ring, cage, animal_age, intake_date, intake_*, discovery_*, outtake_*, stay duration via `DATEDIFF(outtake_date, intake_date)+1`). Fields with no console equivalent (discoverer contact, vet visits count, travel KM) are omitted from the console variant of the template — **or** kept as empty columns to keep the template/pivot layout identical (preferred: keep layout, emit empty strings).
  - `stat_communes`: `year, year_number, species, intake_date, entry_cause, discovery_location, discovery_city, discovery_postal_code` from consolidated_animals. Locality hierarchy (Commune/Province/Région/Direction) has no console equivalent — same empty-column approach.
  - Scope predicate: `instance_id = ?` when scoped; absent when global.
- **Routes** (auth-restricted like existing exports — admin/data_entry):
  - `GET /export/excel?query=registre_detail&instance_id=` 
  - `GET /export/excel?query=stat_communes&instance_id=`
  - Handler resolves scope via `reportScope(c, tx)` (404 on unknown instance, as existing reports).
- **UI**: links on the new Reports/CSV section page (Bug 3) — each report row gets an "Excel" download link carrying the current `instance_id` selection; plus the same Center selector on that page.

### Implementation Steps

1. Create `creaves-console/excel/` package (port of creaves `excel.go` + Bug 5 fix helpers + tests).
2. Copy XLSX templates + write console `config.yaml` with adapted queries.
3. `ExportExcel` handler in `actions/` with scope resolution.
4. Routes in `app.go`.
5. Links on Bug 3 reports page (with instance_id passthrough).
6. Unit tests: query building per scope (global vs instance), pivot-cache rewrite helpers, handler auth (302/401 unauthorized, 200 authorized), generated XLSX validation (pivot cache ranges match row count).
7. e2e via agent-browser: select "All centers" → download both Excel files → inspect XML; select one instance → download → verify rows are scoped (recordCount matches per-instance count).

### Test Approach

- Unit: adapted SQL produces expected columns; scope predicate appended correctly (`report_scope_sqlite_test.go` pattern exists to follow).
- Integration: run export against sqlite test DB with 2 instances seeded; assert per-instance file contains only that instance's rows.
- e2e (mandatory, per guideline 5): agent-browser download + XML inspection.

### Validation

- Both queries work for global and per-instance scope.
- Generated files open without repair prompt (XML structure check + manual Excel/LibreOffice open).
- Pivot caches refresh on load with scoped data.

### Risks

| Risk | Mitigation |
|------|------------|
| Console schema lacks some register fields (discoverer, vet visits, KM, locality hierarchy) | Keep template layout; emit empty columns; document difference |
| Duplicated excel package drifts from creaves version | Single-source the pivot-update logic as near-identical copy; cross-reference comments in both packages |
| Templates assume French headers from creaves queries | Console queries use identical column aliases (AS "Année" etc.) so templates match |

---

## Execution Order

| Priority | Item | Project | Effort |
|----------|------|---------|--------|
| 1 | Collapsible search state | creaves-console | Small |
| 2 | Menu simplification | creaves | Small |
| 3 | CSV Reports section | creaves-console | Medium |
| 4 | Drill Export (online CSV) | creaves | Medium |
| 5 | Excel export fix | creaves | Medium |
| 6 | Excel export (all/per instance) | creaves-console | Medium |

---

## Quality Checks (per bugs.md guidelines)

For each fix (each tool run separately — no chaining):
1. `go vet ./...`
2. `staticcheck ./...`
3. `gocognit -over 15 .`
4. `gocyclo -over 12 .`
5. `go test -count=1 -race -cover ./...`
6. **e2e validation with the agent-browser skill is mandatory (bugs.md guideline 5)** — capture command output + URL as evidence; screenshots alone are not sufficient

---

## Notes

- **creaves** contains production data — no destructive migrations
- **creaves-console** dev DB can be rebuilt freely
- Each fix should be committed separately with clear message
- Move completed items to `docs/archive/`
