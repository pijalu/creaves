package actions

import (
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

// quickOuttakeFixture builds two animals for one unique year: one still in
// care (no outtake) and one already released — the two states the
// quick-outtake buttons have to tell apart (#197 sub-item 6).
type quickOuttakeFixture struct {
	year       int
	freeNumber int // year number of the animal without outtake
	doneNumber int // year number of the animal with an outtake
	freeID     int
	doneID     int
}

func createQuickOuttakeFixture(t *testing.T, tx *pop.Connection) quickOuttakeFixture {
	t.Helper()
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	must := func(err error) {
		t.Helper()
		require.NoError(t, err)
	}

	sp := models.Species{
		ID:             "qosp-" + marker,
		Species:        "Quick species " + marker,
		CreavesSpecies: "QO-SP-" + marker,
		Class:          "Aves",
	}
	must(tx.Create(&sp))
	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "QOType-" + marker}
	must(tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "QOAge-" + marker}
	must(tx.Create(&aa))

	// A unique year keeps the /animals?year= result down to the fixture rows.
	ynBase := 7
	for _, b := range []byte(marker) {
		ynBase = ynBase*31 + int(b)
	}
	year := 2031 + ynBase%18 // 2031..2048
	numBase := 100000 + ynBase%100000

	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	must(tx.Create(&in))
	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("QO-" + marker)}
	must(tx.Create(&disc))
	d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: disc.ID}
	must(tx.Create(&d))

	free := models.Animal{
		Year:         year,
		YearNumber:   numBase,
		Species:      sp.CreavesSpecies,
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now(),
	}
	must(tx.Create(&free))
	done := models.Animal{
		Year:         year,
		YearNumber:   numBase + 1,
		Species:      sp.CreavesSpecies,
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now(),
	}
	must(tx.Create(&done))

	ot := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "QOOT-" + marker, LocationMode: models.OuttakeLocationModeFree}
	must(tx.Create(&ot))
	o := models.Outtake{
		ID:       uuid.Must(uuid.NewV4()),
		Date:     time.Now(),
		TypeID:   ot.ID,
		Location: nulls.NewString("Quick site"),
	}
	must(tx.Create(&o))
	must(tx.RawQuery("UPDATE animals SET outtake_id = ? WHERE id = ?", o.ID, done.ID).Exec())

	t.Cleanup(func() {
		tx.RawQuery("UPDATE animals SET outtake_id = NULL WHERE id = ?", done.ID).Exec()
		tx.RawQuery("DELETE FROM outtakes WHERE id = ?", o.ID).Exec()
		// NOTE: "id IN (?)" with two args would bind both args to ONE
		// placeholder and fail silently — spell the placeholders out.
		tx.RawQuery("DELETE FROM animals WHERE id IN (?, ?)", free.ID, done.ID).Exec()
		tx.RawQuery("DELETE FROM discoveries WHERE id = ?", d.ID).Exec()
		tx.RawQuery("DELETE FROM discoverers WHERE id = ?", disc.ID).Exec()
		tx.RawQuery("DELETE FROM intakes WHERE id = ?", in.ID).Exec()
		tx.RawQuery("DELETE FROM species WHERE id = ?", sp.ID).Exec()
		tx.RawQuery("DELETE FROM outtaketypes WHERE id = ?", ot.ID).Exec()
		tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
	})

	return quickOuttakeFixture{
		year:       year,
		freeNumber: free.YearNumber,
		doneNumber: done.YearNumber,
		freeID:     free.ID,
		doneID:     done.ID,
	}
}

// quickOuttakeLink is the href the quick-outtake buttons build for an animal.
func quickOuttakeLink(number, year int) string {
	return fmt.Sprintf(`href="/outtakes/new?animal_year_number=%d/%d"`, number, year%100)
}

// fetchPageGET performs an authenticated GET and returns the body.
func fetchPageGET(t *testing.T, client *http.Client, url string) string {
	t.Helper()
	resp, err := client.Get(url)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", truncate(body, 800))
	return string(body)
}

// TestQuickOuttakeButtonsFollowOuttakeState: animals without an outtake get
// the quick "create outtake" button on the list page and on their own page,
// animals with an outtake must not (#197 sub-item 6).
func TestQuickOuttakeButtonsFollowOuttakeState(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	freeLink := quickOuttakeLink(f.freeNumber, f.year)
	doneLink := quickOuttakeLink(f.doneNumber, f.year)
	require.True(t, strings.Contains(freeLink, "/outtakes/new?animal_year_number="))

	// List page: button for the animal in care, none for the released one.
	listHTML := fetchPageGET(t, client, fmt.Sprintf("%s/animals?year=%d&per_page=100", baseURL, f.year))
	require.Contains(t, listHTML, freeLink)
	require.NotContains(t, listHTML, doneLink)

	// Show page of the animal in care: button present.
	freeHTML := fetchPageGET(t, client, fmt.Sprintf("%s/animals/%d", baseURL, f.freeID))
	require.Contains(t, freeHTML, freeLink)

	// Show page of the released animal: button absent.
	doneHTML := fetchPageGET(t, client, fmt.Sprintf("%s/animals/%d", baseURL, f.doneID))
	require.NotContains(t, doneHTML, doneLink)
}
