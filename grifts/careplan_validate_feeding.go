package grifts

import (
	"fmt"

	"creaves/actions"
	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

// careplan:validate-feeding / careplan:repair-feeding — §8.1 gate over the
// feeding conversion (bugs.md BUG-50, 2026-10-27 user directive: "All
// animals must be validated with original information!").
//
// validate-feeding is READ-ONLY: it compares EVERY in-care animal carrying
// a legacy feeding trace (animals.feeding / feeding_start / feeding_end /
// feeding_period) against the feeding plan the current converter semantics
// produce, and lists converter-owned plans sitting on out-of-scope
// (outtaken / empty-diet) animals. Human-authored plans are never modified
// and count as covered.
//
// repair-feeding runs the marker-guarded startup_v5 pass (also executed at
// boot once): stale converter schedules converge to the legacy columns,
// missing plans are created, zombie plans retired.
var _ = grift.Namespace("careplan", func() {
	grift.Desc("validate-feeding", "Validate every converted feeding plan against the original legacy columns (read-only)")
	grift.Add("validate-feeding", func(c *grift.Context) error {
		var report *actions.FeedingValidationReport
		err := models.DB.Transaction(func(tx *pop.Connection) error {
			var err error
			report, err = actions.ValidateLegacyFeedingConversion(tx, false)
			return err
		})
		if err != nil {
			return err
		}
		printFeedingValidation(report)
		return nil
	})

	grift.Desc("repair-feeding", "Run the startup_v5 feeding repair (marker-guarded; converges converter plans to the legacy columns)")
	grift.Add("repair-feeding", func(c *grift.Context) error {
		report, err := actions.RunCarePlanFeedingRepair(models.DB)
		if err != nil {
			return err
		}
		if report == nil {
			fmt.Println("careplan:repair-feeding — marker startup_v5 present, no-op")
			return nil
		}
		printFeedingValidation(report)
		return nil
	})
})

func printFeedingValidation(report *actions.FeedingValidationReport) {
	for _, l := range report.Lines {
		if l.Status != "OK" {
			fmt.Printf("[%s] animal %d %s: %s\n", l.Status, l.AnimalID, l.Label, l.Detail)
		}
	}
	fmt.Printf("feeding validation — animals considered %d: OK %d, repaired %d, created %d, retired-stale %d, zombies retired %d, missing %d, human-only %d\n",
		report.AnimalsConsidered, report.OK, report.Repaired, report.Created, report.RetiredStale, report.ZombiesRetired, report.Missing, report.HumanOnly)
	if report.Healthy() {
		fmt.Println("VALIDATION: PASS")
	} else {
		fmt.Printf("VALIDATION: FAIL — %d animal(s) with NO feeding plan, %d with a STALE schedule (differs from the legacy columns)\n",
			report.Missing, report.Stale)
	}
}
