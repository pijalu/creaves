package main

import (
	"fmt"
	"time"

	"github.com/gobuffalo/nulls"
)

type FeedingSchedule struct {
	feedingStart time.Time
	feedingEnd   time.Time
	Period       int

	feedings    []time.Time
	LastFeeding nulls.Time
}

/*
SELECT a.id, a.feeding_start, a.feeding_end, a.feeding_period, MAX(c.date) AS last_care_time
FROM animals a
LEFT JOIN cares c ON (a.id = c.animal_id and c.type_id in (select id from caretypes where type=1))
WHERE a.outtake_id IS NULL
AND a.feeding_period > 0
GROUP BY a.id;
*/

func main() {
	fmt.Printf("Testing stuff\n")

	// Fix start-end
	now := time.Now()
	nowOneHourAgo := now.Add(-1 * time.Hour)
	nowOneHourLater := now.Add(1 * time.Hour)

	fs := FeedingSchedule{
		feedingStart: time.Date(1, 1, 1, 7, 0, 0, 0, time.Local),
		feedingEnd:   time.Date(1, 1, 1, 17, 0, 0, 0, time.Local),
		Period:       15,
		//LastFeeding:  nulls.Time{},
		LastFeeding: nulls.NewTime(time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())),
	}

	fs.feedingStart = time.Date(now.Year(), now.Month(), now.Day(), fs.feedingStart.Hour(), fs.feedingStart.Minute(), 0, 0, fs.feedingStart.Location())
	fs.feedingEnd = time.Date(now.Year(), now.Month(), now.Day(), fs.feedingEnd.Hour(), fs.feedingEnd.Minute(), 0, 0, fs.feedingEnd.Location())
	if fs.LastFeeding.Valid && fs.LastFeeding.Time.After(fs.feedingStart) {
		fs.feedingStart = fs.LastFeeding.Time.Add(time.Minute * time.Duration(fs.Period))
	}

	startTime := fs.feedingStart
	for startTime.Before(fs.feedingEnd) && startTime.Before(nowOneHourAgo) {
		startTime = startTime.Add(time.Minute * time.Duration(fs.Period))
	}
	for startTime.Before(fs.feedingEnd) && startTime.Before(nowOneHourLater) {
		fs.feedings = append(fs.feedings, startTime)
		break
		//startTime = startTime.Add(time.Minute * time.Duration(fs.Period))
	}

	fmt.Printf("Time Window: %s - %s\n", nowOneHourAgo.Format("15:04"), nowOneHourLater.Format("15:04"))
	fmt.Printf("Feeding schedule:\n")
	for _, t := range fs.feedings {
		fmt.Printf("%s\n", t.Format("15:04"))
	}
}
