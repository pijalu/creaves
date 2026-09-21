package actions

import (
	"fmt"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// TestAnimalShowCareTabHasNoSpeciesDietFrame (#199-18): the animal sheet read
// mode care tab must keep only the animal's own alimentation field and feeding
// times — the species diet-by-stage frame (added for #145) must NOT be
// rendered, even when feeding guides exist for the animal's species.
// Edit mode is untouched (the form never displayed the frame).
func TestAnimalShowCareTabHasNoSpeciesDietFrame(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	// Feeding guides exist for the animal's species — the frame would show
	// if the read mode still listed them.
	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	g := &models.FeedingGuide{
		SpeciesName: a.Species,
		Stage:       models.FeedingStageAdult,
		Text:        "Diet frame must not render #199-18",
	}
	require.NoError(t, tx.Create(g))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM feeding_guides WHERE species_name = ?", a.Species).Exec()
	})

	showHTML := fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d", f.freeID))

	// Alimentation stays…
	require.Contains(t, showHTML, `<label class="small d-block">Feeding</label>`,
		"read mode must keep the animal's own alimentation field")
	// …but no species diet frame, in any locale wording.
	require.NotContains(t, showHTML, "Diet by life stage")
	require.NotContains(t, showHTML, "Régime alimentaire par stade")
	require.NotContains(t, showHTML, "Fütterung nach Lebensstadium")
	require.NotContains(t, showHTML, "Voeding per levensfase")
	require.NotContains(t, showHTML, "Diet frame must not render #199-18",
		"guide text must not leak into the read mode")
}
