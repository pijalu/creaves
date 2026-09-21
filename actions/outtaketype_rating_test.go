package actions

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"creaves/models"
)

// Issue #199-11: an outtake type may carry no rating (NULL, e.g. OT7
// "Doublon"). The animal sheet must then hide the Positive/Negative/Neutral
// badge entirely; a rated type still shows it.

const ratingTestYear = 2097

// seedRatingFixture builds two animals: one whose outtake type has a NULL
// rating, one with a numeric rating. Returns the two animal IDs.
func seedRatingFixture(t *testing.T) (noRatingAnimal, ratedAnimal int) {
	t.Helper()
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	mkUUID := func(i byte) string {
		return fmt.Sprintf("cc000000-0000-0000-0000-%012d", i)
	}
	nullTypeID := mkUUID(1)
	ratedTypeID := mkUUID(2)
	nullOuttakeID := mkUUID(3)
	ratedOuttakeID := mkUUID(4)
	ageID := mkUUID(5)
	typID := mkUUID(6)
	dscID := mkUUID(7)
	dcvID := mkUUID(8)
	intNull := mkUUID(9)
	intRated := mkUUID(10)

	cleanup := func() {
		stmts := []string{
			"DELETE FROM animals WHERE year = " + fmt.Sprint(ratingTestYear),
			"DELETE FROM outtakes WHERE id IN ('" + nullOuttakeID + "','" + ratedOuttakeID + "')",
			"DELETE FROM outtaketypes WHERE id IN ('" + nullTypeID + "','" + ratedTypeID + "')",
			"DELETE FROM intakes WHERE id IN ('" + intNull + "','" + intRated + "')",
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
		// one outtake type with NULL rating (like OT7), one rated -1
		"INSERT INTO outtaketypes (id, name, `def`, dead, error, rating, location_mode, created_at, updated_at) VALUES ('" + nullTypeID + "', 'rating-test-null', 0, 0, 0, NULL, 'free', NOW(), NOW())",
		"INSERT INTO outtaketypes (id, name, `def`, dead, error, rating, location_mode, created_at, updated_at) VALUES ('" + ratedTypeID + "', 'rating-test-neg', 0, 1, 0, -1, 'free', NOW(), NOW())",
		"INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + nullOuttakeID + "', NOW(), '" + nullTypeID + "', NOW(), NOW())",
		"INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + ratedOuttakeID + "', NOW(), '" + ratedTypeID + "', NOW(), NOW())",
		"INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + ageID + "', 'ratingtest', 0, NOW(), NOW())",
		"INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + typID + "', 'ratingtest', 0, NOW(), NOW())",
		"INSERT INTO discoverers (id, firstname, lastname, created_at, updated_at) VALUES ('" + dcvID + "', 'rating', 'tester', NOW(), NOW())",
		"INSERT INTO discoveries (id, date, location, reason, discoverer_id, created_at, updated_at) VALUES ('" + dscID + "', NOW(), 'Ratingville', 'test', '" + dcvID + "', NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intNull + "', NOW(), NOW(), NOW())",
		"INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intRated + "', NOW(), NOW(), NOW())",
		"INSERT INTO animals (year, yearNumber, species, IntakeDate, animalage_id, animaltype_id, intake_id, discovery_id, outtake_id, created_at, updated_at) VALUES (" + fmt.Sprint(ratingTestYear) + ", 9001, 'Nullus ratingus', NOW(), '" + ageID + "', '" + typID + "', '" + intNull + "', '" + dscID + "', '" + nullOuttakeID + "', NOW(), NOW())",
		"INSERT INTO animals (year, yearNumber, species, IntakeDate, animalage_id, animaltype_id, intake_id, discovery_id, outtake_id, created_at, updated_at) VALUES (" + fmt.Sprint(ratingTestYear) + ", 9002, 'Ratus ratingus', NOW(), '" + ageID + "', '" + typID + "', '" + intRated + "', '" + dscID + "', '" + ratedOuttakeID + "', NOW(), NOW())",
	}
	for _, s := range stmts {
		if err := models.DB.RawQuery(s).Exec(); err != nil {
			t.Fatalf("seed rating fixture %q: %v", s, err)
		}
	}

	var a1, a2 models.Animal
	if err := models.DB.Where("year = ? AND yearNumber = ?", ratingTestYear, 9001).First(&a1); err != nil {
		t.Fatalf("load null-rating animal: %v", err)
	}
	if err := models.DB.Where("year = ? AND yearNumber = ?", ratingTestYear, 9002).First(&a2); err != nil {
		t.Fatalf("load rated animal: %v", err)
	}
	return a1.ID, a2.ID
}

func animalShowBody(t *testing.T, client *http.Client, baseURL string, animalID int) string {
	t.Helper()
	path := fmt.Sprintf("/animals/%d", animalID)
	resp, err := client.Get(baseURL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
	}
	return string(page)
}

func TestAnimalsShowHidesBadgeWhenRatingNull(t *testing.T) {
	nullAnimal, ratedAnimal := seedRatingFixture(t)
	client, baseURL := adminClientWithURL(t)

	// NULL rating: no outcome badge at all near the outtake type.
	body := animalShowBody(t, client, baseURL, nullAnimal)
	if !strings.Contains(body, "rating-test-null") {
		t.Fatalf("null-rating animal show missing outtake type name")
	}
	for _, word := range []string{"badge-danger", "badge-success", ">Negative<", ">Positive<", ">Neutral<"} {
		if strings.Contains(body, word) {
			t.Errorf("null-rating animal show must not contain %q", word)
		}
	}

	// rated type: badge present.
	body = animalShowBody(t, client, baseURL, ratedAnimal)
	if !strings.Contains(body, "badge-danger") || !strings.Contains(body, ">Negative<") {
		t.Errorf("rated (-1) animal show must render a danger/Negative badge")
	}
}
