package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
)

// Issue #199-9: the "Back to animals in care" button on the animal sheet must
// return to the landing tab the user came from. The landing page passes its
// tab anchor through the safe `back` param; without a param, the button
// defaults to the animal's own zone tab in the default (zone) landing view.

const landingBackTestYear = 2098

type landingBackFixture struct {
	animalID int
	zone     string
}

func seedLandingBackFixture(t *testing.T) *landingBackFixture {
	t.Helper()
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	mkUUID := func(i byte) string {
		return fmt.Sprintf("bb000000-0000-0000-0000-%012d", i)
	}
	ageID := mkUUID(1)
	typID := mkUUID(2)
	dscID := mkUUID(3)
	dcvID := mkUUID(4)
	intID := mkUUID(5)

	f := &landingBackFixture{
		zone: "TS-Zone-" + uuid.Must(uuid.NewV4()).String()[:8],
	}

	cleanup := func() {
		stmts := []string{
			"DELETE FROM animals WHERE year = " + fmt.Sprint(landingBackTestYear),
			"DELETE FROM intakes WHERE id = '" + intID + "'",
			"DELETE FROM discoveries WHERE id = '" + dscID + "'",
			"DELETE FROM discoverers WHERE id = '" + dcvID + "'",
			"DELETE FROM animalages WHERE id = '" + ageID + "'",
			"DELETE FROM animaltypes WHERE id = '" + typID + "'",
		}
		for _, s := range stmts {
			models.DB.RawQuery(s).Exec()
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	stmts := []string{
		"INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + ageID + "', 'lbtest', 0, NOW(), NOW())",
		"INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + typID + "', 'lbtest', 0, NOW(), NOW())",
		"INSERT INTO discoverers (id, firstname, lastname, created_at, updated_at) VALUES ('" + dcvID + "', 'lb', 'tester', NOW(), NOW())",
		"INSERT INTO discoveries (id, date, location, reason, discoverer_id, created_at, updated_at) VALUES ('" + dscID + "', NOW(), 'Lbville', 'test', '" + dcvID + "', NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intID + "', NOW(), NOW(), NOW())",
		"INSERT INTO animals (year, yearNumber, species, zone, IntakeDate, animalage_id, animaltype_id, intake_id, discovery_id, created_at, updated_at) VALUES (" + fmt.Sprint(landingBackTestYear) + ", 9101, 'Lb testus', '" + f.zone + "', NOW(), '" + ageID + "', '" + typID + "', '" + intID + "', '" + dscID + "', NOW(), NOW())",
	}
	for _, s := range stmts {
		if err := models.DB.RawQuery(s).Exec(); err != nil {
			t.Fatalf("seed landing-back fixture %q: %v", s, err)
		}
	}
	a := &models.Animal{}
	if err := models.DB.Where("year = ? AND yearNumber = 9101", landingBackTestYear).First(a); err != nil {
		t.Fatalf("load fixture animal: %v", err)
	}
	f.animalID = a.ID
	return f
}

// backLinkNeedle is the (untranslated, en locale) label of the back button —
// unique on the show page, unlike the btn-info class.
var backLinkNeedle = "Back to animals in care"

func showBackHref(t *testing.T, client *http.Client, baseURL, path string) string {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
	}
	body := string(page)
	i := strings.Index(body, backLinkNeedle)
	if i < 0 {
		t.Fatalf("no back button (%s) found on %s", backLinkNeedle, path)
	}
	// href precedes the class attribute in linkTo output
	seg := body[:i]
	j := strings.LastIndex(seg, `href="`)
	if j < 0 {
		t.Fatalf("no href before back button on %s", path)
	}
	rest := seg[j+len(`href="`):]
	k := strings.Index(rest, `"`)
	if k < 0 {
		t.Fatalf("unterminated href on %s", path)
	}
	return rest[:k]
}

func TestAnimalsShowLandingBackDefault(t *testing.T) {
	f := seedLandingBackFixture(t)
	client, baseURL := adminClientWithURL(t)

	href := showBackHref(t, client, baseURL, fmt.Sprintf("/animals/%d", f.animalID))
	want := "/" + landingTabAnchor(f.zone)
	if href != want {
		t.Errorf("back href = %q, want %q (animal zone tab)", href, want)
	}
}

func TestAnimalsShowLandingBackFromParam(t *testing.T) {
	f := seedLandingBackFixture(t)
	client, baseURL := adminClientWithURL(t)

	// landing tab 2 (zone view): anchor passed through the safe back param
	back := "/#t-deadbeef"
	href := showBackHref(t, client, baseURL,
		fmt.Sprintf("/animals/%d?back=%s", f.animalID, url.QueryEscape(back)))
	if href != back {
		t.Errorf("back href = %q, want %q (originating tab honored)", href, back)
	}

	// type view keeps its ?v=type query in the back target
	back = "/?v=type#t-cafef00d"
	href = showBackHref(t, client, baseURL,
		fmt.Sprintf("/animals/%d?back=%s", f.animalID, url.QueryEscape(back)))
	if href != back {
		t.Errorf("back href = %q, want %q (type view preserved)", href, back)
	}

	// external targets stay rejected (open-redirect guard)
	href = showBackHref(t, client, baseURL,
		fmt.Sprintf("/animals/%d?back=%s", f.animalID, url.QueryEscape("https://evil.example")))
	if href == "https://evil.example" || strings.Contains(href, "evil.example") {
		t.Errorf("back href = %q, external target must be rejected", href)
	}
}
