package actions

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"creaves/models"
)

// ---------------------------------------------------------------------------
// Bug 5: Excel exports must open without the "repair/recover" prompt while
// keeping pivot tables fed with the exported rows. These tests drive the real
// /export/excel route end-to-end and inspect the generated XLSX parts.
// ---------------------------------------------------------------------------

// excelPart extracts one file from an in-memory XLSX (zip) archive.
func excelPart(t *testing.T, xlsx []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(xlsx), int64(len(xlsx)))
	if err != nil {
		t.Fatalf("generated file is not a valid zip/xlsx: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open part %s: %v", name, err)
			}
			defer rc.Close()
			b, err := io.ReadAll(rc)
			if err != nil {
				t.Fatalf("read part %s: %v", name, err)
			}
			return string(b)
		}
	}
	t.Fatalf("part %s not found in generated xlsx", name)
	return ""
}

// downloadExcel fetches one Excel export through the authenticated app.
func downloadExcel(t *testing.T, srv *httptest.Server, client *http.Client, query string) []byte {
	t.Helper()
	resp, err := client.Get(srv.URL + "/export/excel?query=" + query)
	if err != nil {
		t.Fatalf("GET /export/excel?query=%s: %v", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /export/excel?query=%s = %d, want 200", query, resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "spreadsheetml") {
		t.Fatalf("Content-Type = %q, want spreadsheetml.sheet", ct)
	}
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return bb
}

// sheetRowCount returns the number of <row> elements on the named sheet.
// The sheet file is resolved via workbook.xml (sheet name → r:id) and
// workbook.xml.rels (r:id → worksheets/sheetN.xml).
func sheetRowCount(t *testing.T, xlsx []byte, sheet string) int {
	t.Helper()

	wb := excelPart(t, xlsx, "xl/workbook.xml")
	sheetRe := regexp.MustCompile(`<sheet\b[^>]*name="` + regexp.QuoteMeta(sheet) + `"[^>]*\br:id="([^"]+)"`)
	m := sheetRe.FindStringSubmatch(wb)
	if m == nil {
		// r:id may precede name in attribute order; try the other order.
		sheetRe = regexp.MustCompile(`<sheet\b[^>]*\br:id="([^"]+)"[^>]*name="` + regexp.QuoteMeta(sheet) + `"`)
		m = sheetRe.FindStringSubmatch(wb)
	}
	if m == nil {
		t.Fatalf("sheet %q not found in workbook.xml", sheet)
	}
	rid := m[1]

	rels := excelPart(t, xlsx, "xl/_rels/workbook.xml.rels")
	relRe := regexp.MustCompile(`<Relationship\b[^>]*Id="` + rid + `"[^>]*Target="([^"]+)"`)
	m = relRe.FindStringSubmatch(rels)
	if m == nil {
		t.Fatalf("relationship %s for sheet %q not found", rid, sheet)
	}
	target := "xl/" + strings.TrimPrefix(m[1], "/")

	xml := excelPart(t, xlsx, target)
	return len(regexp.MustCompile(`<row\b`).FindAllString(xml, -1))
}

// checkPivotCache asserts the pivot-cache invariants on a generated export
// whose data sheet holds `totalRows` rows including the header: the cache
// source points at the written range and refreshOnLoad is set. The
// template's cached shared items/records are kept verbatim (recordCount
// stays consistent with them) — Excel rebuilds the cache on open.
func checkPivotCache(t *testing.T, xlsx []byte, sheet, lastCol string, totalRows int) {
	t.Helper()

	wantRef := fmt.Sprintf(`ref="A1:%s%d"`, lastCol, totalRows)

	def := excelPart(t, xlsx, "xl/pivotCache/pivotCacheDefinition1.xml")
	if !strings.Contains(def, wantRef) {
		t.Errorf("pivot cache definition lacks %s\nfirst 600 bytes: %.600s", wantRef, def)
	}
	if !strings.Contains(def, fmt.Sprintf(`sheet="%s"`, sheet)) {
		t.Errorf("pivot cache definition lacks sheet=%q", sheet)
	}
	if !strings.Contains(def, `refreshOnLoad="1"`) {
		t.Error("pivot cache definition lacks refreshOnLoad=\"1\"")
	}
	// Template cache kept: recordCount must still match the cached records
	// count (Excel rejects a cache whose declared size differs from the
	// records part).
	rc := regexp.MustCompile(`recordCount="(\d+)"`).FindStringSubmatch(def)
	if rc == nil {
		t.Error("pivot cache definition lost recordCount")
	}
	recs := excelPart(t, xlsx, "xl/pivotCache/pivotCacheRecords1.xml")
	cnt := regexp.MustCompile(`count="(\d+)"`).FindStringSubmatch(recs)
	if cnt == nil {
		t.Fatal("pivot cache records lost count attribute")
	}
	if rc != nil && cnt != nil && rc[1] != cnt[1] {
		t.Errorf("recordCount=%s but records count=%s: inconsistent cache", rc[1], cnt[1])
	}
}

// TestExportExcelRegistrePivotCache proves the registre export rewrites its
// pivot cache to the exported range and forces a refresh on open.
func TestExportExcelRegistrePivotCache(t *testing.T) {
	seedExcelExportFixtures(t)

	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	xlsx := downloadExcel(t, srv, client, "registre_detail")

	rows := sheetRowCount(t, xlsx, "animals")
	if rows < 2 {
		t.Fatalf("expected header + at least 1 data row, sheet has %d rows", rows)
	}
	checkPivotCache(t, xlsx, "animals", "AG", rows)
}

// TestExportExcelStatCommunesPivotCache proves the stats export rewrites both
// its pivot cache and the _xlnm._FilterDatabase defined name.
func TestExportExcelStatCommunesPivotCache(t *testing.T) {
	seedExcelExportFixtures(t)

	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	xlsx := downloadExcel(t, srv, client, "stat_communes")

	rows := sheetRowCount(t, xlsx, "bdd")
	if rows < 2 {
		t.Fatalf("expected header + at least 1 data row, sheet has %d rows", rows)
	}
	checkPivotCache(t, xlsx, "bdd", "N", rows)

	wantFilter := fmt.Sprintf("bdd!$A$1:$N$%d", rows)
	wb := excelPart(t, xlsx, "xl/workbook.xml")
	if !strings.Contains(wb, wantFilter) {
		t.Errorf("workbook.xml lacks updated _FilterDatabase %q", wantFilter)
	}
	if strings.Contains(wb, "bdd!$A$1:$N$3696") {
		t.Error("workbook.xml still carries the stale template _FilterDatabase range")
	}
	// Other sheets' filter ranges untouched.
	if !strings.Contains(wb, "Commune!$A$1:$F$560") {
		t.Error("workbook.xml lost the unrelated Commune _FilterDatabase range")
	}
}

// TestExportExcelRequiresLogin proves the excel export route stays behind
// the auth middleware like the other export routes.
func TestExportExcelRequiresLogin(t *testing.T) {
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(srv.URL + "/export/excel?query=registre_detail")
	if err != nil {
		t.Fatalf("GET /export/excel: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("unauthenticated GET = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/auth/new") {
		t.Fatalf("redirect = %q, want /auth/new", loc)
	}
}

// seedExcelExportFixtures inserts the minimum join graph the export queries
// need (animal + intake + discovery + discoverer + age). Idempotent-ish: uses
// a dedicated year so repeated runs don't break counts.
func seedExcelExportFixtures(t *testing.T) {
	t.Helper()
	const markerYear = 2098

	const (
		ageID = "11111111-1111-1111-1111-111111111111"
		dscID = "22222222-2222-2222-2222-222222222222"
		dcvID = "33333333-3333-3333-3333-333333333333"
		intID = "44444444-4444-4444-4444-444444444444"
		typID = "55555555-5555-5555-5555-555555555555"
	)
	// Clean up any partial seed from a previous failed run.
	cleanup := []string{
		"DELETE FROM animals WHERE year = " + fmt.Sprint(markerYear),
		"DELETE FROM intakes WHERE id = '" + intID + "'",
		"DELETE FROM discoveries WHERE id = '" + dscID + "'",
		"DELETE FROM discoverers WHERE id = '" + dcvID + "'",
		"DELETE FROM animalages WHERE id = '" + ageID + "'",
		"DELETE FROM animaltypes WHERE id = '" + typID + "'",
	}
	for _, s := range cleanup {
		if err := models.DB.RawQuery(s).Exec(); err != nil {
			t.Fatalf("cleanup fixture %q: %v", s, err)
		}
	}

	stmts := []string{
		"INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + ageID + "', 'exceltest', 0, NOW(), NOW())",
		"INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + typID + "', 'exceltest', 0, NOW(), NOW())",
		"INSERT INTO discoverers (id, firstname, lastname, created_at, updated_at) VALUES ('" + dcvID + "', 'excel', 'tester', NOW(), NOW())",
		"INSERT INTO discoveries (id, date, location, reason, discoverer_id, created_at, updated_at) VALUES ('" + dscID + "', NOW(), 'Testville', 'test', '" + dcvID + "', NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intID + "', NOW(), NOW(), NOW())",
		`INSERT INTO animals (year, yearNumber, species, gender, cage, IntakeDate,
			animalage_id, animaltype_id, intake_id, discovery_id, created_at, updated_at)
		 VALUES (?, 1, 'Test species', 'M', 'C1', NOW(),
			'` + ageID + `', '` + typID + `', '` + intID + `', '` + dscID + `', NOW(), NOW())`,
	}
	for _, s := range stmts {
		var err error
		if strings.Contains(s, "?") {
			err = models.DB.RawQuery(s, markerYear).Exec()
		} else {
			err = models.DB.RawQuery(s).Exec()
		}
		if err != nil {
			t.Fatalf("seed fixture %q: %v", s, err)
		}
	}
}
