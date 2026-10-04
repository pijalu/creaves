package grifts

import (
	"fmt"

	"creaves/actions"
	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
)

// careplan:seed re-runs the idempotent seed-library step on a database
// whose startup converter already finished (marker present). R8-3: lets
// existing installs pick up the SR13 cleanup rule + SM14 zone matcher
// without a re-conversion.
var _ = grift.Namespace("careplan", func() {
	grift.Desc("seed", "Re-run the idempotent seed library (new matchers/rules land on post-cutover DBs)")
	grift.Add("seed", func(c *grift.Context) error {
		report, err := actions.SeedLibraryOnce(models.DB)
		if err != nil {
			return err
		}
		fmt.Printf("careplan:seed — matchers +%d, rules +%d\n",
			report.Seeds.MatchersInserted, report.Seeds.RulesInserted)
		for _, skip := range report.Seeds.Skipped {
			fmt.Printf("  skipped: %s\n", skip)
		}
		return nil
	})
})
