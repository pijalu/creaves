package grifts

import (
	"encoding/json"
	"fmt"

	"creaves/actions"
	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
)

// careplan:migrate-cage-rules runs the startup_v3 one-shot migration
// (bugs.md t10, 2026-10-27: "cage names are per centers — eliminate the
// cage-name matchers; in doubt, migrate to the animal") outside the boot
// path: converter-owned feeding rules retire in favor of per-animal
// plans, unreferenced matchers are deleted. Marker-guarded exactly like
// the boot run — a finished migration is a no-op; delete the
// care_plan_conversion row key=startup_v3 to force a (write-neutral)
// re-run.
var _ = grift.Namespace("careplan", func() {
	grift.Desc("migrate-cage-rules", "Run the startup_v3 cage-matcher→per-animal migration (marker-guarded, idempotent)")
	grift.Add("migrate-cage-rules", func(c *grift.Context) error {
		report, err := actions.RunCarePlanMigration(models.DB)
		if err != nil {
			return err
		}
		if report == nil {
			fmt.Println("careplan:migrate-cage-rules — marker startup_v3 present, no-op")
			return nil
		}
		m := report.Migration
		fmt.Printf("careplan:migrate-cage-rules — rules considered %d, retired %d (hand-edited kept %d), plans +%d, over-sweep dropped %d, matchers deleted %d, kept %d\n",
			m.RulesConsidered, m.RulesRetired, m.RulesHandEditedSkipped,
			m.PlansCreated, m.OverSweepDropped, m.MatchersDeleted, m.MatchersKept)
		for _, line := range m.Lines {
			animal := "—"
			if line.AnimalID != 0 {
				animal = fmt.Sprintf("%d", line.AnimalID)
			}
			fmt.Printf("  [%s] %s: %s\n", animal, line.Label, line.Reason)
		}
		raw, _ := json.MarshalIndent(m, "", "  ")
		fmt.Printf("%s\n", raw)
		return nil
	})
})
