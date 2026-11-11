package actions

import (
	"fmt"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// TestAnimalShowCareTabHasNoSpeciesDietFrame (#199-18, #208): the animal
// sheet read mode care tab must keep only the animal's own alimentation field
// and feeding times. The feeding-guide system (#145) has been removed
// entirely (#208.1); this test pins that no species diet-by-stage frame is
// rendered in the read mode.
func TestAnimalShowCareTabHasNoSpeciesDietFrame(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	showHTML := fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d", f.freeID))

	// Alimentation is GONE from the read-mode care tab (legacy feeding
	// columns frozen; feeding is fully managed by protocol plans — the
	// feeding-plan summary banner replaced it, BUG-13).
	require.NotContains(t, showHTML, `>Feeding</label>`,
		"read mode must not render the legacy frozen alimentation field")
	// …and no species diet frame, in any locale wording.
	require.NotContains(t, showHTML, "Diet by life stage")
	require.NotContains(t, showHTML, "Régime alimentaire par stade")
	require.NotContains(t, showHTML, "Fütterung nach Lebensstadium")
	require.NotContains(t, showHTML, "Voeding per levensfase")
	// The feeding-guide modal/JS must be gone from the edit form too (#208.1).
	editHTML := fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d/edit", f.freeID))
	require.NotContains(t, editHTML, "feeding-guide-btn")
	require.NotContains(t, editHTML, "feedingGuideModal")
	require.NotContains(t, editHTML, "/suggestions/feeding_guide")
}
