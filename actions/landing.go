package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

type listAnimalWithCleanCageReply struct {
	Id int `db:"ID"`
}

// SQL_ANIMAL_WITH_CLEAN_CAGE lists animals having a care flagged clean=1
// dated within the rolling last 24 hours (BUG-2). It used to be
// `c.date >= DATE_ADD(CURDATE(), INTERVAL 3 HOUR)`, which dropped clean
// cares dated 00:00–03:00 of the current calendar day (and yesterday
// evening cares after midnight) even though they belong to the current
// care-day cycle — while the documented semantics (function comment,
// ACTIONS_DOCUMENTATION.md) were "within the last 24h". The cutoff is
// bound from Go instead of using DATE_SUB(NOW(), INTERVAL 24 HOUR) so the
// exact same statement runs on MySQL (dev/prod) and on the SQLite test
// harness, which has no MySQL date functions.
const SQL_ANIMAL_WITH_CLEAN_CAGE = `
	SELECT DISTINCT c.animal_id as 'ID'
	FROM cares c
	WHERE c.clean = 1
		AND c.date >= ?
`

// List all animals id with a clean cage within the last 24h
func listAnimalWithCleanCage(c buffalo.Context) (map[int]bool, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	return animalIDsCleanCageSince(tx, time.Now().Add(-24*time.Hour))
}

// animalIDsCleanCageSince runs the clean-cage lookup with a caller-supplied
// cutoff (animals with a clean=1 care dated at or after `since`) so tests
// can pin the window boundary deterministically.
func animalIDsCleanCageSince(tx *pop.Connection, since time.Time) (map[int]bool, error) {
	var a []listAnimalWithCleanCageReply
	// RawQuery returns raw rows; Eager() would be ignored here and only
	// misleads readers (it cannot preload anything onto a raw scan).
	if err := tx.RawQuery(SQL_ANIMAL_WITH_CLEAN_CAGE, since).All(&a); err != nil {
		return nil, err
	}

	//remap
	res := map[int]bool{}

	for _, item := range a {
		res[item.Id] = true
	}

	return res, nil
}

// LandingIndex is the default landing view with validation
func LandingIndex(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Load config if not already loaded
	if CurrentConfigGet() == nil {
		if _, err := LoadConfig(tx); err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}
	}

	// Load all animals with outtake_id is null (validated to ensure correct count)
	animals := models.Animals{}
	if err := tx.Where("outtake_id is null").Order("ID desc").All(&animals); err != nil {
		return c.Error(http.StatusNoContent, err)
	}

	// Log the actual number of animals returned
	fmt.Printf("Info: Loaded %d animals\n", len(animals))

	animalsByType := models.AnimalsByTypeMap{}
	animalsByZone := models.AnimalByZoneMap{}

	if _, err := EnrichAnimalsOptimized(&animals, c); err != nil {
		return err
	}

	for _, animal := range animals {
		keyType := models.AnimalViewKey{ID: sha256(animal.Animaltype.Name), Name: animal.Animaltype.Name}
		animalsByType[keyType] = append(animalsByType[keyType], animal)

		keyZone := models.AnimalViewKey{ID: sha256("?"), Name: "?"}
		if animal.Zone.Valid {
			keyZone.ID = sha256(animal.Zone.String)
			keyZone.Name = animal.Zone.String
		}
		animalsByZone[keyZone] = append(animalsByZone[keyZone], animal)
	}

	zm, err := zonesMap(c)
	if err != nil {
		return err
	}
	c.Set("zoneMap", zm)

	// Care plan badge (§7.2): open items (due/late/missing) on today's
	// plan. A planning failure degrades to zero — the badge is decorative.
	// M3: dedicated count path, no full day-plan materialization.
	// Round 10: short-TTL cached — the count is a full §6.1 assembly and
	// the landing is the most-hit page (invalidated post-commit by the
	// apply paths; 30s backstop).
	dayOpen, dayLate := CountOpenItemsCached(tx, time.Now())
	c.Set("dayPlanOpen", dayOpen)
	c.Set("dayPlanLate", dayLate)

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add clean cage flag
		animalWithCleanCage, err := listAnimalWithCleanCage(c)
		if err != nil {
			return err
		}
		c.Set("animalsWithCleanCage", animalWithCleanCage)
		c.Set("animalsByType", animalsByType)
		c.Set("animalsByZone", animalsByZone)
		// Traitements column (2026-10-09): merge the protocol's today
		// medication occurrences into the legacy per-bucket stat — protocol
		// rows only exist in the treatments table after the day's first
		// apply, so the column would otherwise read "not required" all day
		// for protocol animals.
		c.Set("treatmentStats", mergedTreatmentStats(&animals, LandingTreatmentBucketsCached(tx, time.Now())))
		return c.Render(http.StatusOK, r.HTML("landing/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animalsByType))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animalsByType))
	}).Respond(c)
}
