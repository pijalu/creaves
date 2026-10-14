package actions

import (
	"strings"

	"creaves/models"
	"creaves/models/careplan"
)

// FeedingPlanSummary is the view-model of ONE active feeding plan for the
// animal show/edit pages (bugs.md BUG-13): the legacy "Feeding from …
// every Nh" banner used the frozen animals.feeding* columns whose
// FeedingStartFmt/FeedingEndFmt fall back to defaults when NULL, rendering
// phantom times that can no longer be edited. The banner is replaced by a
// summary of the ACTUAL protocol feeding (care_animal_plans, kind
// "feeding") — the data the engine really applies.
type FeedingPlanSummary struct {
	// Diet is the payload "food" (already normalized at write time).
	Diet string
	// TimesLabel is the pre-joined daily slots ("07:00, 13:30, 19:00").
	TimesLabel string
	// NeedsReview is the plan's MANUAL needs_confirmation flag (BUG-52e
	// user ruling): the user asked for review via the UI; an admin
	// resolves it on the "Protocols to confirm" screen.
	NeedsReview bool
}

// feedingPlanSummaries builds the summary list of the animal's ACTIVE
// feeding plans (kind feeding). Parse failures skip the plan rather than
// failing the page — a malformed plan must not break the animal page.
func feedingPlanSummaries(plans *models.CareAnimalPlans) []FeedingPlanSummary {
	if plans == nil {
		return nil
	}
	var out []FeedingPlanSummary
	for _, p := range *plans {
		if p.ActionKind != careplan.KindFeeding || !p.Active {
			continue
		}
		s := FeedingPlanSummary{
			Diet:        payloadFoodOf(p.ActionPayload),
			NeedsReview: p.NeedsConfirmation,
		}
		if sched, err := careplan.ParseScheduleJSON(p.Schedule); err == nil {
			times := make([]string, 0, len(sched.Times))
			for _, t := range sched.Times {
				times = append(times, t.String())
			}
			s.TimesLabel = strings.Join(times, ", ")
		}
		// A feeding plan without any diet text carries no useful
		// information for the summary — skip it.
		if strings.TrimSpace(s.Diet) == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}
