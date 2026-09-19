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

	// marked values displayed on the report
	resp, err = client.Get(baseURL + "/reports/corpses?year=" + fmt.Sprint(corpseTestYear))
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
