package actions

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/stretchr/testify/require"
)

// newSpeciesTestApp registers species routes with the shared test DB.
func newSpeciesTestApp(t *testing.T, u *models.User) *buffalo.App {
	t.Helper()
	tx := searchTestDB(t)
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			c.Set("tx", tx)
			if u != nil {
				c.Set("current_user", u)
			}
			return next(c)
		}
	})
	res := SpeciesResource{}
	a.POST("/species/", res.Create)
	a.PUT("/species/{species_id}", res.Update)
	a.GET("/species/{species_id}", res.Show)
	return a
}

func speciesTestFixture(t *testing.T) {
	t.Helper()
	tx := searchTestDB(t)
	require.NoError(t, tx.RawQuery("INSERT INTO native_statuses (id, status, indication, freeable, created_at, updated_at) VALUES ('NS_SP_A', 'NS SPA', 'x', 0, NOW(), NOW()) ON DUPLICATE KEY UPDATE status = VALUES(status)").Exec())
	require.NoError(t, tx.RawQuery("INSERT INTO native_statuses (id, status, indication, freeable, created_at, updated_at) VALUES ('NS_SP_B', 'NS SPB', 'x', 0, NOW(), NOW()) ON DUPLICATE KEY UPDATE status = VALUES(status)").Exec())
	require.NoError(t, tx.RawQuery("DELETE FROM species WHERE id IN ('SP_T1','SP_T2')").Exec())
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE id IN ('SP_T1','SP_T2')").Exec()
		tx.RawQuery("DELETE FROM native_statuses WHERE id IN ('NS_SP_A','NS_SP_B')").Exec()
	})
}

// TestSpeciesCreateBindsAllFields pins that every form field — including the
// previously broken AgwGroup/SubsideGroup names, the new Huntable checkbox and
// the NativeStatus select — binds and persists.
func TestSpeciesCreateBindsAllFields(t *testing.T) {
	speciesTestFixture(t)
	maintainer := &models.User{Login: "sp_maintainer", Admin: true, Maintainer: true, Approved: true}
	a := newSpeciesTestApp(t, maintainer)

	form := url.Values{
		"ID":             {"SP_T1"},
		"Species":        {"Species T1"},
		"CreavesSpecies": {"Creaves T1"},
		"Class":          {"ClassT"},
		"Order":          {"OrderT"},
		"Family":         {"FamilyT"},
		"AgwGroup":       {"AgwT"},
		"SubsideGroup":   {"SubT"},
		"NativeStatus":   {"NS_SP_A"},
		"Game":           {"true"},
		"Huntable":       {"true"},
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, newJSONRequest(http.MethodPost, "/species/", form.Encode()))
	require.Equal(t, http.StatusCreated, w.Code, "create failed: %s", w.Body.String())

	sp := &models.Species{}
	require.NoError(t, searchTestDB(t).Find(sp, "SP_T1"))
	require.Equal(t, "AgwT", sp.AgwGroup)
	require.Equal(t, "SubT", sp.SubsideGroup)
	require.Equal(t, "NS_SP_A", sp.NativeStatus)
	require.True(t, sp.Game)
	require.True(t, sp.Huntable)
}

// TestSpeciesUpdateResetsUncheckedFlags pins the checkbox gotcha: unchecked
// Game/Huntable must persist as false (the handler resets flags before bind).
func TestSpeciesUpdateResetsUncheckedFlags(t *testing.T) {
	speciesTestFixture(t)
	tx := searchTestDB(t)
	require.NoError(t, tx.RawQuery("INSERT INTO species (id, species, creaves_species, class, `order`, family, native_status, agw_group, subside_group, game, huntable, created_at, updated_at) VALUES ('SP_T2', 'Species T2', 'Creaves T2', 'C', 'O', 'F', 'NS_SP_A', 'G1', 'S1', 1, 1, NOW(), NOW())").Exec())

	maintainer := &models.User{Login: "sp_maintainer2", Admin: true, Maintainer: true, Approved: true}
	a := newSpeciesTestApp(t, maintainer)

	// Submit without Game/Huntable (unchecked) and with new values.
	form := url.Values{
		"ID":             {"SP_T2"},
		"Species":        {"Species T2"},
		"CreavesSpecies": {"Creaves T2"},
		"Class":          {"C"},
		"Order":          {"O"},
		"Family":         {"F"},
		"AgwGroup":       {"G2"},
		"SubsideGroup":   {"S2"},
		"NativeStatus":   {"NS_SP_B"},
	}
	w := httptest.NewRecorder()
	a.ServeHTTP(w, newJSONRequest(http.MethodPut, "/species/SP_T2", form.Encode()))
	require.Equal(t, http.StatusOK, w.Code, "update failed: %s", w.Body.String())

	sp := &models.Species{}
	require.NoError(t, tx.Find(sp, "SP_T2"))
	require.False(t, sp.Game, "unchecked Game must persist false")
	require.False(t, sp.Huntable, "unchecked Huntable must persist false")
	require.Equal(t, "NS_SP_B", sp.NativeStatus)
	require.Equal(t, "G2", sp.AgwGroup)
	require.Equal(t, "S2", sp.SubsideGroup)
}

// TestNativeStatusesToSelectables verifies ID values and label fallback.
func TestNativeStatusesToSelectables(t *testing.T) {
	speciesTestFixture(t)
	tx := searchTestDB(t)
	ns := &models.NativeStatuses{}
	require.NoError(t, tx.Where("id IN ('NS_SP_A','NS_SP_B')").Order("status asc").All(ns))

	opts := nativeStatusesToSelectables(ns, "en-US", tx)
	require.Len(t, opts, 2)
	require.Equal(t, "NS_SP_A", opts[0].SelectValue())
	require.Equal(t, "NS SPA", opts[0].SelectLabel())
	require.Equal(t, "NS_SP_B", opts[1].SelectValue())
}
