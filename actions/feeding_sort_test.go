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
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestFeedingIndexSortOrder: on /feeding, animals due at the same minute are
// ordered by species, then feeding instruction, then animal number (#197
// sub-item 7).
func TestFeedingIndexSortOrder(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	must := func(err error) {
		t.Helper()
		require.NoError(t, err)
	}

	// Three animals sharing the same zone and the same feeding window, hence
	// the same next-feeding time. Expected row order:
	//   Asp/alpha (base+2), Asp/beta (base+1), Bsp/alpha (base)
	// species beats instruction, instruction beats number.
	numBase := 900000
	for _, b := range []byte(marker) {
		numBase = numBase*31 + int(b)
	}
	numBase = 900000 + numBase%9000
	specs := []struct {
		species string
		feeding string
		number  int
	}{
		{"FSA-" + marker, "beta instruction", numBase + 1},
		{"FSA-" + marker, "alpha instruction", numBase + 2},
		{"FSB-" + marker, "alpha instruction", numBase},
	}

	spIDs := make([]string, 0, 2)
	for _, s := range []string{"FSA-" + marker, "FSB-" + marker} {
		sp := models.Species{
			ID:             s,
			Species:        "Feeding sort species " + s,
			CreavesSpecies: s,
			Class:          "Aves",
		}
		must(tx.Create(&sp))
		spIDs = append(spIDs, sp.ID)
	}
	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "FSType-" + marker}
	must(tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "FSAge-" + marker}
	must(tx.Create(&aa))
	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	must(tx.Create(&in))
	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("FS-" + marker)}
	must(tx.Create(&disc))
	d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: disc.ID}
	must(tx.Create(&d))

	now := time.Now()
	// NOTE: the MySQL DSN uses parseTime with the default UTC location, so a
	// time.Time is stored as its UTC wall clock and read back identically.
	// Building the window in UTC keeps 00:05 as the stored wall clock, which
	// calculateFeeding recomputes to today 00:05 local — always due.
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, time.UTC)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 55, 0, 0, time.UTC)

	animalIDs := make([]int, 0, len(specs))
	numbers := make([]int, 0, len(specs))
	for _, spec := range specs {
		a := models.Animal{
			Year:          now.Year(),
			YearNumber:    spec.number,
			Species:       spec.species,
			AnimaltypeID:  at.ID,
			AnimalageID:   aa.ID,
			DiscoveryID:   d.ID,
			IntakeID:      in.ID,
			IntakeDate:    now,
			Zone:          nulls.NewString("FeedSortZone-" + marker),
			Feeding:       nulls.NewString(spec.feeding),
			FeedingStart:  nulls.NewTime(start),
			FeedingEnd:    nulls.NewTime(end),
			FeedingPeriod: 60,
		}
		must(tx.Create(&a))
		animalIDs = append(animalIDs, a.ID)
		numbers = append(numbers, spec.number)
	}

	t.Cleanup(func() {
		for _, id := range animalIDs {
			tx.RawQuery("DELETE FROM animals WHERE id = ?", id).Exec()
		}
		tx.RawQuery("DELETE FROM discoveries WHERE id = ?", d.ID).Exec()
		tx.RawQuery("DELETE FROM discoverers WHERE id = ?", disc.ID).Exec()
		tx.RawQuery("DELETE FROM intakes WHERE id = ?", in.ID).Exec()
		for _, id := range spIDs {
			tx.RawQuery("DELETE FROM species WHERE id = ?", id).Exec()
		}
		tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)
	resp, err := client.Get(baseURL + "/feeding")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", truncate(body, 800))
	html := string(body)

	// Rows must appear in species → instruction → number order.
	pos := make([]int, 0, len(numbers))
	for _, n := range numbers {
		idx := strings.Index(html, fmt.Sprintf(">%d<", n))
		require.NotEqual(t, -1, idx, "number %d not rendered", n)
		pos = append(pos, idx)
	}
	require.Less(t, pos[1], pos[0], "species tie: 'alpha instruction' before 'beta instruction'")
	require.Less(t, pos[0], pos[2], "species 'FSA' before 'FSB'")
}
