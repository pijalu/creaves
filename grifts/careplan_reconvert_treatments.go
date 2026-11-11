package grifts

// careplan:reconvert-treatments — one-shot cadence repair (e2e/bugs.md
// BUG-52): deletes the CONVERTER-authored non-feeding plans (created_by IS
// NULL — the §8.2 rollback identity) and re-runs the treatment-series
// conversion, so plans converted before the every-N-days cadence fix are
// rebuilt with the correct schedule. Human-authored plans (created_by set)
// are NEVER touched, and the treatment refresh re-populates from the same
// series on the next boot, so the operation is self-healing afterwards.
//
// Run: POP_PATH=<cfg> buffalo task careplan:reconvert-treatments

import (
	"fmt"
	"log"

	"creaves/actions"
	"creaves/models"
	"creaves/models/careplan"

	grift "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

var _ = grift.Namespace("careplan", func() {
	grift.Desc("reconvert-treatments", "Delete converter-authored treatment/care/observation plans and reconvert the series (cadence repair, BUG-52)")
	grift.Add("reconvert-treatments", func(c *grift.Context) error {
		var removed int64
		err := models.DB.Transaction(func(tx *pop.Connection) error {
			cnt := []struct {
				C int64 `db:"c"`
			}{}
			if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE created_by IS NULL AND action_kind <> ?", careplan.KindFeeding).All(&cnt); err != nil {
				return err
			}
			if len(cnt) > 0 {
				removed = cnt[0].C
			}
			if err := tx.RawQuery("DELETE FROM care_animal_plans WHERE created_by IS NULL AND action_kind <> ?", careplan.KindFeeding).Exec(); err != nil {
				return err
			}
			rep := &actions.ConversionReport{}
			if err := actions.ReconvertTreatmentSeries(tx, rep); err != nil {
				return err
			}
			fmt.Printf("treatment reconvert — deleted %d converter plan(s), recreated %d\n",
				removed, rep.Treatments.PlansCreated)
			return nil
		})
		if err != nil {
			return err
		}
		log.Printf("care_plan_treatment_reconvert: %d plan(s) deleted and reconverted", removed)
		return nil
	})
})
