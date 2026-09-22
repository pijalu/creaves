package actions

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Bug 4: Export menu split into Export > View (online HTML table) and
// Export > CSV (download). These tests drive the full App() so the real
// routes, auth middleware and templates are exercised.
// ---------------------------------------------------------------------------

// TestExportViewRequiresLogin proves GET /export/view is behind Authorize,
// exactly like /export/csv.
func TestExportViewRequiresLogin(t *testing.T) {
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for _, path := range []string{"/export/view", "/export/csv"} {
		resp, err := client.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("GET %s unauthenticated = %d, want 302", path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/auth/new") {
			t.Fatalf("GET %s redirect = %q, want /auth/new", path, loc)
		}
	}
}

// TestExportViewChooserListsQueries proves GET /export/view without a query
// renders the chooser with View and CSV links for every configured query.
func TestExportViewChooserListsQueries(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/view")
	if err != nil {
		t.Fatalf("GET /export/view: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /export/view = %d, want 200", resp.StatusCode)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bb)
	if !strings.Contains(body, "/export/view?query=register") {
		t.Error("chooser lacks View link for 'register' query")
	}
	if !strings.Contains(body, "/export/csv?query=register") {
		t.Error("chooser lacks CSV link for 'register' query")
	}
}

// TestExportViewRendersHTMLTable proves GET /export/view?query=register runs
// the query and renders results as an HTML table.
func TestExportViewRendersHTMLTable(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/view?query=register")
	if err != nil {
		t.Fatalf("GET /export/view?query=register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET /export/view?query=register = %d, want 200 (body: %.300s)", resp.StatusCode, string(bb))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bb)
	if !strings.Contains(body, "<table") {
		t.Error("view page does not render an HTML table")
	}
	if !strings.Contains(body, "<th>") {
		t.Error("view page table has no header cells")
	}
	if !strings.Contains(body, "Registre") {
		t.Error("view page does not show the query description")
	}
	// The CSV download link must be offered from the view page.
	if !strings.Contains(body, "/export/csv?query=register") {
		t.Error("view page lacks link to CSV download")
	}
	// bugs.md datatable item: sort/filter/pagination are delegated to
	// DataTables (bundled in application.js); the hand-rolled JS is gone.
	if !strings.Contains(body, `id="exportTable"`) {
		t.Error("view page lacks the #exportTable element")
	}
	if !strings.Contains(body, `$("#exportTable").DataTable({`) {
		t.Error("view page lacks the DataTables init")
	}
	if !strings.Contains(body, "deferRender: true") {
		t.Error("DataTables init lacks deferRender (10k-row freeze protection)")
	}
	if strings.Contains(body, "sortExportTable") || strings.Contains(body, "filterExportTable") {
		t.Error("view page still contains the removed hand-rolled sort/filter JS")
	}
	if strings.Contains(body, `onclick="sortExportTable(`) {
		t.Error("column headers still carry custom onclick sort handlers")
	}
	// Per-column filters: hidden filter row (one input per column) toggled by
	// a dedicated button; sorting stays on the title row (orderCellsTop).
	// The toggle button sits at the top left of the table (before the table
	// markup), and each input's placeholder is the column name.
	for _, frag := range []string{
		`id="toggleColumnFilters"`,
		`tr class="column-filters" style="display:none"`,
		// Each filter input's placeholder is the column name (rendered), not
		// the generic word "Filter".
		`class="form-control form-control-sm column-filter" placeholder="Espèce"`,
		"orderCellsTop: true",
		`.search(this.value).draw()`,
	} {
		if !strings.Contains(body, frag) {
			t.Errorf("view page lacks per-column filter fragment %q", frag)
		}
	}
	if strings.Contains(body, `placeholder="Filter"`) {
		t.Error("filter inputs still use the generic 'Filter' placeholder")
	}
	btnIdx := strings.Index(body, `id="toggleColumnFilters"`)
	tableIdx := strings.Index(body, `id="exportTable"`)
	if btnIdx <= 0 || btnIdx >= tableIdx {
		t.Error("filter toggle button must render before the table")
	}
	// Bug 10: the back-to-list link sits next to the export title (inside the
	// <h3>), not in the right-hand toolbar.
	if !strings.Contains(body, `<h3 class="d-inline-block"><a href="/export/view" title="Back to exports"`) {
		t.Error("back-to-exports link must render inside the title")
	}
}

// TestExportViewUnknownQuery proves an unknown query id does not crash and
// returns 404.
func TestExportViewUnknownQuery(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/view?query=does_not_exist")
	if err != nil {
		t.Fatalf("GET /export/view?query=does_not_exist: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown query = %d, want 404", resp.StatusCode)
	}
}

// TestExportCsvStillDownloads proves the CSV path is unchanged: it downloads
// a text/csv attachment.
func TestExportCsvStillDownloads(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/csv?query=register")
	if err != nil {
		t.Fatalf("GET /export/csv?query=register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /export/csv?query=register = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Fatalf("Content-Type = %q, want text/csv", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Fatalf("Content-Disposition = %q, want attachment", cd)
	}
}

// TestExportCsvUTF8Encoding proves the configured CSV export signals its
// encoding: Content-Type carries charset=utf-8 and the body starts with a
// UTF-8 BOM, so spreadsheet readers (Excel) don't fall back to the local
// ANSI codepage and garble accented values (bug: animal_age showing boxes).
func TestExportCsvUTF8Encoding(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/csv?query=register")
	if err != nil {
		t.Fatalf("GET /export/csv?query=register: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /export/csv?query=register = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "charset=utf-8") {
		t.Fatalf("Content-Type = %q, want charset=utf-8", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.HasPrefix(string(body), "\ufeff") {
		t.Fatalf("CSV body must start with UTF-8 BOM, got % x", body[:3])
	}
}

// TestReportsNavShowsExportsEntry proves the Reports dropdown in the layout
// exposes a single "Exports" entry pointing at /export/view, and no longer
// links the removed CSV chooser page.
func TestReportsNavShowsExportsEntry(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/dashboard")
	if err != nil {
		t.Fatalf("GET /dashboard: %v", err)
	}
	defer resp.Body.Close()
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bb)
	if !strings.Contains(body, `href="/export/view"`) {
		t.Error("nav lacks the Exports entry pointing at /export/view")
	}
	if !strings.Contains(body, `href="/export/view"><i class="fas fa-table"></i> Exports</a>`) {
		t.Error("nav lacks the 'Exports' label")
	}
	if strings.Contains(body, `href="/export/csv/"`) {
		t.Error("nav still links the removed CSV chooser page")
	}
	if strings.Contains(body, `dropdown-header">Export`) {
		t.Error("nav still has an Export sub-header")
	}
}

// TestExportCsvChooserRedirects proves bare /export/csv no longer renders
// the CSV chooser page: it redirects to the online view chooser.
func TestExportCsvChooserRedirects(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(srv.URL + "/export/csv")
	if err != nil {
		t.Fatalf("GET /export/csv: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("GET /export/csv = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/export/view" {
		t.Fatalf("redirect = %q, want /export/view", loc)
	}
}

// TestExportCsvHonorsViewFilters is the regression test for the #197
// sub-item 3 bug: the CSV download used to stream the raw query result,
// ignoring the filters applied in the online view. With a global search
// that matches nothing the download must contain only the header row.
func TestExportCsvHonorsViewFilters(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	// Unfiltered download: header + at least one data row.
	resp, err := client.Get(srv.URL + "/export/csv?query=register")
	if err != nil {
		t.Fatalf("GET /export/csv?query=register: %v", err)
	}
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unfiltered download = %d (body: %.200s)", resp.StatusCode, raw)
	}
	unfiltered := strings.Count(strings.TrimRight(string(raw), "\n"), "\n") + 1
	if unfiltered < 2 {
		t.Fatalf("unfiltered download has %d lines, want header + >=1 row:\n%.200s", unfiltered, raw)
	}

	// Filtered download: the global search term cannot match any row.
	resp, err = client.Get(srv.URL + "/export/csv?query=register&q=ZZZNOMATCH9471")
	if err != nil {
		t.Fatalf("GET filtered download: %v", err)
	}
	raw, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	filtered := strings.Count(strings.TrimRight(string(raw), "\n"), "\n") + 1
	if filtered != 1 {
		t.Fatalf("filtered download has %d lines, want header only:\n%.200s", filtered, raw)
	}

	// Column filter via cols=idx=substring.
	resp, err = client.Get(srv.URL + "/export/csv?query=register&cols=0%3DZZZNOMATCH9471")
	if err != nil {
		t.Fatalf("GET column-filtered download: %v", err)
	}
	raw, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	colFiltered := strings.Count(strings.TrimRight(string(raw), "\n"), "\n") + 1
	if colFiltered != 1 {
		t.Fatalf("column-filtered download has %d lines, want header only:\n%.200s", colFiltered, raw)
	}
}

// ---------------------------------------------------------------------------
// Bug 4 (2026-10): every export report can run for a specific year via an
// optional ?year= parameter, applied in SQL (subquery wrap on the query's
// declared year column), consistently on the online view, the CSV download
// and the Excel exports.
// ---------------------------------------------------------------------------

// TestExportViewYearFilter proves ?year= restricts the online view to the
// requested year and that the page shows the active filter.
func TestExportViewYearFilter(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	countRows := func(url string) int {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			bb, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET %s = %d (body: %.300s)", url, resp.StatusCode, string(bb))
		}
		bb, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return strings.Count(string(bb), "<tr>")
	}

	all := countRows(srv.URL + "/export/view?query=register")
	filtered := countRows(srv.URL + "/export/view?query=register&year=2024")
	if filtered > all {
		t.Errorf("year-filtered view has more rows (%d) than unfiltered (%d)", filtered, all)
	}

	// The year dropdown must be rendered with the requested year selected;
	// the CSV sync reads the dropdown, so it always follows the user's
	// in-view selection (bugs.md bug 5).
	resp, err := client.Get(srv.URL + "/export/view?query=register&year=2024")
	if err != nil {
		t.Fatalf("GET view: %v", err)
	}
	bb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	body := string(bb)
	if !strings.Contains(body, `<select id="viewYear" name="year"`) {
		t.Error("view page lacks the in-view year dropdown")
	}
	if !strings.Contains(body, `<option value="2024" selected>`) {
		t.Error("view page dropdown does not select the requested year")
	}
	if !strings.Contains(body, `yearSelect.value`) {
		t.Error("view page CSV sync does not read the year dropdown")
	}

	// A garbage year is ignored (no filter, no crash).
	if countRows(srv.URL+"/export/view?query=register&year=garbage") != all {
		t.Error("garbage year should be ignored and return the unfiltered result")
	}
}

// TestExportCsvYearFilter proves the CSV download is year-filtered in SQL:
// every data row's year column equals the requested year and the filename
// carries the year.
func TestExportCsvYearFilter(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/csv?query=register&year=2024")
	if err != nil {
		t.Fatalf("GET /export/csv?query=register&year=2024: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (body: %.300s)", resp.StatusCode, string(bb))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "register_2024.csv") {
		t.Errorf("Content-Disposition = %q, want filename with year", cd)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	lines := strings.Split(strings.TrimPrefix(strings.TrimSpace(string(body)), "\ufeff"), "\n")
	if len(lines) < 1 {
		t.Fatal("empty CSV")
	}
	// Column 0 of the register query is the year ("année").
	for i, line := range lines[1:] {
		first := strings.SplitN(line, ",", 2)[0]
		if first != "2024" {
			t.Errorf("row %d: year column = %q, want 2024", i+1, first)
		}
	}
}

// TestExportCsvYearFilterNoYearColumn proves a query without a year column
// (animal_gavage: animals currently in care) ignores ?year= instead of
// erroring.
func TestExportCsvYearFilterNoYearColumn(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/csv?query=animal_gavage&year=2024")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (body: %.300s)", resp.StatusCode, string(bb))
	}
	if cd := resp.Header.Get("Content-Disposition"); strings.Contains(cd, "_2024") {
		t.Errorf("filename %q must not carry a year for a year-less query", cd)
	}
}

// TestExportExcelYearFilter proves the Excel export accepts ?year= and tags
// the downloaded filename with it.
func TestExportExcelYearFilter(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/excel?query=registre_detail&year=2024")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		bb, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d (body: %.300s)", resp.StatusCode, string(bb))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "registre_detail_2024.xlsx") {
		t.Errorf("Content-Disposition = %q, want filename with year", cd)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Errorf("Content-Type = %q, want xlsx", ct)
	}
}

// TestExportExcelChooserUI proves the Excel chooser renders as a table with
// one top-of-page year dropdown (DB years + "All years" default) and one
// download button per query (bugs.md bug 5).
func TestExportExcelChooserUI(t *testing.T) {
	requireMySQLTestDB(t)
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	resp, err := client.Get(srv.URL + "/export/excel")
	if err != nil {
		t.Fatalf("GET /export/excel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	bb, _ := io.ReadAll(resp.Body)
	body := string(bb)
	for _, frag := range []string{
		"<table",
		`id="exportYear"`,
		"All years",
		`class="btn btn-success btn-sm export-link" data-base="/export/excel?query=registre_detail"`,
		"/export/excel?query=registre_detail",
		"/export/excel?query=stat_communes",
	} {
		if !strings.Contains(body, frag) {
			t.Errorf("Excel chooser lacks fragment %q", frag)
		}
	}
	if strings.Contains(body, "<ul>") {
		t.Error("Excel chooser still renders the old bare <ul> list")
	}
	if strings.Contains(body, `type="number"`) {
		t.Error("Excel chooser still renders per-row year inputs")
	}
}
