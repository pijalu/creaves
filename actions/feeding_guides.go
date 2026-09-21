package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// stageOption is a canonical stage plus its localized display label.
type stageOption struct {
	Value string
	Label string
}

// feedingGuideView is the template-facing shape of a feeding guide with its
// stage label resolved in the current UI language.
type feedingGuideView struct {
	ID          int
	SpeciesName string
	Stage       string
	StageLabel  string
	Text        string
}

// FeedingGuidesIndex renders the single-page admin CRUD: the list of guides
// grouped by species (ordered), plus one form used for both create and edit
// (?edit=<id> prefills it).
func FeedingGuidesIndex(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	guides := &models.FeedingGuides{}
	if err := tx.Order("species_name asc, id asc").All(guides); err != nil {
		return err
	}
	c.Set("feedingGuides", feedingGuideViews(c, *guides))

	// Edit prefill: ?edit=<id> loads the record into the form.
	edit := &models.FeedingGuide{}
	if raw := c.Param("edit"); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			return c.Error(http.StatusNotFound, err)
		}
		if err := tx.Find(edit, id); err != nil {
			return c.Error(http.StatusNotFound, err)
		}
	}
	c.Set("feedingGuide", edit)

	c.Set("feedingStageOptions", feedingStageOptions(c))

	return c.Render(http.StatusOK, r.HTML("/feeding_guides/index.plush.html"))
}

// FeedingGuidesCreate creates a guide, or updates the one whose id is posted.
// Duplicate (species, stage) pairs are rejected with a friendly error.
func FeedingGuidesCreate(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	guide := &models.FeedingGuide{}
	if err := c.Bind(guide); err != nil {
		return err
	}

	// Uniqueness of (species_name, stage) — checked explicitly so the user
	// gets a readable flash message instead of a raw MySQL duplicate-key error.
	dup := &models.FeedingGuide{}
	q := tx.Where("species_name = ? AND stage = ?", guide.SpeciesName, guide.Stage)
	if guide.ID != 0 {
		q = q.Where("id <> ?", guide.ID)
	}
	if err := q.First(dup); err == nil {
		c.Flash().Add("danger", T.Translate(c, "feeding_guide.duplicate"))
		return c.Redirect(http.StatusFound, "/feeding_guides")
	}

	if guide.ID != 0 {
		// Update: the id rides along with the bound fields.
		existing := &models.FeedingGuide{}
		if err := tx.Find(existing, guide.ID); err != nil {
			return c.Error(http.StatusNotFound, err)
		}
		existing.SpeciesName = guide.SpeciesName
		existing.Stage = guide.Stage
		existing.Text = guide.Text
		if verrs, err := tx.ValidateAndUpdate(existing); err != nil || verrs.HasAny() {
			c.Flash().Add("danger", verrs.String())
			return c.Redirect(http.StatusFound, "/feeding_guides")
		}
		c.Flash().Add("success", T.Translate(c, "feeding_guide.updated.success"))
	} else {
		if verrs, err := tx.ValidateAndCreate(guide); err != nil || verrs.HasAny() {
			c.Flash().Add("danger", verrs.String())
			return c.Redirect(http.StatusFound, "/feeding_guides")
		}
		c.Flash().Add("success", T.Translate(c, "feeding_guide.created.success"))
	}

	return c.Redirect(http.StatusFound, "/feeding_guides")
}

// FeedingGuidesDestroy removes a guide. Admin only.
func FeedingGuidesDestroy(c buffalo.Context) error {
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	guide := &models.FeedingGuide{}
	if err := tx.Find(guide, c.Param("feeding_guide_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := tx.Destroy(guide); err != nil {
		return err
	}
	c.Flash().Add("success", T.Translate(c, "feeding_guide.destroyed.success"))

	return c.Redirect(http.StatusFound, "/feeding_guides")
}

// feedingGuideViews maps guides to template views with localized stage labels.
func feedingGuideViews(c buffalo.Context, guides models.FeedingGuides) []feedingGuideView {
	options := feedingStageOptions(c)
	stageIndex := map[string]int{}
	for i, o := range options {
		stageIndex[o.Value] = i
	}
	out := make([]feedingGuideView, 0, len(guides))
	for _, g := range guides {
		label := g.Stage
		if o := stageIndex[g.Stage]; o < len(options) {
			label = options[o].Label
		}
		out = append(out, feedingGuideView{
			ID:          g.ID,
			SpeciesName: g.SpeciesName,
			Stage:       g.Stage,
			StageLabel:  label,
			Text:        g.Text,
		})
	}
	// Keep the SQL species ordering; sort stages canonically within a species.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SpeciesName != out[j].SpeciesName {
			return false
		}
		return stageIndex[out[i].Stage] < stageIndex[out[j].Stage]
	})
	return out
}

// feedingStageOptions resolves canonical stages to the current UI language,
// in canonical display order.
func feedingStageOptions(c buffalo.Context) []stageOption {
	out := make([]stageOption, 0, len(models.FeedingStages()))
	for _, s := range models.FeedingStages() {
		out = append(out, stageOption{Value: s, Label: T.Translate(c, models.FeedingStageI18NKeys[s])})
	}
	return out
}
