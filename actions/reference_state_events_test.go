package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refStateDB returns the database the application itself uses (models.DB is
// what popmw.Transaction hands to handlers, captured once via appOnce). Tests
// must fixture against THAT database so handlers and assertions agree
// regardless of GO_ENV. Skips when no database is reachable.
func refStateDB(t *testing.T) *pop.Connection {
	t.Helper()
	if models.DB == nil {
		t.Skip("models.DB is nil — no database configured")
	}
	var one int
	if err := models.DB.RawQuery("SELECT 1").First(&one); err != nil {
		t.Skipf("application test database unavailable: %v", err)
	}
	return models.DB
}

// refStateConfig activates an event-stream-enabled config for the duration of
// the test (webhook delivery stays disabled; only event_streams rows matter).
func refStateConfig(t *testing.T) *models.Config {
	t.Helper()
	cfg := seedConfig(t, "refstate", true)
	saved := CurrentConfig
	CurrentConfig = cfg
	t.Cleanup(func() {
		CurrentConfig = saved
		models.DB.RawQuery("DELETE FROM event_streams WHERE instance_id = ?", cfg.InstanceID).Exec()
		models.DB.RawQuery("DELETE FROM config WHERE id = ?", cfg.ID).Exec()
	})
	return cfg
}

// refStateEventCount counts animal_state events for one animal within the
// test config's instance.
func refStateEventCount(t *testing.T, cfg *models.Config, animalID int) int {
	t.Helper()
	n, err := models.DB.Where("instance_id = ? AND animal_id = ? AND event_type = ?",
		cfg.InstanceID, animalID, string(models.EventTypeAnimalState)).Count(&models.EventStream{})
	require.NoError(t, err)
	return n
}

// refStateLatestPayload decodes the newest animal_state payload for one animal.
func refStateLatestPayload(t *testing.T, cfg *models.Config, animalID int) models.EventPayload {
	t.Helper()
	ev := &models.EventStream{}
	require.NoError(t, models.DB.Where("instance_id = ? AND animal_id = ? AND event_type = ?",
		cfg.InstanceID, animalID, string(models.EventTypeAnimalState)).
		Order("created_at desc, id desc").First(ev))
	payload, err := ev.GetPayload()
	require.NoError(t, err)
	return payload
}

// refStateRemap issues an admin remap POST and asserts HTTP 200.
func refStateRemap(t *testing.T, path, replacementID string) {
	t.Helper()
	refStatePost(t, http.StatusOK, path, replacementID)
}

// refStateDelete issues an admin reference-delete POST and asserts the 303
// See Other redirect the flow returns.
func refStateDelete(t *testing.T, path, replacementID string) {
	t.Helper()
	refStatePost(t, http.StatusSeeOther, path, replacementID)
}

// refStatePost issues an admin POST with a replacement_id JSON body and
// asserts the expected status. JSON content type bypasses the CSRF form
// check so the helper works with or without GO_ENV=test.
func refStatePost(t *testing.T, wantStatus int, path, replacementID string) {
	t.Helper()
	client := adminClient(t)
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	body := fmt.Sprintf(`{"replacement_id": %q}`, replacementID)
	resp, err := client.Post(srv.URL+path, "application/json",
		strings.NewReader(body))
	require.NoError(t, err)
	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, wantStatus, resp.StatusCode,
		"POST %s failed: %d %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
}

// TestAnimalagesRemapRepublishesAnimalState proves the generic reference
// remap re-emits animal_state events for exactly the animals that referenced
// the removed age, with the replacement age in the payload.
func TestAnimalagesRemapRepublishesAnimalState(t *testing.T) {
	requireMySQLTestDB(t)
	tx := refStateDB(t)
	cfg := refStateConfig(t)
	f := createAnimalSearchFixtures(t, tx)

	// Baseline: no state events for the fixture animals.
	for _, id := range []int{f.animalA, f.animalB, f.animalC} {
		require.Zero(t, refStateEventCount(t, cfg, id))
	}

	var replacement models.Animalage
	require.NoError(t, tx.Find(&replacement, f.animalage2))

	refStateRemap(t,
		fmt.Sprintf("/animalages/%s/remap", f.animalage1),
		f.animalage2.String())

	// A and C referenced age1 -> both must have a fresh state event.
	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalA), 1)
	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalC), 1)
	// B referenced age2 -> untouched.
	assert.Zero(t, refStateEventCount(t, cfg, f.animalB), "unaffected animal must not get an event")

	payload := refStateLatestPayload(t, cfg, f.animalA)
	assert.Equal(t, replacement.Name, payload.Animal.AnimalAge, "payload must carry the replacement age")

	var count int
	require.NoError(t, tx.RawQuery("SELECT COUNT(*) FROM animalages WHERE id = ?", f.animalage1).First(&count))
	assert.Zero(t, count, "source age must be deleted")
}

// TestAnimaltypesRemapRepublishesAnimalState proves the animaltype remap
// handler re-emits animal_state events for affected animals.
func TestAnimaltypesRemapRepublishesAnimalState(t *testing.T) {
	requireMySQLTestDB(t)
	tx := refStateDB(t)
	cfg := refStateConfig(t)
	f := createAnimalSearchFixtures(t, tx)

	var replacement models.Animaltype
	require.NoError(t, tx.Find(&replacement, f.animaltype2))

	refStateRemap(t,
		fmt.Sprintf("/animaltypes/%s/remap", f.animaltype1),
		f.animaltype2.String())

	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalA), 1)
	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalC), 1)
	assert.Zero(t, refStateEventCount(t, cfg, f.animalB), "unaffected animal must not get an event")

	payload := refStateLatestPayload(t, cfg, f.animalA)
	assert.Equal(t, replacement.Name, payload.Animal.AnimalType, "payload must carry the replacement type")
}

// TestReferenceDeleteCreateRepublishesAnimalState proves the reference
// delete-create flow (with replacement) re-emits animal_state events for the
// affected animals.
func TestReferenceDeleteCreateRepublishesAnimalState(t *testing.T) {
	requireMySQLTestDB(t)
	tx := refStateDB(t)
	cfg := refStateConfig(t)
	f := createAnimalSearchFixtures(t, tx)

	var replacement models.Animalage
	require.NoError(t, tx.Find(&replacement, f.animalage2))

	refStateDelete(t,
		fmt.Sprintf("/animalages/%s/delete", f.animalage1),
		f.animalage2.String())

	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalA), 1)
	assert.GreaterOrEqual(t, refStateEventCount(t, cfg, f.animalC), 1)
	assert.Zero(t, refStateEventCount(t, cfg, f.animalB), "unaffected animal must not get an event")

	payload := refStateLatestPayload(t, cfg, f.animalA)
	assert.Equal(t, replacement.Name, payload.Animal.AnimalAge, "payload must carry the replacement age")
}

// TestReferenceAffectedAnimalIDsSkipsUnlinkedTables proves tables without an
// animal linkage (species, dosages) are skipped instead of failing the
// collection query.
func TestReferenceAffectedAnimalIDsSkipsUnlinkedTables(t *testing.T) {
	tx := searchTestDB(t)

	ids, err := referenceAffectedAnimalIDs(tx, map[string]string{
		"species": "animaltype_id",
		"dosages": "drug_id",
	}, uuid.Must(uuid.NewV4()))
	require.NoError(t, err)
	assert.Empty(t, ids)
}
