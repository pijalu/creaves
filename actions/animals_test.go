package actions

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// searchTestDB returns a shared connection to the MySQL test database
// (database.yml "test"). Skips when the database is unreachable. The
// connection is shared across tests (single connection pool) to avoid
// exhausting MySQL max_connections.
var (
	searchSharedDB     *pop.Connection
	searchSharedDBErr  error
	searchSharedDBOnce sync.Once
)

func searchTestDB(t *testing.T) *pop.Connection {
	t.Helper()
	searchSharedDBOnce.Do(func() {
		// cap the pool: unlimited pools exhaust MySQL max_connections when
		// the driver recycles connections under readTimeout.
		cd := &pop.ConnectionDetails{
			Dialect:  "mysql",
			URL:      "mysql://creaves:creaves@(localhost:3306)/creaves_test?parseTime=true&multiStatements=true&readTimeout=3s",
			Pool:     20,
			IdlePool: 2,
		}
		searchSharedDB, searchSharedDBErr = pop.NewConnection(cd)
		if searchSharedDBErr != nil {
			return
		}
		searchSharedDBErr = searchSharedDB.Open()
	})
	if searchSharedDBErr != nil {
		t.Skipf("test database unavailable: %v", searchSharedDBErr)
	}
	if err := searchSharedDB.RawQuery("SELECT 1").Exec(); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return searchSharedDB
}

// animalSearchFixtures builds an isolated fixture graph tagged with a unique
// marker so tests never depend on pre-existing rows.
type animalSearchFixtures struct {
	marker string

	animaltype1 uuid.UUID
	animaltype2 uuid.UUID
	animalage1  uuid.UUID
	animalage2  uuid.UUID
	entryCause1 string
	entryCause2 string
	outtakeOK   uuid.UUID // error = false
	outtakeErr  uuid.UUID // error = true

	// matching animals
	animalA int // year 2021, type1, age1, cause1, species "Testsp Alpha", ring "RNG-<m>-001", outtake OK
	animalB int // year 2022, type2, age2, cause2, species "Testsp Beta", ring "ZZZ-<m>-002", outtake ERR
	animalC int // year 2021, type1, age1, cause1, species "Testsp Alpha", no ring, no outtake

	discovererIDs []uuid.UUID
	discoveryIDs  []uuid.UUID
	intakeIDs     []uuid.UUID
	outtakeIDs    []uuid.UUID
}

func createAnimalSearchFixtures(t *testing.T, tx *pop.Connection) *animalSearchFixtures {
	t.Helper()
	f := &animalSearchFixtures{marker: uuid.Must(uuid.NewV4()).String()[:8]}

	// unique (year, yearNumber) index: derive a fixture-unique base number
	// from the marker (hex) so fixtures never collide, even leftover ones.
	ynBase := 900000
	for _, b := range []byte(f.marker) {
		ynBase = ynBase*31 + int(b)
	}
	ynBase = 900000 + ynBase%90000 // keep clear of small production-like numbers

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("fixture creation failed: %v", err)
		}
	}

	// reference data
	at1 := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "TSType1-" + f.marker}
	at2 := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "TSType2-" + f.marker}
	must(tx.Create(&at1))
	must(tx.Create(&at2))
	f.animaltype1, f.animaltype2 = at1.ID, at2.ID

	aa1 := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "TSAge1-" + f.marker}
	aa2 := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "TSAge2-" + f.marker}
	must(tx.Create(&aa1))
	must(tx.Create(&aa2))
	f.animalage1, f.animalage2 = aa1.ID, aa2.ID

	f.entryCause1 = "TSC1-" + f.marker
	f.entryCause2 = "TSC2-" + f.marker
	ec1 := models.EntryCause{ID: f.entryCause1, Cause: "Cause1 " + f.marker, Detail: "d1", Nature: "n1", Indication: "i1"}
	ec2 := models.EntryCause{ID: f.entryCause2, Cause: "Cause2 " + f.marker, Detail: "d2", Nature: "n2", Indication: "i2"}
	must(tx.Create(&ec1))
	must(tx.Create(&ec2))

	otOK := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "TSOutOK-" + f.marker, Error: false}
	otErr := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "TSOutErr-" + f.marker, Error: true}
	must(tx.Create(&otOK))
	must(tx.Create(&otErr))
	f.outtakeOK, f.outtakeErr = otOK.ID, otErr.ID

	mkAnimal := func(year, yearNumber int, atID, aaID uuid.UUID, causeID, species string, ring nulls.String, outtaketypeID *uuid.UUID) int {
		t.Helper()
		disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("TS-" + f.marker)}
		must(tx.Create(&disc))
		f.discovererIDs = append(f.discovererIDs, disc.ID)
		d := models.Discovery{
			ID:           uuid.Must(uuid.NewV4()),
			Date:         time.Now(),
			DiscovererID: disc.ID,
			EntryCauseID: causeID,
		}
		must(tx.Create(&d))
		f.discoveryIDs = append(f.discoveryIDs, d.ID)
		in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
		must(tx.Create(&in))
		f.intakeIDs = append(f.intakeIDs, in.ID)

		var outtakeID nulls.UUID
		if outtaketypeID != nil {
			o := models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), TypeID: *outtaketypeID}
			must(tx.Create(&o))
			f.outtakeIDs = append(f.outtakeIDs, o.ID)
			outtakeID = nulls.NewUUID(o.ID)
		}

		a := models.Animal{
			Year:         year,
			YearNumber:   yearNumber,
			Species:      species,
			Ring:         ring,
			AnimaltypeID: atID,
			AnimalageID:  aaID,
			DiscoveryID:  d.ID,
			IntakeID:     in.ID,
			OuttakeID:    outtakeID,
			IntakeDate:   time.Now(),
		}
		must(tx.Create(&a))
		return a.ID
	}

	f.animalA = mkAnimal(2021, ynBase+1, f.animaltype1, f.animalage1, f.entryCause1, "Testsp Alpha "+f.marker,
		nulls.NewString("RNG-"+f.marker+"-001"), &f.outtakeOK)
	f.animalB = mkAnimal(2022, ynBase+2, f.animaltype2, f.animalage2, f.entryCause2, "Testsp Beta "+f.marker,
		nulls.NewString("ZZZ-"+f.marker+"-002"), &f.outtakeErr)
	f.animalC = mkAnimal(2021, ynBase+3, f.animaltype1, f.animalage1, f.entryCause1, "Testsp Alpha "+f.marker,
		nulls.String{}, nil)

	t.Cleanup(func() {
		// clean only the fixture rows (children first)
		for _, id := range []int{f.animalA, f.animalB, f.animalC} {
			tx.RawQuery("DELETE FROM animals WHERE id = ?", id).Exec()
		}
		for _, id := range f.outtakeIDs {
			tx.RawQuery("DELETE FROM outtakes WHERE id = ?", id).Exec()
		}
		for _, id := range f.discoveryIDs {
			tx.RawQuery("DELETE FROM discoveries WHERE id = ?", id).Exec()
		}
		for _, id := range f.discovererIDs {
			tx.RawQuery("DELETE FROM discoverers WHERE id = ?", id).Exec()
		}
		for _, id := range f.intakeIDs {
			tx.RawQuery("DELETE FROM intakes WHERE id = ?", id).Exec()
		}
		tx.RawQuery("DELETE FROM animaltypes WHERE id IN (?, ?)", f.animaltype1, f.animaltype2).Exec()
		tx.RawQuery("DELETE FROM animalages WHERE id IN (?, ?)", f.animalage1, f.animalage2).Exec()
		tx.RawQuery("DELETE FROM entry_causes WHERE id IN (?, ?)", f.entryCause1, f.entryCause2).Exec()
		tx.RawQuery("DELETE FROM outtaketypes WHERE id IN (?, ?)", f.outtakeOK, f.outtakeErr).Exec()
	})

	return f
}

// runSearch applies the filters and returns the matching animal IDs within
// the fixture scope (restricted by species marker to stay isolated).
func runSearch(t *testing.T, tx *pop.Connection, f *animalSearchFixtures, p animalSearchParams) map[int]bool {
	t.Helper()
	// scope to fixture rows via species marker when no explicit species filter
	q := tx.Q()
	if p.Species == "" {
		q = q.Where("animals.species LIKE ?", "Testsp %"+f.marker)
	}
	q, err := applyAnimalSearchFilters(tx, "", q, p)
	if err != nil {
		t.Fatalf("applyAnimalSearchFilters: %v", err)
	}
	var animals models.Animals
	if err := q.All(&animals); err != nil {
		t.Fatalf("query: %v", err)
	}
	ids := map[int]bool{}
	for _, a := range animals {
		ids[a.ID] = true
	}
	return ids
}

func idsMatch(t *testing.T, got map[int]bool, want ...int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %d results %v, got %v", len(want), want, got)
	}
	for _, w := range want {
		if !got[w] {
			t.Fatalf("expected animal %d in results %v", w, got)
		}
	}
}

func TestAnimalSearchFilterYear(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	ids := runSearch(t, tx, f, animalSearchParams{Year: "2021"})
	idsMatch(t, ids, f.animalA, f.animalC)

	ids = runSearch(t, tx, f, animalSearchParams{Year: "2022"})
	idsMatch(t, ids, f.animalB)
}

func TestAnimalSearchFilterAnimaltype(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	ids := runSearch(t, tx, f, animalSearchParams{AnimaltypeID: f.animaltype1.String()})
	idsMatch(t, ids, f.animalA, f.animalC)

	ids = runSearch(t, tx, f, animalSearchParams{AnimaltypeID: f.animaltype2.String()})
	idsMatch(t, ids, f.animalB)
}

func TestAnimalSearchFilterSpecies(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	ids := runSearch(t, tx, f, animalSearchParams{Species: "Testsp Alpha " + f.marker})
	idsMatch(t, ids, f.animalA, f.animalC)

	ids = runSearch(t, tx, f, animalSearchParams{Species: "Testsp Beta " + f.marker})
	idsMatch(t, ids, f.animalB)

	ids = runSearch(t, tx, f, animalSearchParams{Species: "Testsp Nope " + f.marker})
	idsMatch(t, ids)
}

func TestAnimalSearchFilterEntryCause(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	ids := runSearch(t, tx, f, animalSearchParams{EntryCauseID: f.entryCause1})
	idsMatch(t, ids, f.animalA, f.animalC)

	ids = runSearch(t, tx, f, animalSearchParams{EntryCauseID: f.entryCause2})
	idsMatch(t, ids, f.animalB)
}

func TestAnimalSearchFilterAnimalage(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	ids := runSearch(t, tx, f, animalSearchParams{AnimalageID: f.animalage1.String()})
	idsMatch(t, ids, f.animalA, f.animalC)

	ids = runSearch(t, tx, f, animalSearchParams{AnimalageID: f.animalage2.String()})
	idsMatch(t, ids, f.animalB)
}

func TestAnimalSearchFilterRingPartial(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	// partial match on the marker substring
	ids := runSearch(t, tx, f, animalSearchParams{Ring: f.marker})
	idsMatch(t, ids, f.animalA, f.animalB)

	// partial match prefix
	ids = runSearch(t, tx, f, animalSearchParams{Ring: "RNG-"})
	idsMatch(t, ids, f.animalA)

	// no match
	ids = runSearch(t, tx, f, animalSearchParams{Ring: "NOPE-" + f.marker})
	idsMatch(t, ids)
}

func TestAnimalSearchFilterOuttaketypeExcludesErrors(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	// non-error outtake type matches animalA
	ids := runSearch(t, tx, f, animalSearchParams{OuttaketypeID: f.outtakeOK.String()})
	idsMatch(t, ids, f.animalA)

	// error outtake type must NOT match animalB (error outtakes excluded)
	ids = runSearch(t, tx, f, animalSearchParams{OuttaketypeID: f.outtakeErr.String()})
	idsMatch(t, ids)
}

func TestAnimalSearchFiltersANDCombined(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	// year + type + species + cause + age: only animalA matches all
	ids := runSearch(t, tx, f, animalSearchParams{
		Year:         "2021",
		AnimaltypeID: f.animaltype1.String(),
		Species:      "Testsp Alpha " + f.marker,
		EntryCauseID: f.entryCause1,
		AnimalageID:  f.animalage1.String(),
		Ring:         "RNG-",
	})
	idsMatch(t, ids, f.animalA)

	// contradictory combination yields nothing
	ids = runSearch(t, tx, f, animalSearchParams{
		Year:         "2021",
		AnimaltypeID: f.animaltype2.String(),
	})
	idsMatch(t, ids)

	// outtake type + year AND-combined
	ids = runSearch(t, tx, f, animalSearchParams{
		Year:          "2021",
		OuttaketypeID: f.outtakeOK.String(),
	})
	idsMatch(t, ids, f.animalA)
}

func TestAnimalSearchInvalidYear(t *testing.T) {
	tx := searchTestDB(t)
	q := tx.Q()
	if _, err := applyAnimalSearchFilters(tx, "", q, animalSearchParams{Year: "abc"}); err == nil {
		t.Fatalf("expected error for invalid year filter")
	}
}

func TestAnimalSearchCSVHeaderKeysInLocales(t *testing.T) {
	// handler builds headers via T.Translate on these keys; they must exist
	// in every animals locale file.
	keys := []string{
		"animal.search.csv.year", "animal.search.csv.number", "animal.search.csv.type",
		"animal.search.csv.species", "animal.search.csv.gender", "animal.search.csv.age",
		"animal.search.csv.ring", "animal.search.csv.intake_date",
		"animal.search.csv.entry_cause", "animal.search.csv.entry_cause_detail",
		"animal.search.csv.exit_date", "animal.search.csv.exit_reason",
		"animal.search.csv.zone", "animal.search.csv.cage",
	}
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		b, err := os.ReadFile(fmt.Sprintf("../locales/animals.%s.yaml", lang))
		if err != nil {
			t.Fatalf("read locale %s: %v", lang, err)
		}
		content := string(b)
		for _, key := range keys {
			if !strings.Contains(content, `id: "`+key+`"`) {
				t.Errorf("locale animals.%s.yaml missing key %s", lang, key)
			}
		}
	}
}
