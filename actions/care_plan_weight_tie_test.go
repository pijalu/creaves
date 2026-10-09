package actions

import (
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// TestFillLastWeightsSameDateTie (bugs.md BUG-16): two cares with the SAME
// date must resolve deterministically to the latest INSERTED row — the
// previous MAX(date)-only join returned both rows and the loop kept the
// winner of InnoDB's pick order, feeding a random weight into the §10-B6
// dosage computation.
func TestFillLastWeightsSameDateTie(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	animalID := f.animalIDs[0]

	stamp := time.Now().Add(-time.Hour).Truncate(time.Second)
	older := &models.Care{AnimalID: animalID, TypeID: f.defCare, Date: stamp, Weight: nulls.NewString("100")}
	require.NoError(t, tx.Create(older))
	// Same date, recorded later (the backdated-correction case: datetime is
	// second-precision, so give it a visibly later created_at) → wins.
	newer := &models.Care{AnimalID: animalID, TypeID: f.defCare, Date: stamp, Weight: nulls.NewString("250")}
	require.NoError(t, tx.Create(newer))
	// NOTE: pop's Create stores created_at as LOCAL wall clock (string),
	// while time.Time RawQuery args are converted to UTC by go-sql-driver
	// (DSN loc defaults to UTC) — pass a pre-formatted local string so the
	// stamp lands in the same representation pop uses.
	require.NoError(t, tx.RawQuery("UPDATE cares SET created_at = ? WHERE id = ?",
		time.Now().Add(time.Minute).Format("2006-01-02 15:04:05"), newer.ID).Exec())
	// Distinct later date → wins over both regardless of insertion order.
	latest := &models.Care{AnimalID: animalID, TypeID: f.defCare, Date: stamp.Add(time.Minute), Weight: nulls.NewString("300")}
	require.NoError(t, tx.Create(latest))

	pa, err := loadAnimalContexts(tx, time.Now())
	require.NoError(t, err)
	ctx := pa.ctxs[animalID]
	require.NotNil(t, ctx)
	require.NotNil(t, ctx.LastWeightG, "last weight must resolve")
	require.Equal(t, 300.0, *ctx.LastWeightG)

	// Remove the distinct-date row: the same-date tie must resolve to the
	// row inserted LAST (created_at desc), repeatably.
	require.NoError(t, tx.Destroy(latest))
	for i := 0; i < 5; i++ {
		pa, err = loadAnimalContexts(tx, time.Now())
		require.NoError(t, err)
		ctx = pa.ctxs[animalID]
		require.NotNil(t, ctx.LastWeightG)
		require.Equal(t, 250.0, *ctx.LastWeightG, "run %d: same-date tie must pick the latest inserted row", i)
	}
}
