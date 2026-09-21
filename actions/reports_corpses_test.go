package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
)

// Corpse register tests (issue #149). Uses a marker year so fixtures are
// isolated from real data and easy to clean up.

const corpseTestYear = 2097

type corpseFixture struct {
	deadOuttakeID     string
	aliveOuttakeID    string
	deadAnimalNumber  int
	aliveAnimalNumber int
	destination       string
}

func seedCorpseFixtures(t *testing.T) *corpseFixture {
	t.Helper()
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	mkUUID := func(i byte) string {
		return fmt.Sprintf("aa000000-0000-0000-0000-%012d", i)
	}
	deadTypeID := mkUUID(1)
	aliveTypeID := mkUUID(2)
	deadOuttakeID := mkUUID(3)
	aliveOuttakeID := mkUUID(4)
	ageID := mkUUID(5)
	typID := mkUUID(6)
	dscID := mkUUID(7)
	dcvID := mkUUID(8)
	intDead := mkUUID(9)
	intAlive := mkUUID(10)
	destination := "TS-CorpseDest-" + uuid.Must(uuid.NewV4()).String()[:8]

	f := &corpseFixture{
		deadOuttakeID:     deadOuttakeID,
		aliveOuttakeID:    aliveOuttakeID,
		deadAnimalNumber:  9001,
		aliveAnimalNumber: 9002,
		destination:       destination,
	}

	cleanup := func() {
		stmts := []string{
			"DELETE FROM animals WHERE year = " + fmt.Sprint(corpseTestYear),
			"DELETE FROM outtakes WHERE id IN ('" + deadOuttakeID + "','" + aliveOuttakeID + "')",
			"DELETE FROM outtaketypes WHERE id IN ('" + deadTypeID + "','" + aliveTypeID + "')",
			"DELETE FROM intakes WHERE id IN ('" + intDead + "','" + intAlive + "')",
			"DELETE FROM discoveries WHERE id = '" + dscID + "'",
			"DELETE FROM discoverers WHERE id = '" + dcvID + "'",
			"DELETE FROM animalages WHERE id = '" + ageID + "'",
			"DELETE FROM animaltypes WHERE id = '" + typID + "'",
		}
		for _, s := range stmts {
			models.DB.RawQuery(s).Exec()
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	stmts := []string{
		// dead outtake type (generates a corpse) and a regular one
		"INSERT INTO outtaketypes (id, name, `def`, dead, error, rating, location_mode, created_at, updated_at) VALUES ('" + deadTypeID + "', 'corpse-test-dead', 0, 1, 0, 0, 'free', NOW(), NOW())",
		"INSERT INTO outtaketypes (id, name, `def`, dead, error, rating, location_mode, created_at, updated_at) VALUES ('" + aliveTypeID + "', 'corpse-test-alive', 0, 0, 0, 0, 'free', NOW(), NOW())",
		// outtakes: one dead-type, one alive-type
		"INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + deadOuttakeID + "', NOW(), '" + deadTypeID + "', NOW(), NOW())",
		"INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + aliveOuttakeID + "', NOW(), '" + aliveTypeID + "', NOW(), NOW())",
		"INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + ageID + "', 'corpsetest', 0, NOW(), NOW())",
		"INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + typID + "', 'corpsetest', 0, NOW(), NOW())",
		"INSERT INTO discoverers (id, firstname, lastname, created_at, updated_at) VALUES ('" + dcvID + "', 'corpse', 'tester', NOW(), NOW())",
		"INSERT INTO discoveries (id, date, location, reason, discoverer_id, created_at, updated_at) VALUES ('" + dscID + "', NOW(), 'Corpseville', 'test', '" + dcvID + "', NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intDead + "', NOW(), NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intAlive + "', NOW(), NOW(), NOW())",
		// dead animal (outtake = dead type) and released animal (outtake = alive type)
		"INSERT INTO animals (year, yearNumber, species, IntakeDate, animalage_id, animaltype_id, intake_id, discovery_id, outtake_id, created_at, updated_at) VALUES (" + fmt.Sprint(corpseTestYear) + ", " + fmt.Sprint(f.deadAnimalNumber) + ", 'Corpus testus', NOW(), '" + ageID + "', '" + typID + "', '" + intDead + "', '" + dscID + "', '" + deadOuttakeID + "', NOW(), NOW())",
		"INSERT INTO animals (year, yearNumber, species, IntakeDate, animalage_id, animaltype_id, intake_id, discovery_id, outtake_id, created_at, updated_at) VALUES (" + fmt.Sprint(corpseTestYear) + ", " + fmt.Sprint(f.aliveAnimalNumber) + ", 'Vivus testus', NOW(), '" + ageID + "', '" + typID + "', '" + intAlive + "', '" + dscID + "', '" + aliveOuttakeID + "', NOW(), NOW())",
	}
	for _, s := range stmts {
		if err := models.DB.RawQuery(s).Exec(); err != nil {
			t.Fatalf("seed corpse fixture %q: %v", s, err)
		}
	}
	return f
}

// TestCorpseRegisterQuery: the register lists only dead-outtake animals of
// the requested year with species, entry and death dates (issue #149).
func TestCorpseRegisterQuery(t *testing.T) {
	f := seedCorpseFixtures(t)
	tx := searchTestDB(t)

	rows, err := listCorpseRows(tx, fmt.Sprint(corpseTestYear))
	if err != nil {
		t.Fatalf("listCorpseRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 (live animals must be excluded)", len(rows))
	}
	r := rows[0]
	if r.YearNumber != f.deadAnimalNumber {
		t.Errorf("YearNumber = %d, want %d", r.YearNumber, f.deadAnimalNumber)
	}
	if r.Species != "Corpus testus" {
		t.Errorf("Species = %q, want %q", r.Species, "Corpus testus")
	}
	if r.OuttakeID.String() != f.deadOuttakeID {
		t.Errorf("OuttakeID = %s, want %s", r.OuttakeID, f.deadOuttakeID)
	}
	if r.CorpseDestination.Valid {
		t.Errorf("CorpseDestination = %q, want NULL", r.CorpseDestination.String)
	}
	// other years untouched
	rows, err = listCorpseRows(tx, "2096")
	if err != nil {
		t.Fatalf("listCorpseRows(2096): %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("year filter leaked: got %d rows for 2096", len(rows))
	}
}

// markCorpse posts the bulk-mark form.
func markCorpse(t *testing.T, client *http.Client, baseURL string, f *corpseFixture, ids ...string) *http.Response {
	t.Helper()
	form := url.Values{}
	for _, id := range ids {
		form.Add("outtake_ids", id)
	}
	form.Set("destination", f.destination)
	form.Set("year", fmt.Sprint(corpseTestYear))
	req, err := http.NewRequest("POST", baseURL+"/reports/corpses/mark", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST mark: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestCorpseRegisterUnmark: admin can clear a corpse mark; non-admin gets
// 403; unmark only touches dead-type outtakes.
func TestCorpseRegisterUnmark(t *testing.T) {
	f := seedCorpseFixtures(t)
	client, baseURL := adminClientWithURL(t)

	// mark first
	resp := markCorpse(t, client, baseURL, f, f.deadOuttakeID)
	if resp.StatusCode >= 400 {
		t.Fatalf("mark status = %d", resp.StatusCode)
	}
	dead := &models.Outtake{}
	if err := models.DB.Find(dead, f.deadOuttakeID); err != nil {
		t.Fatalf("load dead outtake: %v", err)
	}
	if !dead.CorpseDestination.Valid {
		t.Fatal("precondition: outtake must be marked")
	}

	// non-admin may not unmark
	userLogin, userPass := feedingGuideUser(t, false)
	uclient, ubaseURL := feedingGuideLogin(t, userLogin, userPass)
	form := url.Values{"outtake_ids": {f.deadOuttakeID}, "year": {fmt.Sprint(corpseTestYear)}}
	resp, err := uclient.PostForm(ubaseURL+"/reports/corpses/unmark", form)
	if err != nil {
		t.Fatalf("POST unmark (non-admin): %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-admin unmark status = %d, want 403", resp.StatusCode)
	}
	if err := models.DB.Find(dead, f.deadOuttakeID); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !dead.CorpseDestination.Valid {
		t.Error("non-admin unmark must not clear the destination")
	}

	// admin unmark clears destination/date/recorder
	resp = unmarkCorpse(t, client, baseURL, f.deadOuttakeID)
	if resp.StatusCode >= 400 {
		t.Fatalf("admin unmark status = %d", resp.StatusCode)
	}
	dead = &models.Outtake{}
	if err := models.DB.Find(dead, f.deadOuttakeID); err != nil {
		t.Fatalf("reload after unmark: %v", err)
	}
	if dead.CorpseDestination.Valid || dead.CorpseDestinationAt.Valid || dead.CorpseDestinationByID.Valid {
		t.Errorf("unmark must clear all mark fields, got dest=%v at=%v by=%v",
			dead.CorpseDestination, dead.CorpseDestinationAt, dead.CorpseDestinationByID)
	}
}

// TestCorpseRegisterMarkBackRedirect: the mark/unmark handlers honor the
// optional `back` param (used by the animal sheet corpse block, issue
// #199-6) via safeRedirectTarget; unsafe values fall back to the report.
func TestCorpseRegisterMarkBackRedirect(t *testing.T) {
	f := seedCorpseFixtures(t)
	client, baseURL := adminClientWithURL(t)

	post := func(path, back string) *http.Response {
		t.Helper()
		form := url.Values{}
		form.Add("outtake_ids", f.deadOuttakeID)
		form.Set("destination", f.destination)
		form.Set("back", back)
		form.Set("year", fmt.Sprint(corpseTestYear))
		req, err := http.NewRequest("POST", baseURL+path, strings.NewReader(form.Encode()))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	// mark with a local back target: redirect Location must be that target
	resp := post("/reports/corpses/mark", "/animals/123#nav-outtake")
	if loc := resp.Header.Get("Location"); loc != "/animals/123#nav-outtake" {
		t.Errorf("mark Location = %q, want back target", loc)
	}

	// unmark with an unsafe back target: must fall back to the report URL
	resp = post("/reports/corpses/unmark", "https://evil.example.org/x")
	want := "/reports/corpses?year=" + fmt.Sprint(corpseTestYear)
	if loc := resp.Header.Get("Location"); loc != want {
		t.Errorf("unmark Location = %q, want fallback %q", loc, want)
	}
}

// TestCorpseRegisterFiltersAndMarkOnce (issue #199-7): default view shows
// only unmarked corpses; column filters narrow the list; a second mark on
// an already-marked outtake is a no-op (value unchanged).
func TestCorpseRegisterFiltersAndMarkOnce(t *testing.T) {
	f := seedCorpseFixtures(t)
	client, baseURL := adminClientWithURL(t)
	year := fmt.Sprint(corpseTestYear)

	// mark the dead outtake once
	resp := markCorpse(t, client, baseURL, f, f.deadOuttakeID)
	if resp.StatusCode >= 400 {
		t.Fatalf("mark status = %d", resp.StatusCode)
	}

	// default view (no marked param): marked row must be hidden
	resp, err := client.Get(baseURL + "/reports/corpses?year=" + year)
	if err != nil {
		t.Fatalf("GET default: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(page), f.destination) {
		t.Errorf("default view must hide marked corpses")
	}

	// marked=all: row visible again
	resp, err = client.Get(baseURL + "/reports/corpses?year=" + year + "&marked=all")
	if err != nil {
		t.Fatalf("GET marked=all: %v", err)
	}
	page, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), f.destination) {
		t.Errorf("marked=all view must show the marked corpse")
	}

	// species filter that cannot match: row hidden
	resp, err = client.Get(baseURL + "/reports/corpses?year=" + year + "&marked=all&f_species=NoSuchSpeciesZZZ")
	if err != nil {
		t.Fatalf("GET species filter: %v", err)
	}
	page, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(page), f.destination) {
		t.Errorf("species filter must hide non-matching rows")
	}

	// mark-once: a second mark with a different destination must not
	// overwrite the existing one
	form := url.Values{}
	form.Add("outtake_ids", f.deadOuttakeID)
	form.Set("destination", f.destination+"-CHANGED")
	form.Set("year", year)
	req, err := http.NewRequest("POST", baseURL+"/reports/corpses/mark", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("POST re-mark: %v", err)
	}
	resp.Body.Close()
	dead := &models.Outtake{}
	if err := models.DB.Find(dead, f.deadOuttakeID); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if dead.CorpseDestination.String != f.destination {
		t.Errorf("re-mark must not overwrite destination, got %q want %q",
			dead.CorpseDestination.String, f.destination)
	}
}

// unmarkCorpse posts the unmark form for the given outtake ids.
func unmarkCorpse(t *testing.T, client *http.Client, baseURL string, ids ...string) *http.Response {
	t.Helper()
	form := url.Values{}
	for _, id := range ids {
		form.Add("outtake_ids", id)
	}
	form.Set("year", fmt.Sprint(corpseTestYear))
	req, err := http.NewRequest("POST", baseURL+"/reports/corpses/unmark", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST unmark: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestCorpseRegisterMark: bulk marking stores destination + date + current
// user on dead outtakes only (issue #149).
func TestCorpseRegisterMark(t *testing.T) {
	f := seedCorpseFixtures(t)
	client, baseURL := adminClientWithURL(t)

	// report lists the dead animal, not the live one
	resp, err := client.Get(baseURL + "/reports/corpses?year=" + fmt.Sprint(corpseTestYear))
	if err != nil {
		t.Fatalf("GET report: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("report status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(page), fmt.Sprint(f.deadAnimalNumber)) {
		t.Errorf("report page does not contain dead animal number %d", f.deadAnimalNumber)
	}
	if strings.Contains(string(page), fmt.Sprint(f.aliveAnimalNumber)) {
		t.Errorf("report page must not list live animal %d", f.aliveAnimalNumber)
	}

	// bulk mark: dead + alive selected; only the dead one may be updated
	resp = markCorpse(t, client, baseURL, f, f.deadOuttakeID, f.aliveOuttakeID)
	if resp.StatusCode >= 400 {
		t.Fatalf("mark status = %d", resp.StatusCode)
	}

	dead := &models.Outtake{}
	if err := models.DB.Find(dead, f.deadOuttakeID); err != nil {
		t.Fatalf("load dead outtake: %v", err)
	}
	if !dead.CorpseDestination.Valid || dead.CorpseDestination.String != f.destination {
		t.Errorf("corpse_destination = %v, want %q", dead.CorpseDestination, f.destination)
	}
	if !dead.CorpseDestinationAt.Valid {
		t.Errorf("corpse_destination_at not set")
	}
	if !dead.CorpseDestinationByID.Valid {
		t.Errorf("corpse_destination_by_id not set (must be the recording user)")
	}

	alive := &models.Outtake{}
	if err := models.DB.Find(alive, f.aliveOuttakeID); err != nil {
		t.Fatalf("load alive outtake: %v", err)
	}
	if alive.CorpseDestination.Valid || alive.CorpseDestinationAt.Valid || alive.CorpseDestinationByID.Valid {
		t.Errorf("non-dead outtake must stay unmarked, got dest=%v at=%v by=%v",
			alive.CorpseDestination, alive.CorpseDestinationAt, alive.CorpseDestinationByID)
	}

	// marked values displayed on the report (default view hides marked
	// rows — issue #199-7 — so request marked=all explicitly)
	resp, err = client.Get(baseURL + "/reports/corpses?year=" + fmt.Sprint(corpseTestYear) + "&marked=all")
	if err != nil {
		t.Fatalf("GET report after mark: %v", err)
	}
	page, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(page), f.destination) {
		t.Errorf("report page must display the destination after marking")
	}

	// suggestions endpoint returns the previously used value
	resp, err = client.Get(baseURL + "/suggestions/corpse_destination?q=" + url.QueryEscape(f.destination))
	if err != nil {
		t.Fatalf("GET suggestions: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), f.destination) {
		t.Errorf("suggestions response %q must contain %q", body, f.destination)
	}

	// CSV export contains the new columns
	resp, err = client.Get(baseURL + "/reports/corpses/export.csv?year=" + fmt.Sprint(corpseTestYear))
	if err != nil {
		t.Fatalf("GET export: %v", err)
	}
	csvBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(csvBody), f.destination) {
		t.Errorf("CSV export must contain the destination column value")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("export status = %d, want 200", resp.StatusCode)
	}
}
