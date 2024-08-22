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

const LOWTIMELIMIT = 4 * time.Hour
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

	NextFeedings []time.Time
	// 0 - Late, 1 - in time, 2 - future
	NextFeedingCode int

	// Missing feeding count
	MissingFeedingCount int
}

func (a AnimalFeeding) String() string {
	b, err := json.Marshal(a)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	return fmt.Sprint(string(b))
}

func (a AnimalFeeding) NextFeedingTime() string {
	if len(a.NextFeedings) == 0 {
		return "n.a."
	}
	return a.NextFeedings[0].Format("15:04")
}

func (a AnimalFeeding) NextFeedingTimes() []string {
	times := []string{}
	for i := 0; i < len(a.NextFeedings); i++ {
		times = append(times, a.NextFeedings[i].Format("15:04"))
	}
	return times
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
	//_, offsetSec := now.Zone()

	lowTimeLimit := now.Add(-1 * LOWTIMELIMIT)
	highTimeLimit := now.Add(HIGHTIMELIMIT)

	// To calculate if near
	lowNearTimeLimit := now.Add(-1 * NEARTIMELIMIT)
	highNearTimeLimit := now.Add(NEARTIMELIMIT)

	c.Logger().Debugf("low time: %s - high time: %s", lowTimeLimit.Format(time.RFC3339), highTimeLimit.Format(time.RFC3339))

	afCalc := []AnimalFeeding{}

	// Calculate next feedings
	for _, af := range afRaw {
		// Recalc Start/End to match today
		af.FeedingStart = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingStart.Hour(), af.FeedingStart.Minute(), 0, 0, time.Local)
		af.FeedingEnd = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingEnd.Hour(), af.FeedingEnd.Minute(), 0, 0, time.Local)

		//c.Logger().Debugf("Start: %s - End: %s", af.FeedingStart.Format(time.RFC3339), af.FeedingEnd.Format(time.RFC3339))
		startTime := af.FeedingStart
		if af.LastFeeding.Valid {
			// fix last feeding time zone
			af.LastFeeding.Time = switchTimeZone(af.LastFeeding.Time, time.Local)
			if af.LastFeeding.Time.After(af.FeedingStart) {
				startTime = af.LastFeeding.Time.Add(time.Minute * time.Duration(af.FeedingPeriod))
				switchTimeZone(startTime, time.Local)
			}
		}
		//c.Logger().Debugf("startTime (start): %s", startTime.Format(time.RFC3339))

		for startTime.Before(af.FeedingEnd) && startTime.Before(lowTimeLimit) {
			startTime = startTime.Add(time.Minute * time.Duration(af.FeedingPeriod))
			//c.Logger().Debugf("startTime (next): %s", startTime.Format(time.RFC3339))
		}
		for startTime.Before(af.FeedingEnd) && startTime.Before(highTimeLimit) {
			//c.Logger().Debugf("startTime (end): %s", startTime.Format(time.RFC3339))
			af.NextFeedings = append(af.NextFeedings, startTime)

			if startTime.Before(lowNearTimeLimit) {
				af.MissingFeedingCount++
			}

			// Calc color of feeding (first occurence)
			if len(af.NextFeedings) == 1 {
				if startTime.After(af.FeedingStart) && !af.LastFeeding.Valid {
					af.NextFeedingCode = 0 // late - but starttime moved
				} else if startTime.Before(lowNearTimeLimit) {
					af.NextFeedingCode = 0 // late
				} else if startTime.Before(highNearTimeLimit) {
					af.NextFeedingCode = 1 // to do
				} else {
					af.NextFeedingCode = 2 // future
				}
			}
			// next
			startTime = startTime.Add(time.Minute * time.Duration(af.FeedingPeriod))
		}
		if len(af.NextFeedings) > 0 {
			afCalc = append(afCalc, af)
		}
	}

	// Sort the feeding (always 1 elem)
	sort.Slice(afCalc, func(i, j int) bool {
		return afCalc[i].NextFeedings[0].Before(afCalc[j].NextFeedings[0])
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
