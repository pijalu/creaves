package grifts

import (
	"encoding/json"
	"fmt"
	"strings"

	"creaves/actions"
	"creaves/models"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

// careplan:fixnames repairs conversion names written by the EARLIER,
// byte-based truncation: names like "Alimentation — lait réf. 30/50 (1 dose
// de lait + 1 dose d’e (conversion)" were cut mid-word at 60 BYTES (and can
// even hold an invalid UTF-8 tail). The current converter shortens at a word
// boundary in runes (R4-7.24), but its re-run guard matches on the exact
// name, so already-converted rows kept their broken names forever.
//
// This task recomputes every conversion-stamped name from the row's FULL
// payload diet and rewrites the ones that differ — idempotent: run it as
// often as needed, it only UPDATEs actual repairs.
//
// (buffalo task careplan:fixnames)
var _ = grift.Namespace("careplan", func() {
	grift.Desc("fixnames", "Rebuild conversion-stamped names word-safely from the payload diet (repairs the old byte-truncated names)")
	grift.Add("fixnames", func(c *grift.Context) error {
		plansFixed, err := fixAnimalPlanNames(models.DB)
		if err != nil {
			return err
		}
		rulesFixed, err := fixCareRuleNames(models.DB)
		if err != nil {
			return err
		}
		matchersFixed, err := fixMatcherNames(models.DB)
		if err != nil {
			return err
		}
		fmt.Printf("careplan:fixnames — repaired %d animal plan(s), %d care rule(s), %d matcher(s)\n",
			plansFixed, rulesFixed, matchersFixed)
		return nil
	})
})

// payloadFood extracts the "food" key of a feeding action payload.
func payloadFood(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	food, _ := m["food"].(string)
	return food, nil
}

// rebuiltName is the canonical name for a diet, preserving the "(à
// vérifier)" fallback suffix the old row carried.
func rebuiltName(oldName, diet string, matcher bool) string {
	fallback := strings.Contains(oldName, " (à vérifier)")
	if matcher {
		return actions.RebuildConvertedMatcherName(diet)
	}
	return actions.RebuildConvertedFeedingName(diet, fallback)
}

func fixAnimalPlanNames(tx *pop.Connection) (int, error) {
	var plans []models.CareAnimalPlan
	if err := tx.Where("name LIKE ?", "%(conversion)%").All(&plans); err != nil {
		return 0, err
	}
	fixed := 0
	for i := range plans {
		p := &plans[i]
		diet, err := payloadFood(p.ActionPayload)
		if err != nil {
			return fixed, fmt.Errorf("plan %s: %w", p.ID, err)
		}
		if diet == "" {
			continue
		}
		want := rebuiltName(p.Name, diet, false)
		if want == p.Name {
			continue
		}
		fmt.Printf("  plan %s (%d):\n    old: %s\n    new: %s\n", p.ID, p.AnimalID, p.Name, want)
		p.Name = want
		if err := tx.Update(p); err != nil {
			return fixed, err
		}
		fixed++
	}
	return fixed, nil
}

func fixCareRuleNames(tx *pop.Connection) (int, error) {
	var rules []models.CareRule
	if err := tx.Where("name LIKE ?", "%(conversion)%").All(&rules); err != nil {
		return 0, err
	}
	fixed := 0
	for i := range rules {
		r := &rules[i]
		diet, err := payloadFood(r.ActionPayload)
		if err != nil {
			return fixed, fmt.Errorf("rule %s: %w", r.ID, err)
		}
		if diet == "" {
			continue
		}
		want := rebuiltName(r.Name, diet, false)
		if want == r.Name {
			continue
		}
		fmt.Printf("  rule %s:\n    old: %s\n    new: %s\n", r.ID, r.Name, want)
		r.Name = want
		if err := tx.Update(r); err != nil {
			return fixed, err
		}
		fixed++
	}
	return fixed, nil
}

// fixMatcherNames rebuilds the matcher names from the diet of their linked
// care rule (a matcher row stores no diet of its own).
func fixMatcherNames(tx *pop.Connection) (int, error) {
	var matchers []models.CareMatcher
	if err := tx.Where("name LIKE ?", "%(conversion)%").All(&matchers); err != nil {
		return 0, err
	}
	fixed := 0
	for i := range matchers {
		m := &matchers[i]
		var rules []models.CareRule
		if err := tx.Where("matcher_id = ?", m.ID).All(&rules); err != nil {
			return fixed, err
		}
		diet := ""
		for j := range rules {
			d, err := payloadFood(rules[j].ActionPayload)
			if err != nil {
				return fixed, fmt.Errorf("matcher %s: %w", m.ID, err)
			}
			if d != "" {
				diet = d
				break
			}
		}
		if diet == "" {
			fmt.Printf("  matcher %s: no linked rule with a diet — skipped\n", m.ID)
			continue
		}
		want := rebuiltName(m.Name, diet, true)
		if want == m.Name {
			continue
		}
		fmt.Printf("  matcher %s:\n    old: %s\n    new: %s\n", m.ID, m.Name, want)
		m.Name = want
		if err := tx.Update(m); err != nil {
			return fixed, err
		}
		fixed++
	}
	return fixed, nil
}
