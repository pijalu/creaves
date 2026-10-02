package actions

import (
	"strings"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// R4-7.11b (feedback item 12): "add a link between activities and the logged
// items — so in case of a feed/observation a click on the action can be seen
// in the animal feeding; back should route back to the source".
//
// An applied activity on the animal Protocol tab now links to the record that
// was actually logged (the feeding / care entry), and the record's own back
// button returns to the animal's Plan tab.
func TestAnimalAppliedRowLinksToFulfillment(t *testing.T) {
	forks := []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `if (item.FulfillmentLink != "")`, f,
			"the applied branch offers a link only when a fulfillment exists")
		require.Contains(t, raw, `class="badge badge-success plan-item-view"`, f,
			"the done badge doubles as the link to the logged record")
		require.Contains(t, raw, `href="<%= item.FulfillmentLink %>"`, f,
			"the link targets the fulfillment record")
		require.Contains(t, raw, `t("care_plan.animal_plans.view_record")`, f,
			"the link carries an accessible label")
	}
}

// The back target of an animal-page fulfillment link is THIS page's Plan tab,
// so the record's back button lands on the source, not a dead end.
func TestAnimalDayCardBackTargetsPlanTab(t *testing.T) {
	// exercise the pure link builder the animal page uses
	back := "/animals/7#nav-plan"
	require.Equal(t, "/treatments/t-1?back=%2Fanimals%2F7%23nav-plan",
		cardFulfillmentLink(models.ApplicationFulfillmentTreatment, "t-1", false, back))
	require.Equal(t, "/cares/c-1?back=%2Fanimals%2F7%23nav-plan",
		cardFulfillmentLink(models.ApplicationFulfillmentCare, "c-1", false, back))
}

// The care detail page must honour back the same (sanitized, labelled) way
// the treatment page does — a care-plan link lands here too.
func TestCareShowUsesSanitizedBack(t *testing.T) {
	forks := []string{
		"../templates/cares/show.plush.html",
		"../templates/cares/show.plush.fr.html",
		"../templates/cares/show.plush.de.html",
		"../templates/cares/show.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.NotContains(t, raw, `params["back"]`, f,
			"the raw back param must not reach the template (unsanitized redirect)")
		require.Contains(t, raw, `if (back)`, f, "the sanitized back is what gates the button")
		require.Contains(t, raw, `linkTo(back,`, f)
		require.Contains(t, raw, `t(landingBackLabel)`, f, "the label names the destination")
	}

	// the handler sets both values the template now reads
	handler := readTemplate(t, "../actions/cares.go")
	require.Contains(t, handler, `unwrapBackChain(safeBackParam(c))`)
	require.Contains(t, handler, `c.Set("landingBackLabel", landingBackLabelKey(back))`)
}

// The localized "view the logged record" label exists in every locale.
func TestViewRecordLabelInAllLocales(t *testing.T) {
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		require.Contains(t, raw, `- id: "care_plan.animal_plans.view_record"`, lang)
		i := strings.Index(raw, `- id: "care_plan.animal_plans.view_record"`)
		rest := raw[i:]
		j := strings.Index(rest, "\n- id:")
		require.Greater(t, j, 0, lang)
		require.Contains(t, rest[:j], "translation:", lang+" — the label has no translation")
	}
}
