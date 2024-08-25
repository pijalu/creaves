package actions

import (
	"creaves/models"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

const FEEDING_SQL = `
SELECT a.id, a.year, a.yearNumber, a.species, a.cage, a.zone, a.feeding, a.force_feed, a.feeding_start, a.feeding_end, a.feeding_period, MAX(c.date) AS last_feeding
FROM animals a
LEFT JOIN cares c ON (a.id = c.animal_id and c.type_id in (select id from caretypes where type=1))
WHERE a.outtake_id IS NULL and a.feeding_start IS NOT NULL and a.feeding_end IS NOT NULL
AND a.feeding_period > 0
GROUP BY a.id;`

const HIGHTIMELIMIT = 2 * time.Hour
const NEARTIMELIMIT = 15 * time.Minute

type AnimalFeeding struct {
	ID         int          `db:"id"`
	Year       int          `db:"year"`
	YearNumber int          `db:"yearNumber"`
	Species    string       `db:"species"`
	Cage       nulls.String `db:"cage"`
	Zone       nulls.String `db:"zone"`

	Feeding   string `db:"feeding"`
	ForceFeed bool   `db:"force_feed"`

	FeedingStart  time.Time  `db:"feeding_start"`
	FeedingEnd    time.Time  `db:"feeding_end"`
	FeedingPeriod int        `db:"feeding_period"`
	LastFeeding   nulls.Time `db:"last_feeding"`

	NextFeeding nulls.Time
	// 0 - Late, 1 - in time, 2 - future

	NextFeedingCode int
}

func (a AnimalFeeding) String() string {
	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return fmt.Sprint(string(b))
}

func (a AnimalFeeding) NextFeedingTime() string {
	if a.NextFeeding.Valid {
		return a.NextFeeding.Time.Format("15:04")
	}
	return "n.a."
}

// YearNumberFormatted returns the year number formatted
func (a AnimalFeeding) YearNumberFormatted() string {
	return fmt.Sprintf("%d/%d", a.YearNumber, a.Year%100)
}

type FeedingByZoneMap map[models.AnimalViewKey]([]AnimalFeeding)

// Return orderedkeys from map
func (t FeedingByZoneMap) OrderedKeys() []models.AnimalViewKey {
	var keys []models.AnimalViewKey
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys
}

func computeAnimalFeeding(c buffalo.Context) (FeedingByZoneMap, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	afRaw := []AnimalFeeding{}
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(FEEDING_SQL).All(&afRaw); err != nil {
		return nil, err
	}

	// Set time
	// Fix start-end
	now := time.Now()
	lowNearLimit := now.Add(-1 * NEARTIMELIMIT)
	highNearLimit := now.Add(1 * NEARTIMELIMIT)

	//_, offsetSec := now.Zone()

	afCalc := []AnimalFeeding{}

	// Calculate next feedings
	for _, af := range afRaw {
		// Recalc Start/End to match today
		af.FeedingStart = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingStart.Hour(), af.FeedingStart.Minute(), 0, 0, time.Local)
		af.FeedingEnd = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingEnd.Hour(), af.FeedingEnd.Minute(), 0, 0, time.Local)
		if af.FeedingEnd.Before(af.FeedingStart) {
			af.FeedingEnd = af.FeedingEnd.Add(24 * time.Hour)
		}

		startTime := af.FeedingStart
		if af.LastFeeding.Valid {
			// fix last feeding time zone
			startTime = switchTimeZone(af.LastFeeding.Time, time.Local)

			previousStarttime := af.FeedingStart.Add(-24 * time.Hour)
			previousEndtime := af.FeedingEnd.Add(-24 * time.Hour)

			if startTime.After(af.FeedingEnd) || startTime.Equal(af.FeedingEnd) {
				startTime = af.FeedingStart.Add(24 * time.Hour)
			} else if startTime.Before(previousStarttime) {
				startTime = af.FeedingStart
			} else if startTime.Before(af.FeedingStart) && (startTime.After(previousEndtime) || startTime.Equal(previousEndtime)) {
				startTime = af.FeedingStart
			} else {
				startTime = startTime.Add(time.Minute * time.Duration(af.FeedingPeriod))
			}
		}

		// not in the future
		if startTime.Before(now.Add(HIGHTIMELIMIT)) {
			af.NextFeeding = nulls.NewTime(startTime)

			if startTime.Before(lowNearLimit) {
				af.NextFeedingCode = 0
			} else if startTime.Before(highNearLimit) {
				af.NextFeedingCode = 1
			} else {
				af.NextFeedingCode = 2
			}

			afCalc = append(afCalc, af)
		}
	}

	// Sort the feeding (always 1 elem)
	sort.Slice(afCalc, func(i, j int) bool {
		return afCalc[i].NextFeeding.Time.Before(afCalc[j].NextFeeding.Time)
	})

	feedingByZone := FeedingByZoneMap{}
	for _, f := range afCalc {
		keyZone := models.AnimalViewKey{ID: sha256("?"), Name: "?"}
		if f.Zone.Valid {
			keyZone.ID = sha256(f.Zone.String)
			keyZone.Name = f.Zone.String
		}
		feedingByZone[keyZone] = append(feedingByZone[keyZone], f)
	}

	return feedingByZone, nil
}

// FeedingFeeding default implementation.
func FeedingIndex(c buffalo.Context) error {
	zm, err := zonesMap(c)
	if err != nil {
		return err
	}
	c.Set("zoneMap", zm)

	af, err := computeAnimalFeeding(c)
	if err != nil {
		return err
	}
	c.Set("feedingByZone", af)

	return c.Render(http.StatusOK, r.HTML("feeding/index.html"))
}
