package actions

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestAnimalShowDiscoveryReasonPlacement (#199-19): in the animal sheet read
// mode, the discovery tab must show the "Reason" (condition) entry directly
// below "Cause d'entrée" — as in edit mode — and the FR label must be
// "Raison" (not "Condition").
func TestAnimalShowDiscoveryReasonPlacement(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	// Give the animal a discovery reason so the block renders. Find does not
	// eager-load associations — load the discovery row explicitly.
	reason := "e2e-199-19 " + uuid.Must(uuid.NewV4()).String()[:8]
	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	d := models.Discovery{}
	require.NoError(t, tx.Find(&d, a.DiscoveryID))
	d.Reason.Scan(reason)
	require.NoError(t, tx.Update(&d))
	t.Cleanup(func() {
		d.Reason.Scan("")
		tx.Update(&d)
	})

	getWithLang := func(lang string) string {
		t.Helper()
		req, err := http.NewRequest("GET", baseURL+fmt.Sprintf("/animals/%d", f.freeID), nil)
		require.NoError(t, err)
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
		resp, err := client.Do(req)
		require.NoError(t, err)
		b, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		return string(b)
	}

	// Extract the nav-discovery tab section only (the audit log at the page
	// bottom can echo raw values and must not confuse the assertions).
	discoveryTab := func(html string) string {
		t.Helper()
		start := strings.Index(html, `id="nav-discovery"`)
		require.NotEqual(t, -1, start, "nav-discovery tab missing")
		end := strings.Index(html[start:], `id="nav-intake"`)
		require.NotEqual(t, -1, end, "nav-intake tab missing")
		return html[start : start+end]
	}

	// EN: Reason directly after Entry Cause, before Postal Code.
	tab := discoveryTab(getWithLang("en-US"))
	iCause := strings.Index(tab, ">Entry Cause</label>")
	iReason := strings.Index(tab, ">Reason</label>")
	iPostal := strings.Index(tab, ">Postal Code</label>")
	require.NotEqual(t, -1, iCause, "EN Entry Cause missing")
	require.NotEqual(t, -1, iReason, "EN Reason missing")
	require.NotEqual(t, -1, iPostal, "EN Postal Code missing")
	require.True(t, iCause < iReason && iReason < iPostal,
		"EN order must be Entry Cause < Reason < Postal Code (got %d, %d, %d)", iCause, iReason, iPostal)
	require.Contains(t, tab, reason, "reason value must render")

	// FR: label is «Raison», placed right after «Cause d'entrée», and the
	// old «Condition» label is gone.
	tab = discoveryTab(getWithLang("fr"))
	iCause = strings.Index(tab, ">Cause d'entrée</label>")
	iReason = strings.Index(tab, ">Raison</label>")
	iPostal = strings.Index(tab, ">Code Postal</label>")
	require.NotEqual(t, -1, iCause, "FR Cause d'entrée missing")
	require.NotEqual(t, -1, iReason, "FR Raison missing")
	require.NotEqual(t, -1, iPostal, "FR Code Postal missing")
	require.True(t, iCause < iReason && iReason < iPostal,
		"FR order must be Cause d'entrée < Raison < Code Postal (got %d, %d, %d)", iCause, iReason, iPostal)
	require.NotContains(t, tab, ">Condition</label>", "FR label must be Raison, not Condition")

	// DE/NL keep their labels, same placement.
	tab = discoveryTab(getWithLang("de"))
	require.True(t,
		strings.Index(tab, ">Einzugsgrund</label>") < strings.Index(tab, ">Grund</label>") &&
			strings.Index(tab, ">Grund</label>") < strings.Index(tab, ">Postleitzahl</label>"),
		"DE order must be Einzugsgrund < Grund < Postleitzahl")
	tab = discoveryTab(getWithLang("nl"))
	require.True(t,
		strings.Index(tab, ">Reden van binnenkomst</label>") < strings.Index(tab, ">Reden</label>") &&
			strings.Index(tab, ">Reden</label>") < strings.Index(tab, ">Postcode</label>"),
		"NL order must be Reden van binnenkomst < Reden < Postcode")
}
