package actions

// Phase 7 / defect D7 — /zones/{id} show completeness + i18n.
//
// 7-T1 browser GET → 200 HTML, every zone field rendered including
//     RequiresCleanup (bool2html), back link to /zones, edit + delete
//     buttons.
// 7-T2 4-locale sweep: localized title/labels/values, no missing-key
//     marker, no hardcoded English in fr/de/nl.

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// zoneGetHTML issues a browser-style GET with an explicit lang cookie and
// returns status+body.
func zoneGetHTML(t *testing.T, client *http.Client, baseURL, path, lang string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// showZone builds one zone with RequiresCleanup on so the show page renders
// every field.
func showZone(t *testing.T, marker string) *models.Zone {
	t.Helper()
	z := &models.Zone{
		ID:              uuid.Must(uuid.NewV4()),
		Zone:            "D7-" + marker,
		Type:            "external",
		Default:         false,
		RequiresCleanup: true,
	}
	require.NoError(t, models.DB.Create(z))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM zones WHERE id = ?", z.ID).Exec()
	})
	return z
}

// TestZoneShowAllFields (7-T1): a browser GET renders every zone field —
// Zone, Type (localized value), Default, RequiresCleanup — plus back link
// and admin buttons. French UI language (the app default in production).
func TestZoneShowAllFields(t *testing.T) {
	f := setupPlanFixture(t)
	zone := showZone(t, f.marker)
	client, baseURL := planAdminClient(t)

	code, body := zoneGetHTML(t, client, baseURL, "/zones/"+zone.ID.String(), "fr")
	require.Equal(t, http.StatusOK, code, "body: %.500s", body)

	// French: localized labels/values.
	require.Contains(t, body, "Détails de la zone", "localized title")
	require.Contains(t, body, zone.Zone, "zone name")
	require.Contains(t, body, "externe au centre", "localized type value")
	require.Contains(t, body, "Nettoyage requis", "requires_cleanup label")
	require.Contains(t, body, ">✓</p>", "requires_cleanup bool2html true")

	// Both bools rendered: Default false → ×, RequiresCleanup true → ✓.
	require.Contains(t, body, ">×</p>", "default bool2html false")

	// Back link + edit + delete.
	require.Contains(t, body, `href="/zones/"`, "back link target")
	require.Contains(t, body, "/zones/"+zone.ID.String()+"/edit", "edit link")
	require.Contains(t, body, `data-method="DELETE"`, "delete button")
}

// TestZoneShowAllLocales (7-T2): the show page renders localized title +
// field labels + type values in every UI language with no missing-key
// marker and no hardcoded English.
func TestZoneShowAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	zone := showZone(t, f.marker)
	client, baseURL := planAdminClient(t)

	want := map[string][]string{
		"fr":    {"Détails de la zone", "Retour à toutes les zones", "Type", "Défaut", "Nettoyage requis", "externe au centre"},
		"en-US": {"Zone Details", "Back to all Zones", "Type", "Default", "Requires cleanup", "external to the center"},
		"de":    {"Zonedetails", "Zurück zu allen Zonen", "Typ", "Standard", "Reinigung erforderlich", "extern zum Zentrum"},
		"nl":    {"Zonedetails", "Terug naar alle zones", "Type", "Standaard", "Schoonmaak vereist", "extern aan het centrum"},
	}
	// Hardcoded English strings that must NOT appear outside en-US.
	banned := map[string][]string{
		"fr": {"Zone Details", "Back to all Zones", "external to the center", "internal to the center", ">Default<"},
		"de": {"Zone Details", "Back to all Zones", "external to the center", "internal to the center", ">Default<"},
		"nl": {"Zone Details", "Back to all Zones", "external to the center", "internal to the center", ">Default<"},
	}

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			code, body := zoneGetHTML(t, client, baseURL, "/zones/"+zone.ID.String(), lang)
			require.Equal(t, http.StatusOK, code, "%s body: %.500s", lang, body)
			require.NotContains(t, body, "translation missing",
				"%s must not render missing-key markers", lang)
			require.Contains(t, body, zone.Zone, "%s zone name", lang)
			for _, s := range want[lang] {
				esc := strings.ReplaceAll(s, "'", "&#39;")
				require.Contains(t, body, esc, "%s must render %q", lang, s)
			}
			for _, s := range banned[lang] {
				require.NotContains(t, body, s, "%s must not render hardcoded %q", lang, s)
			}
		})
	}
}

// TestZoneShowInternalType (7-T1b): a non-external zone renders the
// "internal to the center" localized value branch.
func TestZoneShowInternalType(t *testing.T) {
	f := setupPlanFixture(t)
	zone := showZone(t, f.marker)
	zone.Type = "internal"
	zone.RequiresCleanup = false
	require.NoError(t, models.DB.Save(zone))
	client, baseURL := planAdminClient(t)

	code, body := zoneGetHTML(t, client, baseURL, "/zones/"+zone.ID.String(), "fr")
	require.Equal(t, http.StatusOK, code, "body: %.500s", body)
	require.Contains(t, body, "interne au centre", "internal branch (fr)")
	// RequiresCleanup false → both bools now render ×.
	require.NotContains(t, body, ">✓</p>", "no true bool expected")
}
