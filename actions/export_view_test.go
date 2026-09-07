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
}

// TestExportViewUnknownQuery proves an unknown query id does not crash and
// returns 404.
func TestExportViewUnknownQuery(t *testing.T) {
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

// TestReportsNavShowsExportsEntry proves the Reports dropdown in the layout
// exposes a single "Exports" entry pointing at /export/view, and no longer
// links the removed CSV chooser page.
func TestReportsNavShowsExportsEntry(t *testing.T) {
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
