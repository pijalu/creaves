package actions

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// BUG-23 (2026-10-28): deleting a care must retrigger the schedule — the
// cached landing badge counts (open/late) are invalidated post-commit
// exactly like on the apply/unapply paths, so the very next landing render
// reflects the removal without waiting out the TTL.
func TestBug23CareDestroyInvalidatesDayPlanBadgeCache(t *testing.T) {
	f := setupPlanFixture(t)
	admin, baseURL := planAdminClient(t)
	token := planToken(t, admin, baseURL)

	// An applied feeding rule → a fulfillment care row to delete.
	rule := f.feedRule(t, models.DB, itemDueSoon(time.Now()))
	_, body := planGetJSON(t, admin, baseURL, "/care_plan")
	it := findItemFrom(planItemsOf(t, body), f.animalIDs[0], "feeding", rule.ID)
	require.NotNil(t, it)
	code, raw := planDoJSON(t, admin, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ?", rule.ID).First(app))

	// Prime the badge cache (the apply path already invalidated once).
	CountOpenItemsCached(models.DB, time.Now())
	dayPlanBadgeCache.Lock()
	primed := dayPlanBadgeCache.valid
	dayPlanBadgeCache.Unlock()
	require.True(t, primed, "the badge cache is primed before the delete")

	// Delete the care (admin DELETE /cares/{id}).
	del := url.Values{}
	del.Set("_method", "DELETE")
	del.Set("authenticity_token", token)
	resp, err := admin.PostForm(baseURL+"/cares/"+app.FulfillmentID, del)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// The destroy path dropped the cached counts post-commit — the next
	// landing render recomputes instead of serving the stale badge.
	dayPlanBadgeCache.Lock()
	valid := dayPlanBadgeCache.valid
	dayPlanBadgeCache.Unlock()
	require.False(t, valid,
		"cares Destroy must invalidate the landing badge cache (BUG-23)")

	// NOTE: the occurrence itself stays APPLIED (the application row is
	// marked fulfillment_deleted, §10-CP1) — deleting the record does not
	// re-open the occurrence. The weight side is pinned below.
}

// BUG-23 companion: the weight-loss cache is dropped when a WEIGHTED care
// is removed, so a weight-derived dosage recomputes from the previous
// weight on the next plan build (dosage resolution itself queries the
// cares table per request — no stale-weight cache exists).
func TestBug23WeightedCareDestroyInvalidatesWeightLossCache(t *testing.T) {
	f := setupPlanFixture(t)
	admin, baseURL := planAdminClient(t)
	token := planToken(t, admin, baseURL)

	// A weighing care row carrying a weight on a fixture animal.
	ct := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "Bug23W-" + f.marker}
	require.NoError(t, models.DB.Create(ct))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec()
	})
	weighted := &models.Care{
		ID:       uuid.Must(uuid.NewV4()),
		Date:     time.Now(),
		AnimalID: f.animalIDs[0],
		TypeID:   ct.ID,
		Weight:   nulls.NewString("850"),
	}
	require.NoError(t, models.DB.Create(weighted))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM cares WHERE id = ?", weighted.ID).Exec()
	})

	// Prime the weight-loss cache (package-internal state — the refresh
	// goroutine stub cannot populate it from a test).
	cacheMutex.Lock()
	weightLossCache = &[]AnimalWithWeight{}
	cacheLastUpdate = time.Now()
	cacheMutex.Unlock()
	require.False(t, WeightLossCachedAt().IsZero(), "weight-loss cache primed")

	del := url.Values{}
	del.Set("_method", "DELETE")
	del.Set("authenticity_token", token)
	resp, err := admin.PostForm(baseURL+"/cares/"+weighted.ID.String(), del)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	require.True(t, WeightLossCachedAt().IsZero(),
		"deleting a weighted care must invalidate the weight-loss cache (BUG-23)")
}
