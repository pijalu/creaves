//go:build !sqlite
// +build !sqlite

package actions

import (
	"encoding/json"
	"testing"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// BUG-44: normalizeFeedingPayload pins a feeding-kind payload to the
// feeding caretype (resolveFeedingCaretype) whatever the client sent;
// other kinds and degenerate inputs pass through untouched.

// ensureFeedingCaretype guarantees a type=1 caretype row exists (the bare
// test DB has none) and returns the resolver's pick.
func ensureFeedingCaretype(t *testing.T) string {
	t.Helper()
	tx := models.DB
	feedID, err := resolveFeedingCaretype(tx)
	require.NoError(t, err)
	if feedID != "" {
		return feedID
	}
	ct := models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "Repas", Type: models.CareTypeFeed}
	require.NoError(t, tx.Create(&ct))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec()
	})
	feedID, err = resolveFeedingCaretype(tx)
	require.NoError(t, err)
	require.NotEmpty(t, feedID)
	return feedID
}

func TestNormalizeFeedingPayloadPinsWrongCaretype(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	feedID := ensureFeedingCaretype(t)

	raw := json.RawMessage(`{"caretype_id":"00000000-0000-0000-0000-000000000000","food":"Pomme"}`)
	out := normalizeFeedingPayload(tx, careplan.KindFeeding, raw)

	var p map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &p))
	require.Equal(t, feedID, p["caretype_id"], "feeding payload must be pinned to the feeding caretype")
	require.Equal(t, "Pomme", p["food"], "other payload fields preserved")
}

func TestNormalizeFeedingPayloadAlreadyPinned(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	feedID := ensureFeedingCaretype(t)

	raw := json.RawMessage(`{"caretype_id":"` + feedID + `"}`)
	out := normalizeFeedingPayload(tx, careplan.KindFeeding, raw)
	require.JSONEq(t, string(raw), string(out))
}

func TestNormalizeFeedingPayloadMissingCaretypeID(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	feedID := ensureFeedingCaretype(t)

	raw := json.RawMessage(`{"food":"Pomme"}`)
	out := normalizeFeedingPayload(tx, careplan.KindFeeding, raw)

	var p map[string]interface{}
	require.NoError(t, json.Unmarshal(out, &p))
	require.Equal(t, feedID, p["caretype_id"], "a missing caretype_id is filled with the feeding caretype")
}

func TestNormalizeFeedingPayloadOtherKindsUntouched(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	for _, kind := range []string{careplan.KindCare, careplan.KindMedication, careplan.KindObservation, careplan.KindWeighing, careplan.KindCleanup} {
		raw := json.RawMessage(`{"caretype_id":"00000000-0000-0000-0000-000000000000"}`)
		out := normalizeFeedingPayload(tx, kind, raw)
		require.Equal(t, string(raw), string(out), "%s payload must pass through byte-identical", kind)
	}
}

func TestNormalizeFeedingPayloadDegenerateInputs(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	require.Nil(t, normalizeFeedingPayload(tx, careplan.KindFeeding, nil))
	require.Equal(t, "not-json", string(normalizeFeedingPayload(tx, careplan.KindFeeding, json.RawMessage(`not-json`))))
}
