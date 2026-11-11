package actions

import (
	"encoding/json"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// §4.4/§7.1-6 fail-closed regression (review finding #1): a rule whose
// matcher expression is broken — or whose matcher row is missing — must
// match NOTHING, in the bulk membership path AND the single-item reverify
// path. Only a rule WITHOUT a matcher matches every animal.
func TestBrokenMatcherMatchesNothing(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()

	slot := func(min int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day(), 12, min, 0, 0, time.Local)
	}
	mkRule := func(t2 *testing.T, min int, matcher *uuid.NullUUID) *models.CareRule {
		t2.Helper()
		sched, err := json.Marshal(map[string]interface{}{
			"times":       []string{slot(min).Format("15:04")},
			"anchor":      "fixed",
			"anchor_date": slot(min).Format("2006-01-02"),
		})
		require.NoError(t2, err)
		payload, err := json.Marshal(map[string]interface{}{
			"caretype_id": f.feedCare.String(),
			"food":        "grenouilles",
		})
		require.NoError(t2, err)
		r := &models.CareRule{
			ID:            uuid.Must(uuid.NewV4()),
			Name:          "R-fc-" + f.marker + "-" + slot(min).Format("1504"),
			ActionKind:    "feeding",
			ActionPayload: payload,
			Schedule:      sched,
			Active:        true,
		}
		if matcher != nil {
			r.MatcherID = *matcher
		}
		require.NoError(t2, tx.Create(r))
		return r
	}

	// control: rule WITHOUT matcher → matches every in-care animal
	open := mkRule(t, 0, nil)

	// broken expression (unknown field fails §5.2 parse validation).
	// (A dangling matcher_id is impossible: care_rules_ibfk_1 is FK
	// RESTRICT — the Destroy handler also refuses in-use matchers.)
	brokenM := &models.CareMatcher{
		ID:         uuid.Must(uuid.NewV4()),
		Name:       "MBrk-" + f.marker,
		Expression: `nosuchfield = "x"`,
	}
	require.NoError(t, tx.Create(brokenM))
	broken := mkRule(t, 1, &uuid.NullUUID{UUID: brokenM.ID, Valid: true})

	plan, err := BuildDayPlan(tx, now, time.Time{}, time.Time{})
	require.NoError(t, err)
	found := map[string]int{}
	for i := range plan.Items {
		if s := plan.Items[i].Occurrence.Source; s != nil && s.SourceType() == models.ApplicationSourceRule {
			found[s.SourceID()]++
		}
	}
	require.Positive(t, found[open.ID.String()], "control: no-matcher rule must match")
	require.Zero(t, found[broken.ID.String()], "broken matcher must match nothing (§4.4)")

	// reverify path (ruleCoversAnimal) must agree with the bulk path
	revRef := func(r *models.CareRule, min int) planItemRef {
		return planItemRef{
			SourceType: models.ApplicationSourceRule,
			SourceID:   r.ID.String(),
			AnimalID:   f.animalIDs[0],
			DueAt:      slot(min),
		}
	}
	_, err = ReverifyItem(tx, revRef(open, 0), now)
	require.NoError(t, err, "control: no-matcher rule reproduces")
	_, err = ReverifyItem(tx, revRef(broken, 1), now)
	require.ErrorIs(t, err, errItemNotReproduced, "broken matcher must reproduce nothing")
}
