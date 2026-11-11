package grifts

// careplan:validate-treatments — read-only coverage gate for the
// treatment conversion (bugs.md BUG-52): every future-dated treatment
// series must be covered by an ACTIVE converter-authored plan. Run
// against the default DB (POP_PATH override selects a fixture clone).

import (
	"fmt"

	"creaves/actions"
	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
)

var _ = grift.Namespace("careplan", func() {
	grift.Desc("validate-treatments", "Check every future treatment series has an active converted plan (read-only)")
	grift.Add("validate-treatments", func(c *grift.Context) error {
		report, err := actions.ValidateTreatmentConversion(models.DB)
		if err != nil {
			return err
		}
		for _, l := range report.Lines {
			fmt.Printf("[%s] %s: %s\n", l.Status, l.Label, l.Detail)
		}
		fmt.Printf("treatment validation — series %d, plans %d: OK %d, missing %d, inactive %d, orphans %d\n",
			report.Series, report.Plans, report.OK, report.Missing, report.Inactive, report.Orphans)
		if report.Healthy() {
			fmt.Println("VALIDATION: PASS")
		} else {
			fmt.Printf("VALIDATION: FAIL — %d treatment series with NO converted plan (miscare risk: the medication never shows on the day plan)\n",
				report.Missing)
		}
		return nil
	})
})
