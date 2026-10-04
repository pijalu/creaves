package actions

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

// Rev review: the Observation tab rendered rows whose label block
// (.plan-med-label) came out EMPTY while the same card's ℹ data attributes
// were filled — reproduce at the plush level.
func TestPlanRowLineLabelRenders(t *testing.T) {
	raw := readTemplate(t, "../templates/care_plan/_plan_row_line.plush.html")
	card := CardView{
		SourceType:   "animal",
		SourceID:     "59da89ac",
		SourceName:   "Traitement — Sexage",
		Detail:       "Sexage",
		ActionKind:   "observation",
		AnimalID:     9866,
		AnimalLabel:  "1557/26 · Hérisson · R4",
		AnimalLink:   "/animals/9866",
		SourceLink:   "/animals/9866",
		DueAt:        time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local),
		DueHM:        "12:00",
		Status:       "missing",
		Tier:         0,
		Applicable:   true,
		RemainingCap: "0",
	}
	ctx := plush.NewContextWithContext(context.Background())
	ctx.Set("card", card)
	ctx.Set("t", func(s string, h plush.HelperContext) (string, error) { return s, nil })
	ctx.Set("dueLabel", func(hm, dayKey, shortDate string, h plush.HelperContext) (string, error) {
		return hm, nil
	})
	out, err := plush.Render(raw, ctx)
	require.NoError(t, err)
	require.Contains(t, out, "1557/26 · Hérisson · R4", "label block must render the animal label")
	require.Contains(t, out, "Sexage", "label block must render the detail")
	// The label div itself must not be empty.
	const marker = "plan-med-label"
	at := strings.Index(out, marker)
	require.GreaterOrEqual(t, at, 0)
	div := out[at:]
	end := strings.Index(div, "</div>")
	require.Greater(t, strings.TrimSpace(div[len(marker)+1:end]), "",
		"plan-med-label div must not be empty; got: %q", div[:end+6])
}
