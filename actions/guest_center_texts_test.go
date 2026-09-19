package actions

// #197 sub-item 9: the guest pages gain the center's own free texts
// (ConfigSettings.GuestText1/GuestText2) and report the intake findings
// has_wounds / has_parasites to the discoverer.

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// guestPageGET fetches a public (unauthenticated) guest page.
func guestPageGET(t *testing.T, srv *httptest.Server, path string) string {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s failed: %s", path, truncate(body, 500))
	return string(body)
}

// seedGuestCenterConfig makes an active config whose settings carry the two
// center guest texts and installs it as the current config (restored on
// cleanup).
func seedGuestCenterConfig(t *testing.T, text1, text2 string) {
	t.Helper()
	saved := CurrentConfigGet()
	t.Cleanup(func() { CurrentConfigSet(saved) })

	cfg := seedConfig(t, "guest-texts", true)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE id = ?", cfg.ID).Exec()
	})
	s := models.DefaultSettings()
	s.GuestText1 = text1
	s.GuestText2 = text2
	require.NoError(t, cfg.SetSettings(s))
	require.NoError(t, models.DB.Update(cfg))
	CurrentConfigSet(cfg)
}

// TestGuestCenterTextsDisplayed: the center free texts from the active
// config are rendered on the guest form and on the guest status view.
func TestGuestCenterTextsDisplayed(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createGuestFixtures(t, tx)
	seedGuestCenterConfig(t, "Blabla-one-"+f.marker, "Blabla-two-"+f.marker)

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	newPage := guestPageGET(t, srv, "/guest/")
	require.Contains(t, newPage, "Blabla-one-"+f.marker)
	require.Contains(t, newPage, "Blabla-two-"+f.marker)

	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.animalID))
	tok, err := guestEnsureToken(tx, &a)
	require.NoError(t, err)
	showPage := guestPageGET(t, srv, fmt.Sprintf("/guest/?number=%d&token=%s&lang=en", guestFixtureYearNumber(t, tx, f.animalID), tok))
	require.Contains(t, showPage, "Blabla-one-"+f.marker)
	require.Contains(t, showPage, "Blabla-two-"+f.marker)
}

// TestGuestCenterTextsHiddenWhenEmpty: with untouched settings no (empty)
// guest text block is rendered.
func TestGuestCenterTextsHiddenWhenEmpty(t *testing.T) {
	requireMySQLTestDB(t)
	seedGuestCenterConfig(t, "", "")

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	newPage := guestPageGET(t, srv, "/guest/")
	require.NotContains(t, newPage, "Blabla-")
	require.NotContains(t, newPage, "Center information")
	require.NotContains(t, newPage, "Informations du centre")
}

// TestGuestShowsIntakeFindings: the status view tells the discoverer when
// the animal was brought in with wounds and/or parasites, and stays silent
// when neither flag is set (#197 sub-item 9).
func TestGuestShowsIntakeFindings(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createGuestFixtures(t, tx)
	seedGuestCenterConfig(t, "", "")

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	// Flag both findings on the present animal's intake.
	require.NoError(t, tx.RawQuery(
		"UPDATE intakes SET has_wounds = 1, has_parasites = 1 WHERE id = (SELECT intake_id FROM animals WHERE id = ?)",
		f.animalID).Exec())

	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.animalID))
	tok, err := guestEnsureToken(tx, &a)
	require.NoError(t, err)
	showPage := guestPageGET(t, srv, fmt.Sprintf("/guest/?number=%d&token=%s&lang=en", guestFixtureYearNumber(t, tx, f.animalID), tok))
	require.Contains(t, showPage, "Your animal is injured.")
	require.Contains(t, showPage, "Your animal has parasites.")

	// The fr locale carries the ticket wording.
	frPage := guestPageGET(t, srv, fmt.Sprintf("/guest/?number=%d&token=%s&lang=fr", guestFixtureYearNumber(t, tx, f.animalID), tok))
	require.Contains(t, frPage, "Votre animal est blessé.")
	require.Contains(t, frPage, "Votre animal a des parasites.")

	// The control animal (untouched intake) shows neither line, in any locale.
	gone := models.Animal{}
	require.NoError(t, tx.Find(&gone, f.animalGoneID))
	goneTok, err := guestEnsureToken(tx, &gone)
	require.NoError(t, err)
	plainPage := guestPageGET(t, srv, fmt.Sprintf("/guest/?number=%d&token=%s&lang=en", guestFixtureYearNumber(t, tx, f.animalGoneID), goneTok))
	require.NotContains(t, plainPage, "Your animal is injured.")
	require.NotContains(t, plainPage, "Your animal has parasites.")
}
