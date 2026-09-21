package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// rulesTestOuttaketypes mirrors the reference matrix from bugs.md:
// OT1 excludes NS2/NS3/NS4 (free location), OT4 excludes NS3 (list),
// OT6 excludes NS1/NS3 (list); the other types have no rule.
func rulesTestOuttaketypes() models.Outtaketypes {
	return models.Outtaketypes{
		{ID: uuid.Must(uuid.NewV4()), Name: "OT1", ExcludedNativeStatuses: nulls.NewString("NS2,NS3,NS4"), LocationMode: models.OuttakeLocationModeFree},
		{ID: uuid.Must(uuid.NewV4()), Name: "OT2"},
		{ID: uuid.Must(uuid.NewV4()), Name: "OT3"},
		{ID: uuid.Must(uuid.NewV4()), Name: "OT4", ExcludedNativeStatuses: nulls.NewString("NS3"), LocationMode: models.OuttakeLocationModeList},
		{ID: uuid.Must(uuid.NewV4()), Name: "OT5"},
		{ID: uuid.Must(uuid.NewV4()), Name: "OT6", ExcludedNativeStatuses: nulls.NewString("NS1,NS3"), LocationMode: models.OuttakeLocationModeList},
	}
}

func outtakeTypeNames(ts *models.Outtaketypes) []string {
	names := []string{}
	for _, ot := range *ts {
		names = append(names, ot.Name)
	}
	return names
}

// TestFilterOuttaketypesForNativeStatus covers the bug matrix: which outcome
// types stay selectable per species native status.
func TestFilterOuttaketypesForNativeStatus(t *testing.T) {
	cases := []struct {
		nativeStatus string
		allowed      []string
	}{
		{"", []string{"OT1", "OT2", "OT3", "OT4", "OT5", "OT6"}},
		{"NS1", []string{"OT1", "OT2", "OT3", "OT4", "OT5"}},        // OT6 blocked
		{"NS2", []string{"OT2", "OT3", "OT4", "OT5", "OT6"}},        // OT1 blocked
		{"NS3", []string{"OT2", "OT3", "OT5"}},                      // OT1, OT4, OT6 blocked
		{"NS4", []string{"OT2", "OT3", "OT4", "OT5", "OT6"}},        // OT1 blocked
		{"NS5", []string{"OT1", "OT2", "OT3", "OT4", "OT5", "OT6"}}, // nothing excludes NS5
	}
	for _, tc := range cases {
		t.Run("ns="+tc.nativeStatus, func(t *testing.T) {
			all := rulesTestOuttaketypes()
			got := filterOuttaketypesForNativeStatus(&all, tc.nativeStatus)
			require.ElementsMatch(t, tc.allowed, outtakeTypeNames(got))
		})
	}

	// Empty native status must return the original slice untouched.
	all := rulesTestOuttaketypes()
	require.Equal(t, len(all), len(*filterOuttaketypesForNativeStatus(&all, "")))
}

// TestOuttaketypeExcludesNativeStatus pins the CSV parsing rules.
func TestOuttaketypeExcludesNativeStatus(t *testing.T) {
	ot := models.Outtaketype{ExcludedNativeStatuses: nulls.NewString("NS2, NS3 ,NS4,")}
	require.True(t, ot.ExcludesNativeStatus("NS2"))
	require.True(t, ot.ExcludesNativeStatus("NS3"))
	require.True(t, ot.ExcludesNativeStatus("NS4"))
	require.False(t, ot.ExcludesNativeStatus("NS1"))
	require.False(t, ot.ExcludesNativeStatus(""))
	require.Equal(t, []string{"NS2", "NS3", "NS4"}, ot.ExcludedNativeStatusList())

	empty := models.Outtaketype{}
	require.False(t, empty.ExcludesNativeStatus("NS1"))
	require.Empty(t, empty.ExcludedNativeStatusList())

	blank := models.Outtaketype{ExcludedNativeStatuses: nulls.NewString("")}
	require.False(t, blank.ExcludesNativeStatus("NS1"))
}

// TestEnforceOuttakeLocationRule exercises the server-side location rule
// against a real reference option row.
func TestEnforceOuttakeLocationRule(t *testing.T) {
	tx := searchTestDB(t)
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	optName := "TSO-" + marker
	opt := models.OuttakeLocationOption{ID: uuid.Must(uuid.NewV4()), Name: optName}
	require.NoError(t, tx.Create(&opt))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM outtake_location_options WHERE id = ?", opt.ID).Exec()
	})

	noneType := &models.Outtaketype{Name: "none", LocationMode: models.OuttakeLocationModeNone}
	listType := &models.Outtaketype{Name: "list", LocationMode: models.OuttakeLocationModeList}
	freeType := &models.Outtaketype{Name: "free", LocationMode: models.OuttakeLocationModeFree}

	// none: any submitted location is silently dropped.
	o := &models.Outtake{Location: nulls.NewString("should be cleared"), PreciseLocation: nulls.NewString("should be cleared")}
	require.NoError(t, enforceOuttakeLocationRule(tx, o, noneType))
	require.False(t, o.Location.Valid)
	require.False(t, o.PreciseLocation.Valid, "precise location only applies to free mode (#197-4)")

	// empty mode behaves like none (pre-migration rows).
	o = &models.Outtake{Location: nulls.NewString("should be cleared"), PreciseLocation: nulls.NewString("should be cleared")}
	require.NoError(t, enforceOuttakeLocationRule(tx, o, &models.Outtaketype{Name: "legacy"}))
	require.False(t, o.Location.Valid)
	require.False(t, o.PreciseLocation.Valid)

	// list: missing location rejected.
	require.Error(t, enforceOuttakeLocationRule(tx, &models.Outtake{}, listType))

	// list: value not in the reference list rejected.
	o = &models.Outtake{Location: nulls.NewString("nope-" + marker)}
	require.Error(t, enforceOuttakeLocationRule(tx, o, listType))

	// list: value from the reference list accepted, precise location cleared.
	o = &models.Outtake{Location: nulls.NewString(optName), PreciseLocation: nulls.NewString("12 rue du Test")}
	require.NoError(t, enforceOuttakeLocationRule(tx, o, listType))
	require.False(t, o.PreciseLocation.Valid)

	// free: arbitrary text accepted, precise location preserved.
	o = &models.Outtake{Location: nulls.NewString("anywhere"), PreciseLocation: nulls.NewString("12 rue du Test")}
	require.NoError(t, enforceOuttakeLocationRule(tx, o, freeType))
	require.True(t, o.PreciseLocation.Valid)
	require.Equal(t, "12 rue du Test", o.PreciseLocation.String)
}

// TestSpeciesNativeStatus checks the species → native status lookup used by
// the new-outtake filtering.
func TestSpeciesNativeStatus(t *testing.T) {
	tx := searchTestDB(t)
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	sp := models.Species{
		ID:             "sns-" + marker,
		Species:        "Species " + marker,
		CreavesSpecies: "TS-NS-" + marker,
		Class:          "class",
		Order:          "order",
		Family:         "family",
		NativeStatus:   "NS3",
	}
	require.NoError(t, tx.Create(&sp))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE ID = ?", sp.ID).Exec()
	})

	require.Equal(t, "NS3", speciesNativeStatus(tx, sp.CreavesSpecies))
	require.Equal(t, "", speciesNativeStatus(tx, "TS-NS-unknown-"+marker))
	require.Equal(t, "", speciesNativeStatus(tx, ""))
}

// outtakeRulesAnimalFixture creates a minimal animal (+ graph) whose species
// maps to a native status, plus an outtake type with the given rule. It uses
// models.DB because the full app serves requests through popmw(models.DB).
type outtakeRulesFixture struct {
	animalID         int
	animalYear       int
	animalYearNumber int
	outtakeTypeID    uuid.UUID
}

func createOuttakeRulesFixture(t *testing.T, tx *pop.Connection, excludedNS, locationMode string) outtakeRulesFixture {
	t.Helper()
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	must := func(err error) {
		t.Helper()
		require.NoError(t, err)
	}

	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "TSOType-" + marker}
	must(tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "TSOAge-" + marker}
	must(tx.Create(&aa))
	ec := models.EntryCause{ID: "TSOC-" + marker, Cause: "Cause " + marker}
	must(tx.Create(&ec))
	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("TSO-" + marker)}
	must(tx.Create(&disc))
	d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: disc.ID, EntryCauseID: ec.ID}
	must(tx.Create(&d))
	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	must(tx.Create(&in))

	sp := models.Species{
		ID:             "ots-" + marker,
		Species:        "Outtake rule species " + marker,
		CreavesSpecies: "TSO-SP-" + marker,
		Class:          "class",
		Order:          "order",
		Family:         "family",
		NativeStatus:   "NS3",
	}
	must(tx.Create(&sp))

	ot := models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "TSOOT-" + marker, LocationMode: locationMode}
	if excludedNS != "" {
		ot.ExcludedNativeStatuses = nulls.NewString(excludedNS)
	}
	must(tx.Create(&ot))

	ynBase := 950000
	for _, b := range []byte(marker) {
		ynBase = ynBase*31 + int(b)
	}
	ynBase = 950000 + ynBase%40000
	a := models.Animal{
		Year:         2026,
		YearNumber:   ynBase,
		Species:      sp.CreavesSpecies,
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now(),
	}
	must(tx.Create(&a))

	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM animals WHERE id = ?", a.ID).Exec()
		tx.RawQuery("DELETE FROM outtakes WHERE outtaketype_id = ?", ot.ID).Exec()
		tx.RawQuery("DELETE FROM discoveries WHERE id = ?", d.ID).Exec()
		tx.RawQuery("DELETE FROM discoverers WHERE id = ?", disc.ID).Exec()
		tx.RawQuery("DELETE FROM intakes WHERE id = ?", in.ID).Exec()
		tx.RawQuery("DELETE FROM species WHERE ID = ?", sp.ID).Exec()
		tx.RawQuery("DELETE FROM outtaketypes WHERE id = ?", ot.ID).Exec()
		tx.RawQuery("DELETE FROM entry_causes WHERE id = ?", ec.ID).Exec()
		tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
	})

	return outtakeRulesFixture{animalID: a.ID, animalYear: a.Year, animalYearNumber: a.YearNumber, outtakeTypeID: ot.ID}
}

// postOuttake submits the new-outtake form for the fixture animal through
// the full app (real middleware stack, incl. CSRF). It fetches the new form
// first to obtain a session cookie and a valid authenticity_token.
// postOuttakeAt submits the new-outtake form with an explicit date
// ("2006/01/02 15:04"). postOuttake is the historical date-only variant.
func postOuttakeAt(t *testing.T, client *http.Client, baseURL string, f outtakeRulesFixture, date, location string, extra ...url.Values) *http.Response {
	t.Helper()

	resp, err := client.Get(fmt.Sprintf("%s/outtakes/new?animal_year_number=%d/%02d", baseURL, f.animalYearNumber, f.animalYear%100))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET new form: %s", truncate(body, 500))
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no authenticity_token on /outtakes/new")

	form := url.Values{
		"Outtake.Date":       {date},
		"TypeID":             {f.outtakeTypeID.String()},
		"animal_id":          {strconv.Itoa(f.animalID)},
		"animal_ring":        {""},
		"authenticity_token": {string(m[1])},
	}
	if location != "" {
		form.Set("Location", location)
	}
	if len(extra) > 0 {
		for k, vs := range extra[0] {
			for _, v := range vs {
				form.Add(k, v)
			}
		}
	}
	resp, err = client.PostForm(baseURL+"/outtakes/", form)
	require.NoError(t, err)
	return resp
}

// postOuttakeFormAt posts the new-outtake form with extra fields (e.g.
// PreciseLocation) on top of the standard ones.
func postOuttakeFormAt(t *testing.T, client *http.Client, baseURL string, f outtakeRulesFixture, date string, extra url.Values) *http.Response {
	t.Helper()
	return postOuttakeAt(t, client, baseURL, f, date, "", extra)
}

func postOuttake(t *testing.T, client *http.Client, baseURL string, f outtakeRulesFixture, location string) *http.Response {
	t.Helper()
	return postOuttakeAt(t, client, baseURL, f, "2026/01/15", location)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}

// TestOuttakePreciseLocationRule: free-mode outtake types persist the precise
// address ("Adresse, lieu précis", #197 sub-item 4); non-free types drop it
// server-side even when the client submits one.
func TestOuttakePreciseLocationRule(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	t.Run("free mode persists", func(t *testing.T) {
		f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeFree)
		client, baseURL := adminClientWithURL(t)

		resp := postOuttakeFormAt(t, client, baseURL, f, "2026/01/15 12:00", url.Values{
			"Location":        {"anywhere"},
			"PreciseLocation": {"12 rue du Test, 5000 Namur"},
		})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(body, 800))

		o := &models.Outtake{}
		require.NoError(t, tx.Where("outtaketype_id = ?", f.outtakeTypeID).First(o))
		require.True(t, o.Location.Valid)
		require.Equal(t, "anywhere", o.Location.String)
		require.True(t, o.PreciseLocation.Valid, "precise location must be persisted for free-mode types")
		require.Equal(t, "12 rue du Test, 5000 Namur", o.PreciseLocation.String)
	})

	t.Run("list mode clears", func(t *testing.T) {
		marker := uuid.Must(uuid.NewV4()).String()[:8]
		opt := models.OuttakeLocationOption{ID: uuid.Must(uuid.NewV4()), Name: "TSOPL-" + marker}
		require.NoError(t, tx.Create(&opt))
		t.Cleanup(func() {
			tx.RawQuery("DELETE FROM outtake_location_options WHERE id = ?", opt.ID).Exec()
		})

		f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeList)
		client, baseURL := adminClientWithURL(t)

		resp := postOuttakeFormAt(t, client, baseURL, f, "2026/01/15 12:00", url.Values{
			"Location":        {opt.Name},
			"PreciseLocation": {"12 rue du Test, 5000 Namur"},
		})
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(body, 800))

		o := &models.Outtake{}
		require.NoError(t, tx.Where("outtaketype_id = ?", f.outtakeTypeID).First(o))
		require.Equal(t, opt.Name, o.Location.String)
		require.False(t, o.PreciseLocation.Valid, "precise location must be dropped for list-mode types")
	})
}

// TestOuttakeCreateRejectsTypeForbiddenByNativeStatus posts an outtake whose
// type excludes the animal species' native status as a NON-admin user: the
// server must refuse with 422 and must not persist anything.
func TestOuttakeCreateRejectsTypeForbiddenByNativeStatus(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "NS3", models.OuttakeLocationModeNone)
	login, password := feedingGuideUser(t, false)
	client, baseURL := feedingGuideLogin(t, login, password)

	resp := postOuttake(t, client, baseURL, f, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", truncate(body, 800))

	cnt, err := tx.Where("outtaketype_id = ?", f.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Zero(t, cnt, "no outtake must be persisted for a forbidden type")

	var animal models.Animal
	require.NoError(t, tx.Find(&animal, f.animalID))
	require.False(t, animal.OuttakeID.Valid, "animal must not be linked to a rejected outtake")
}

// TestOuttakeCreateAdminBypassesNativeStatus (#199-12): an admin may use any
// outtake type with any species regardless of the species native status — the
// NS exclusion rule must not block the creation.
func TestOuttakeCreateAdminBypassesNativeStatus(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "NS3", models.OuttakeLocationModeNone)
	client, baseURL := adminClientWithURL(t)

	resp := postOuttake(t, client, baseURL, f, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NotEqual(t, http.StatusUnprocessableEntity, resp.StatusCode,
		"admin must not be blocked by the NS exclusion, body: %s", truncate(body, 800))

	cnt, err := tx.Where("outtaketype_id = ?", f.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Equal(t, 1, cnt, "admin outtake must be persisted despite the NS exclusion")

	var animal models.Animal
	require.NoError(t, tx.Find(&animal, f.animalID))
	require.True(t, animal.OuttakeID.Valid, "animal must be linked to the admin-created outtake")
}

// TestOuttakeCreateRejectsLocationOutsideList posts an outtake with a
// list-mode type and a location that is not a reference option: 422.
func TestOuttakeCreateRejectsLocationOutsideList(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	opt := models.OuttakeLocationOption{ID: uuid.Must(uuid.NewV4()), Name: "TSOL-" + marker}
	require.NoError(t, tx.Create(&opt))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM outtake_location_options WHERE id = ?", opt.ID).Exec()
	})

	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeList)
	client, baseURL := adminClientWithURL(t)

	// Location not in the reference list → rejected.
	resp := postOuttake(t, client, baseURL, f, "bogus-"+marker)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", truncate(body, 800))

	// No location at all → rejected too.
	resp = postOuttake(t, client, baseURL, f, "")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", truncate(body, 800))

	cnt, err := tx.Where("outtaketype_id = ?", f.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Zero(t, cnt)
}

// TestOuttakeCreateRejectsIndigenatType: "ID d'indigénat" is an entry cause
// and must never be accepted as an outtake cause (issue #175). The type is
// stored in the reference table, so the guard lives in Outtake.Validate.
func TestOuttakeCreateRejectsIndigenatType(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)

	indig := models.Outtaketype{
		ID:           uuid.Must(uuid.NewV4()),
		Name:         "ID d'indigénat TSOI-" + uuid.Must(uuid.NewV4()).String()[:8],
		LocationMode: models.OuttakeLocationModeNone,
	}
	require.NoError(t, tx.Create(&indig))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM outtaketypes WHERE id = ?", indig.ID).Exec()
	})

	client, baseURL := adminClientWithURL(t)
	f.outtakeTypeID = indig.ID
	resp := postOuttake(t, client, baseURL, f, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", truncate(body, 800))
	require.Contains(t, string(body), "indigénat")

	cnt, err := tx.Where("outtaketype_id = ?", indig.ID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Zero(t, cnt, "no outtake must be persisted with the indigénat type")

	var animal models.Animal
	require.NoError(t, tx.Find(&animal, f.animalID))
	require.False(t, animal.OuttakeID.Valid)
}

// TestOuttakeCreateRejectsFutureDate: an outtake dated in the future must be
// refused with 422 and nothing persisted (issue #175).
func TestOuttakeCreateRejectsFutureDate(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	client, baseURL := adminClientWithURL(t)

	resp := postOuttakeAt(t, client, baseURL, f, "2030/01/15 10:00", "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, "body: %s", truncate(body, 800))

	cnt, err := tx.Where("outtaketype_id = ?", f.outtakeTypeID).Count(&models.Outtake{})
	require.NoError(t, err)
	require.Zero(t, cnt)

	var animal models.Animal
	require.NoError(t, tx.Find(&animal, f.animalID))
	require.False(t, animal.OuttakeID.Valid, "animal must not be linked to a rejected outtake")
}

// TestOuttakeCreateAcceptsCurrentLocalTime: the flatpickr widget submits the
// browser's local wall clock ("now"). The buffalo form binder parses custom
// time layouts with time.Parse (UTC), so without server-side localization a
// "now" value in a UTC+NN timezone lands in the future and is wrongly
// rejected. The form value must be re-interpreted in the server local time
// zone before the future-date guard runs (issue #175 TZ quirk).
func TestOuttakeCreateAcceptsCurrentLocalTime(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	client, baseURL := adminClientWithURL(t)

	nowLocal := time.Now().Format(models.DateTimeFormat)
	resp := postOuttakeAt(t, client, baseURL, f, nowLocal, "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode,
		"outtake at local %q (server TZ %s) must be accepted, body: %s",
		nowLocal, time.Now().Format("-07:00"), truncate(body, 800))

	o := &models.Outtake{}
	require.NoError(t, tx.Where("outtaketype_id = ?", f.outtakeTypeID).First(o))
	// The persisted wall clock must match what the user typed, and the stored
	// instant must not be in the future.
	require.Equal(t, nowLocal, o.Date.Format(models.DateTimeFormat), "stored wall clock must equal the submitted local wall clock")
	require.False(t, o.Date.After(models.FormWallClockNow().Add(time.Minute)), "stored outtake date must not be in the future")
}

// TestOuttakeStayDurationPersisted: creating an outtake persists the stay
// duration in whole hours between the animal intake and the outtake date
// (issue #175).
func TestOuttakeStayDurationPersisted(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeNone)
	client, baseURL := adminClientWithURL(t)

	resp := postOuttakeAt(t, client, baseURL, f, "2026/01/15 12:00", "")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(body, 800))

	o := &models.Outtake{}
	require.NoError(t, tx.Where("outtaketype_id = ?", f.outtakeTypeID).First(o))
	require.True(t, o.StayDuration.Valid, "stay duration must be persisted")

	var animal models.Animal
	require.NoError(t, tx.Eager().Find(&animal, f.animalID))
	require.True(t, animal.OuttakeID.Valid)

	intake := &models.Intake{}
	require.NoError(t, tx.Find(intake, animal.IntakeID))
	want := int(o.Date.Sub(intake.Date).Hours())
	if want < 0 {
		want = 0
	}
	require.Equal(t, want, o.StayDuration.Int, "stay duration = whole hours between intake %s and outtake %s", intake.Date, o.Date)
}
