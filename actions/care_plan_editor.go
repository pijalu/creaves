package actions

// Plan editor context (bugs.md U17): the structured schedule/payload
// editors need reference data for their dropdowns (caretypes for
// feeding/care payloads, drug names for medication payloads). Shared by
// the animal Plan-tab modal (animals.go) and the rule editor
// (care_rules.go) so both surfaces offer the same fields.

import (
	"creaves/models"

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
