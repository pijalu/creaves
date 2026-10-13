package actions

// Treatment-conversion validation + boot refresh (BUG-52, 2026-10-27
// critical-review round: "issues/bugs will lead to potential
// miscare/death of animals so be extra cautious").
//
// The startup_v2 converter turns FUTURE treatment rows (date >= today at
// conversion time) into medication/care/observation plans — ONCE. Any
// treatment series prescribed AFTER that moment had no plan at all: the
// animal's medication never appeared on the day plan. This file closes
// that hole with two tools:
//
//   - ValidateTreatmentConversion (read-only): mirrors the converter's
//     series grouping and routing (SAME functions — no second source of
//     truth) and checks that every expected series is covered by an
//     ACTIVE converter-authored plan. Run via the
//     `careplan:validate-treatments` grift.
//   - RunCarePlanTreatmentRefreshAtBoot: re-runs the (idempotent,
//     name-guarded) series conversion on EVERY boot, so post-conversion
//     prescriptions are converted automatically. No marker: cheap
//     (future rows only) and the guard makes repeat runs no-ops.
//
// Coverage is judged on ACTIVE plans: a deactivated converter plan does
// not produce occurrences, so it does not count as coverage — but the
// refresh never re-activates it (deactivation is a deliberate human
// decision, e.g. a stopped treatment). Such rows are reported as
// INACTIVE lines without flipping Healthy; a MISSING plan is a hard
// failure.

import (
	"fmt"
	"log"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
)

// TreatmentValidationLine is one per-series or per-plan outcome.
type TreatmentValidationLine struct {
	AnimalID int    `json:"animal_id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
}

// TreatmentValidationReport is the per-series coverage gate for the
// treatment conversion (every future-dated series → an ACTIVE
// converter-authored plan).
type TreatmentValidationReport struct {
	GeneratedAt string                    `json:"generated_at"`
	Series      int                       `json:"series"`   // expected series (future rows, converter grouping)
	Plans       int                       `json:"plans"`    // converter-authored non-feeding plans found
	OK          int                       `json:"ok"`       // series covered by an ACTIVE plan
	Missing     int                       `json:"missing"`  // series with NO plan row at all — hard failure
	Inactive    int                       `json:"inactive"` // series whose plan exists but is deactivated (deliberate stop — reported, not failed)
	Orphans     int                       `json:"orphans"`  // converter plans matching no current series (informational: series age out / get edited)
	Lines       []TreatmentValidationLine `json:"lines,omitempty"`
}

// Healthy reports whether every future treatment series is covered by an
// active converter plan. Orphans and deliberate deactivations do not
// fail the gate; a series with NO plan does.
func (r *TreatmentValidationReport) Healthy() bool { return r.Missing == 0 }

// treatmentWantKey is one coverage unit: (animal, routed plan name) —
// exactly the converter re-run guard's identity.
type treatmentWantKey struct {
	animal int
	name   string
}

// ValidateTreatmentConversion re-derives the expected treatment series
// (converter grouping + routing, shared functions) and checks each one
// against the converter-authored plans in the DB. Read-only.
func ValidateTreatmentConversion(tx *pop.Connection) (*TreatmentValidationReport, error) {
	report := &TreatmentValidationReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	today := time.Now().Truncate(24 * time.Hour)
	var rows []models.Treatment
	if err := tx.Where("date >= ?", today).Order("date").All(&rows); err != nil {
		return nil, err
	}
	known, err := knownDrugNames(tx)
	if err != nil {
		return nil, err
	}
	groups, order := groupTreatmentSeries(rows)
	report.Series = len(order)

	// Expected (animal, name) → kind. The converter's re-run guard
	// dedupes by (animal, name COLLATE utf8mb4_bin, created_by IS NULL)
	// — series sharing a routed name are one coverage unit, exactly as
	// the converter sees them.
	want, labels := expectedTreatmentSeries(tx, known, groups, order)

	// Converter-authored non-feeding plans (any active state — we need
	// both the coverage view and the inactive view).
	var plans []models.CareAnimalPlan
	if err := tx.Where("created_by IS NULL AND action_kind <> ?", careplan.KindFeeding).All(&plans); err != nil {
		return nil, err
	}
	report.Plans = len(plans)

	report.OK, report.Missing, report.Inactive, report.Orphans, report.Lines = classifyTreatmentCoverage(want, plans, labels)
	return report, nil
}

// expectedTreatmentSeries maps the grouped future series through the
// converter's routing into (animal, plan-name) coverage units.
func expectedTreatmentSeries(tx *pop.Connection, known map[string]struct{}, groups map[string]*treatmentSeries, order []string) (map[treatmentWantKey]string, map[int]string) {
	want := map[treatmentWantKey]string{}
	labels := map[int]string{}
	for _, key := range order {
		s := groups[key]
		name, kind, _, _ := treatmentSeriesRouting(tx, known, s.drug, s.dosage, s.remarks)
		want[treatmentWantKey{s.animalID, name}] = kind
		labels[s.animalID] = fmt.Sprintf("animal %d", s.animalID)
	}
	return want, labels
}

// classifyTreatmentCoverage compares the expected coverage units against
// the converter-authored plan rows (any active state): a unit with no row
// is MISSING (hard failure), an inactive row is INACTIVE (deliberate
// stop — reported), an active row is OK. Active converter plans with no
// matching series are ORPHANS (informational).
func classifyTreatmentCoverage(want map[treatmentWantKey]string, plans []models.CareAnimalPlan, labels map[int]string) (ok, missing, inactive, orphans int, lines []TreatmentValidationLine) {
	have := map[treatmentWantKey]bool{}   // plan row exists (any state)
	active := map[treatmentWantKey]bool{} // plan is active (produces occurrences)
	for _, p := range plans {
		k := treatmentWantKey{p.AnimalID, p.Name}
		have[k] = true
		if p.Active {
			active[k] = true
		}
	}

	for k := range want {
		switch {
		case !have[k]:
			missing++
			lines = append(lines, TreatmentValidationLine{k.animal, labels[k.animal], "MISSING",
				fmt.Sprintf("series %q has no converted plan", k.name)})
		case !active[k]:
			inactive++
			lines = append(lines, TreatmentValidationLine{k.animal, labels[k.animal], "INACTIVE",
				fmt.Sprintf("plan %q is deactivated (stopped treatment?)", k.name)})
		default:
			ok++
		}
	}
	// Orphans (informational): plans without a matching current series —
	// series age out (date < today) or were edited after conversion.
	for _, p := range plans {
		_, expected := want[treatmentWantKey{p.AnimalID, p.Name}]
		if p.Active && !expected {
			orphans++
			lines = append(lines, TreatmentValidationLine{p.AnimalID,
				fmt.Sprintf("animal %d", p.AnimalID), "ORPHAN",
				fmt.Sprintf("plan %q matches no current treatment series", p.Name)})
		}
	}
	return ok, missing, inactive, orphans, lines
}

// RunCarePlanTreatmentRefresh re-runs the idempotent treatment-series
// conversion in one transaction and returns the number of plans created.
// Deliberately NOT marker-guarded: it must pick up series prescribed
// after the last boot, and the converter's per-series name guard keeps
// repeat runs at +0.
func RunCarePlanTreatmentRefresh(db *pop.Connection) (int, error) {
	created := 0
	if err := db.Transaction(func(tx *pop.Connection) error {
		rep := &ConversionReport{}
		if err := convertTreatmentSeries(tx, rep); err != nil {
			return err
		}
		created = rep.Treatments.PlansCreated
		return nil
	}); err != nil {
		return 0, err
	}
	return created, nil
}

// RunCarePlanTreatmentRefreshAtBoot refreshes treatment coverage on
// every boot. An error aborts the boot (cmd/app/main.go) so a boot with
// silently-unconverted prescriptions never serves.
func RunCarePlanTreatmentRefreshAtBoot() error {
	created, err := RunCarePlanTreatmentRefresh(models.DB)
	if err != nil {
		return err
	}
	if created > 0 {
		log.Printf("care_plan_treatment_refresh: +%d plan(s) for post-conversion treatment series", created)
	}
	return nil
}
