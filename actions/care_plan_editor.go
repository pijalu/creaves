package actions

// Plan editor context (bugs.md U17): the structured schedule/payload
// editors need reference data for their dropdowns (caretypes for
// feeding/care payloads, drug names for medication payloads). Shared by
// the animal Plan-tab modal (animals.go) and the rule editor
// (care_rules.go) so both surfaces offer the same fields.

import (
	"encoding/json"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// planCaretypeOption is one caretype dropdown row (id + localized display
// name resolved at render time via tname on the base name).
type planCaretypeOption struct {
	ID   string
	Name string
}

// setPlanEditorData loads the reference lists the structured plan editors
// (templates/care_plan/_editor.plush.html) render as selects.
func setPlanEditorData(c buffalo.Context, tx *pop.Connection) error {
	cts := &models.Caretypes{}
	if err := tx.Order("name asc").All(cts); err != nil {
		return err
	}
	opts := make([]planCaretypeOption, 0, len(*cts))
	for _, ct := range *cts {
		opts = append(opts, planCaretypeOption{ID: ct.ID.String(), Name: ct.Name})
	}
	c.Set("planCaretypes", opts)

	// BUG-44: the editor pins a feeding-kind plan to the feeding caretype
	// (same resolver the converter uses); empty when the DB has no type=1 row.
	feedID, err := resolveFeedingCaretype(tx)
	if err != nil {
		return err
	}
	c.Set("planFeedingCaretypeID", feedID)

	drugs := &models.Drugs{}
	if err := tx.Order("name asc").All(drugs); err != nil {
		return err
	}
	names := make([]string, 0, len(*drugs))
	for _, d := range *drugs {
		names = append(names, d.Name)
	}
	c.Set("planDrugs", names)
	return nil
}

// normalizeFeedingPayload (BUG-44) pins a feeding-kind action payload to the
// feeding caretype server-side, so a crafted POST (or a legacy editor
// payload carrying a non-feeding caretype) cannot file feedings under a
// wrong type. Other kinds pass through untouched; a missing feeding
// caretype in the DB leaves the payload as-is (validation reports it).
func normalizeFeedingPayload(tx *pop.Connection, kind string, payload json.RawMessage) json.RawMessage {
	if kind != careplan.KindFeeding || len(payload) == 0 {
		return payload
	}
	feedID, err := resolveFeedingCaretype(tx)
	if err != nil || feedID == "" {
		return payload
	}
	var p map[string]interface{}
	if err := json.Unmarshal(payload, &p); err != nil {
		return payload
	}
	if p["caretype_id"] == feedID {
		return payload
	}
	p["caretype_id"] = feedID
	out, err := json.Marshal(p)
	if err != nil {
		return payload
	}
	return out
}
