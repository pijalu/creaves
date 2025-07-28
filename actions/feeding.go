package actions

import (
	"creaves/models"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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

const feeding_dateFormat = "2006-01-02 15:04" // Date and 24-hour time format
func (a AnimalFeeding) NextFeedingFmt() string {
	if a.NextFeeding.Valid {
		return a.NextFeeding.Time.Format(feeding_dateFormat)
	}
	return ""
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

func calculateFeeding(af AnimalFeeding, now time.Time) AnimalFeeding {
	// Debug log for input parameters
	//log.Printf("calculateFeeding called with: %+v, now: %v", af, now)

	lowNearLimit := now.Add(-1 * NEARTIMELIMIT)
	highNearLimit := now.Add(1 * NEARTIMELIMIT)

	// Recalc Start/End to match today
	af.FeedingStart = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingStart.Hour(), af.FeedingStart.Minute(), 0, 0, time.Local)
	af.FeedingEnd = time.Date(now.Year(), now.Month(), now.Day(), af.FeedingEnd.Hour(), af.FeedingEnd.Minute(), 0, 0, time.Local)
	if af.FeedingEnd.Before(af.FeedingStart) {
		af.FeedingEnd = af.FeedingEnd.Add(24 * time.Hour)
	}

	// Debug log after adjusting feeding start/end times
	//log.Printf("After adjusting feeding times - Start: %v, End: %v", af.FeedingStart, af.FeedingEnd)

	startTime := af.FeedingStart
	if af.LastFeeding.Valid {
		// fix last feeding time zone
		startTime = switchTimeZone(af.LastFeeding.Time, time.Local)

		heuristicEndtime := af.FeedingEnd.Add((time.Duration(-1 * int(af.FeedingPeriod/2) * int(time.Minute))))
		heuristicStarttime := af.FeedingStart.Add(-1 * (time.Duration(int(af.FeedingPeriod) * int(time.Minute))))

		previousStarttime := heuristicStarttime.Add(-24 * time.Hour)
		previousEndtime := heuristicEndtime.Add(-24 * time.Hour)

		if startTime.After(heuristicEndtime) || startTime.Equal(heuristicEndtime) {
			startTime = af.FeedingStart.Add(24 * time.Hour)
		} else if startTime.Before(previousStarttime) {
			// Check if the last feeding was less than 24 hours ago
			if now.Sub(af.LastFeeding.Time) <= 24*time.Hour {
				// If less than 24 hours ago, show previous day's feeding
				startTime = af.FeedingStart.Add(-24 * time.Hour)
			} else {
				// If more than 24 hours ago, check if current time is close to feeding time
				feedingTimeToday := af.FeedingStart
				if now.Add(HIGHTIMELIMIT).After(feedingTimeToday) && now.Before(feedingTimeToday.Add(HIGHTIMELIMIT)) {
					// Current time is close to feeding time, show current day's feeding
					startTime = feedingTimeToday
				} else {
					// Current time is not close to feeding time, show previous day's feeding
					startTime = af.FeedingStart.Add(-24 * time.Hour)
				}
			}
		} else if startTime.Before(heuristicStarttime) && (startTime.After(previousEndtime) || startTime.Equal(previousEndtime)) {
			startTime = af.FeedingStart
		} else {
			startTime = startTime.Add(time.Minute * time.Duration(af.FeedingPeriod))
		}
	}

	// Debug log after calculating startTime
	//log.Printf("Calculated startTime: %v", startTime)

	// not in the future
	if startTime.Before(now.Add(HIGHTIMELIMIT)) {
		af.NextFeeding = nulls.NewTime(startTime)

		criticalLimit := now.Add(time.Duration(-1 * int(af.FeedingPeriod/2) * int(time.Minute)))

		if startTime.Before(criticalLimit) {
			af.NextFeedingCode = 0
		} else if startTime.Before(lowNearLimit) {
			af.NextFeedingCode = 1
		} else if startTime.Before(highNearLimit) {
			af.NextFeedingCode = 2
		} else {
			af.NextFeedingCode = 3
		}

		// Debug log after calculating NextFeeding and NextFeedingCode
		//log.Printf("Calculated NextFeeding: %v, NextFeedingCode: %d", af.NextFeeding, af.NextFeedingCode)
	} else {
		af.NextFeeding = nulls.Time{}

		// Debug log when NextFeeding is not set
		//log.Printf("NextFeeding not set as startTime is in the future")
	}

	// Debug log before returning the result
	//log.Printf("calculateFeeding returning: %+v", af)

	return af
}

func calculateFeedings(afRaw []AnimalFeeding) (FeedingByZoneMap, error) {
	// Set time
	// Fix start-end
	now := time.Now()
	afCalc := []AnimalFeeding{}

	// Calculate next feedings
	for _, af := range afRaw {
		af = calculateFeeding(af, now)
		if af.NextFeeding.Valid {
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

func generateAnimalFeedings(c buffalo.Context) (FeedingByZoneMap, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	// Retrieve info from the DB
	afRaw := []AnimalFeeding{}
	if err := tx.Eager().RawQuery(FEEDING_SQL).All(&afRaw); err != nil {
		return nil, err
	}

	// Calculate results
	return calculateFeedings(afRaw)
}

// FeedingFeeding default implementation.
func FeedingIndex(c buffalo.Context) error {
	zm, err := zonesMap(c)
	if err != nil {
		return err
	}
	c.Set("zoneMap", zm)

	af, err := generateAnimalFeedings(c)
	if err != nil {
		return err
	}
	c.Set("feedingByZone", af)
	c.Logger().Debugf("Feeding: %v", af)

	return c.Render(http.StatusOK, r.HTML("feeding/index.html"))
}

// FeedingFeeding default implementation.
func FeedingClose(c buffalo.Context) error {
	animalIDStr := c.Param("ID")
	timeToCloseSTR := c.Param("time")
	note := c.Param("note")

	care := &models.Care{}
	var err error
	if care.AnimalID, err = strconv.Atoi(animalIDStr); err != nil {
		return err
	}
	if care.Date, err = time.Parse(feeding_dateFormat, timeToCloseSTR); err != nil {
		return err
	}
	now := time.Now().Format(models.DateTimeFormat)
	if len(note) > 0 {
		note = fmt.Sprintf("%s - %s", now, note)
	} else {
		note = now
	}
	care.Note = nulls.NewString(note)

	// Set care type
	ct, err := caretypes(c)
	if err != nil {
		return err
	}

	// get feeding caretype
	for i := 0; i < len(*ct); i++ {
		c := (*ct)[i]
		if c.Type == models.CareTypeFeed {
			care.Type = c
			break
		}
	}
	c.Logger().Debugf("Closing feeding for animalID %d with date at %s", care.AnimalID, care.Date)

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	if err = tx.Create(care); err != nil {
		return err
	}

	c.Flash().Add("success", T.Translate(c, "feeding.close.success"))
	if len(c.Param("back")) > 0 {
		return c.Redirect(http.StatusSeeOther, c.Param("back"))
	}
	return c.Redirect(http.StatusSeeOther, "/feeding")
}
