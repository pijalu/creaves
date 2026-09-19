package actions

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// registertableFixture builds one full animal chain whose species carries a
// class, whose discovery carries postal code/city/entry cause and whose
// free-mode outtake carries a precise location — everything the register
// table gained with #197 sub-item 5.
type registertableFixture struct {
	year       int
	yearNumber int
	species    string // creaves_species value
	class      string
	entryCause models.EntryCause
	city       string
	postalCode string
	precise    string
	animalID   int
}

func createRegistertableFixture(t *testing.T, tx *pop.Connection) registertableFixture {
	t.Helper()
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	must := func(err error) {
		t.Helper()
		require.NoError(t, err)
	}

	sp := models.Species{
		ID:             "rtsp-" + marker,
		Species:        "Register species " + marker,
		CreavesSpecies: "RT-SP-" + marker,
		Class:          "Aves",
		Order:          "order",
		Family:         "family",
		NativeStatus:   "NS1",
	}
	must(tx.Create(&sp))

	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "RTType-" + marker}
	must(tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "RTAge-" + marker}
	must(tx.Create(&aa))

	ec := models.EntryCause{
		ID:    "RTC-" + marker,
		Cause: "Register cause " + marker,
		// The detail column feeds the new register "cause detail" column.
		Detail: "Cause detail " + marker,
	}
	must(tx.Create(&ec))

	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("RT-" + marker)}
	must(tx.Create(&disc))
	d := models.Discovery{
		ID:           uuid.Must(uuid.NewV4()),
		Date:         time.Now(),
		DiscovererID: disc.ID,
		EntryCauseID: ec.ID,
		PostalCode:   nulls.NewString("5000"),
		City:         nulls.NewString("RegisterCity-" + marker),
	}
	must(tx.Create(&d))
	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	must(tx.Create(&in))

	ot := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "RTOOT-" + marker, LocationMode: models.OuttakeLocationModeFree}
	must(tx.Create(&ot))

	ynBase := 990000
	for _, b := range []byte(marker) {
		ynBase = ynBase*31 + int(b)
	}
	yearNumber := 990000 + ynBase%9000
	a := models.Animal{
		Year:         2030,
		YearNumber:   yearNumber,
		Species:      sp.CreavesSpecies,
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now(),
	}
	must(tx.Create(&a))

	o := models.Outtake{
		ID:              uuid.Must(uuid.NewV4()),
		Date:            time.Now(),
		TypeID:          ot.ID,
		Location:        nulls.NewString("Register site"),
		PreciseLocation: nulls.NewString("1 rue du Registre, " + marker),
	}
	must(tx.Create(&o))
	must(tx.RawQuery("UPDATE animals SET outtake_id = ? WHERE id = ?", o.ID, a.ID).Exec())

	t.Cleanup(func() {
		tx.RawQuery("UPDATE animals SET outtake_id = NULL WHERE id = ?", a.ID).Exec()
		tx.RawQuery("DELETE FROM outtakes WHERE id = ?", o.ID).Exec()
		tx.RawQuery("DELETE FROM animals WHERE id = ?", a.ID).Exec()
		tx.RawQuery("DELETE FROM discoveries WHERE id = ?", d.ID).Exec()
		tx.RawQuery("DELETE FROM discoverers WHERE id = ?", disc.ID).Exec()
		tx.RawQuery("DELETE FROM intakes WHERE id = ?", in.ID).Exec()
		tx.RawQuery("DELETE FROM species WHERE id = ?", sp.ID).Exec()
		tx.RawQuery("DELETE FROM outtaketypes WHERE id = ?", ot.ID).Exec()
		tx.RawQuery("DELETE FROM entry_causes WHERE id = ?", ec.ID).Exec()
		tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
	})

	return registertableFixture{
		year:       a.Year,
		yearNumber: a.YearNumber,
		species:    sp.CreavesSpecies,
		class:      sp.Class,
		entryCause: ec,
		city:       d.City.String,
		postalCode: d.PostalCode.String,
		precise:    o.PreciseLocation.String,
		animalID:   a.ID,
	}
}

// readRegisterCSV fetches the register CSV export for the fixture year and
// returns the parsed rows (semicolon-separated per writeCSV).
func readRegisterCSV(t *testing.T, client *http.Client, baseURL string, f registertableFixture) [][]string {
	t.Helper()
	resp, err := client.Get(fmt.Sprintf("%s/registertable/ExportCSV?year=%d", baseURL, f.year))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", truncate(body, 500))

	r := csv.NewReader(strings.NewReader(string(body)))
	r.Comma = ';'
	records, err := r.ReadAll()
	require.NoError(t, err)
	return records
}

// TestRegistertableCSVHasNewColumns: the register CSV gains class (replacing
// type), discovery postal code/city, entry-cause id/detail and the outtake
// precise location (#197 sub-item 5).
func TestRegistertableCSVHasNewColumns(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createRegistertableFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	records := readRegisterCSV(t, client, baseURL, f)
	require.GreaterOrEqual(t, len(records), 2, "header + at least one row")

	header := records[0]
	require.Contains(t, header, "Class")
	require.NotContains(t, header, "Type")
	require.Contains(t, header, "Postal code")
	require.Contains(t, header, "City")
	require.Contains(t, header, "Entry cause")
	require.Contains(t, header, "Cause detail")
	require.Contains(t, header, "Precise location")

	// The fixture animal must appear with the new values filled in.
	var row []string
	for _, r := range records[1:] {
		if r[0] == fmt.Sprintf("%d", f.yearNumber) {
			row = r
			break
		}
	}
	require.NotNil(t, row, "fixture animal not found in CSV")

	byHeader := map[string]string{}
	for i, h := range header {
		byHeader[h] = row[i]
	}
	require.Equal(t, f.class, byHeader["Class"], "class from species table")
	require.Equal(t, f.postalCode, byHeader["Postal code"])
	require.Equal(t, f.city, byHeader["City"])
	require.Equal(t, f.entryCause.ID, byHeader["Entry cause"])
	require.Equal(t, f.entryCause.Detail, byHeader["Cause detail"])
	require.Equal(t, "Register site", byHeader["Location"])
	require.Equal(t, f.precise, byHeader["Precise location"])
}

// TestRegistertableHTMLShowsNewColumns: the register page renders the class
// (instead of the animal type), the entry-cause id/detail and the precise
// location of the outtake (#197 sub-item 5).
func TestRegistertableHTMLShowsNewColumns(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createRegistertableFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	resp, err := client.Get(fmt.Sprintf("%s/registertable?year=%d", baseURL, f.year))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", truncate(body, 800))
	html := string(body)

	// Header: Class replaced Type; the new columns exist.
	require.Contains(t, html, ">Class<")
	require.NotContains(t, html, ">Type<")
	require.Contains(t, html, ">Postal code<")
	require.Contains(t, html, ">Entry cause<")
	require.Contains(t, html, ">Precise location<")

	// The fixture row carries the new values.
	for _, want := range []string{
		f.class,
		f.postalCode,
		f.city,
		f.entryCause.ID,
		f.entryCause.Detail,
		f.precise,
	} {
		require.Contains(t, html, want)
	}
}
