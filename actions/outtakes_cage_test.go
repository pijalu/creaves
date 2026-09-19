package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// setCageOnFixture puts a fixture animal into a cage so several fixture
// animals can share one cage (issue #170).
func setCageOnFixture(t *testing.T, tx *pop.Connection, animalID int, cage string) {
	t.Helper()
	require.NoError(t, tx.RawQuery("UPDATE animals SET cage = ? WHERE id = ?", cage, animalID).Exec())
}

// postOuttakeCage submits the batch cage-outtake form through the full app
// (real middleware stack incl. CSRF).
func postOuttakeCage(t *testing.T, client *http.Client, baseURL, cage, date string, typeID uuid.UUID) *http.Response {
	t.Helper()

	resp, err := client.Get(baseURL + "/outtakes/cage?cage=" + url.QueryEscape(cage))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET cage form: %s", truncate(body, 500))
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no authenticity_token on /outtakes/cage")

	form := url.Values{
		"cage":               {cage},
		"Outtake.Date":       {date},
		"TypeID":             {typeID.String()},
		"authenticity_token": {string(m[1])},
	}
	resp, err = client.PostForm(baseURL+"/outtakes/cage", form)
	require.NoError(t, err)
	return resp
}

// TestOuttakeCageBatchCreate: one submit creates one outtake per in-care
// animal of the cage, each with its own stay duration computed from its own
// intake date; already-out animals are untouched (issue #170).
func TestOuttakeCageBatchCreate(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	cage := "CAGE-" + uuid.Must(uuid.NewV4()).String()[:8]
	f1 := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	f2 := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	setCageOnFixture(t, tx, f1.animalID, cage)
	setCageOnFixture(t, tx, f2.animalID, cage)

	// Distinct intake dates so the two stay durations must differ: the
	// batch must compute per animal, not reuse one value.
	a2 := &models.Animal{}
	require.NoError(t, tx.Find(a2, f2.animalID))
	in2 := &models.Intake{}
	require.NoError(t, tx.Find(in2, a2.IntakeID))
	in2.Date = time.Date(2026, 1, 18, 22, 0, 0, 0, time.UTC)
	require.NoError(t, tx.Update(in2))

	client, baseURL := adminClientWithURL(t)

	// Picker renders.
	resp, err := client.Get(baseURL + "/outtakes/cage")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "/outtakes/cage")

	// Cage form lists both animals.
	resp, err = client.Get(baseURL + "/outtakes/cage?cage=" + url.QueryEscape(cage))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), fmt.Sprintf("%d/%02d", f1.animalYearNumber, f1.animalYear%100))
	require.Contains(t, string(body), fmt.Sprintf("%d/%02d", f2.animalYearNumber, f2.animalYear%100))

	// Batch save.
	resp = postOuttakeCage(t, client, baseURL, cage, "2026/01/20 10:00", f1.outtakeTypeID)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(body, 800))

	// One outtake per animal, type from the first animal, stay duration
	// computed per animal (f1: intake after the outtake date → clamped 0;
	// f2: intake 2026-01-18 22:00 → exactly 36h).
	cnt, err := tx.Where("outtaketype_id = ?", f1.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Equal(t, 2, cnt)

	durations := []int{}
	for _, f := range []outtakeRulesFixture{f1, f2} {
		a := &models.Animal{}
		require.NoError(t, tx.Find(a, f.animalID))
		require.True(t, a.OuttakeID.Valid, "animal %d must be linked to its outtake", f.animalID)

		o := &models.Outtake{}
		require.NoError(t, tx.Find(o, a.OuttakeID.UUID))
		require.True(t, o.StayDuration.Valid, "stay duration must be persisted")

		in := &models.Intake{}
		require.NoError(t, tx.Find(in, a.IntakeID))
		want := int(o.Date.Sub(in.Date).Hours())
		if want < 0 {
			want = 0
		}
		require.Equal(t, want, o.StayDuration.Int, "animal %d stay duration", f.animalID)
		durations = append(durations, o.StayDuration.Int)
	}
	// The two animals have different intake dates: durations differ,
	// proving the batch computed one value per animal.
	require.ElementsMatch(t, []int{0, 36}, durations)

	// The cage no longer offers any animal: redirect with a warning flash.
	resp, err = client.Get(baseURL + "/outtakes/cage?cage=" + url.QueryEscape(cage))
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "empty cage must redirect, not render a form")
}

// TestOuttakeCageCreateRejectsFutureDate: a cage outtake dated in the future
// is refused and nothing is persisted (issue #170).
func TestOuttakeCageCreateRejectsFutureDate(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	cage := "CAGE-" + uuid.Must(uuid.NewV4()).String()[:8]
	f1 := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	f2 := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	setCageOnFixture(t, tx, f1.animalID, cage)
	setCageOnFixture(t, tx, f2.animalID, cage)

	client, baseURL := adminClientWithURL(t)
	resp := postOuttakeCage(t, client, baseURL, cage, "2030/01/15 10:00", f1.outtakeTypeID)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(body, 800))

	cnt, err := tx.Where("outtaketype_id = ?", f1.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Zero(t, cnt, "no outtake may be persisted for a rejected batch")

	for _, f := range []outtakeRulesFixture{f1, f2} {
		a := &models.Animal{}
		require.NoError(t, tx.Find(a, f.animalID))
		require.False(t, a.OuttakeID.Valid, "animal %d must stay in care", f.animalID)
	}
}

// TestOuttakeCageSuggestions lists cages needing an outtake through the
// existing suggestion endpoint used by the picker.
func TestOuttakeCageSuggestions(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	cage := "CAGES-" + uuid.Must(uuid.NewV4()).String()[:8]
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	setCageOnFixture(t, tx, f.animalID, cage)

	client, baseURL := adminClientWithURL(t)
	resp, err := client.Get(baseURL + "/suggestions/CageWithAnimalInCare?q=" + url.QueryEscape(cage))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), cage)
}
