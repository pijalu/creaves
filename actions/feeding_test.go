package actions

import (
	"strconv"
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
)

const ft_dateFormat = "2006-01-02 15:04" // Date and 24-hour time format
const ft_timeFormat = "15:04"

var feedingTests = []struct {
	startTime        string
	endTime          string
	mealFrequency    string
	previousMealTime string
	currentTime      string
	expectedResult   string
}{
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "",
		currentTime:      "2024-08-23 09:00",
		expectedResult:   "2024-08-23 08:00", // First meal of the day (no feeding)
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "",
		currentTime:      "2024-08-23 19:00",
		expectedResult:   "2024-08-23 08:00", // First meal of the day (no feeding)
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 16:59",
		currentTime:      "2024-08-23 09:00",
		expectedResult:   "2024-08-22 18:59", // last feeding+freq as last feeding too early to finish day
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 08:00",
		currentTime:      "2024-08-22 11:00",
		expectedResult:   "2024-08-22 10:00",
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 16:30",
		currentTime:      "2024-08-22 17:05",
		expectedResult:   "2024-08-22 18:30",
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 17:00",
		currentTime:      "2024-08-22 17:05",
		expectedResult:   "", // Too much in future "2024-08-23 08:00",
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 18:00",
		currentTime:      "2024-08-23 09:00",
		expectedResult:   "2024-08-23 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "18:00",
		mealFrequency:    "120",
		previousMealTime: "2024-08-22 21:00",
		currentTime:      "2024-08-23 09:00",
		expectedResult:   "2024-08-23 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "20:00",
		mealFrequency:    "30",
		previousMealTime: "2024-08-23 20:45",
		currentTime:      "2024-08-23 21:00",
		expectedResult:   "", // Too much in the future: "2024-08-24 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "20:00",
		mealFrequency:    "30",
		previousMealTime: "2024-08-23 20:45",
		currentTime:      "2024-08-24 07:00",
		expectedResult:   "2024-08-24 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "23:59",
		mealFrequency:    "180",
		previousMealTime: "2024-08-23 23:00",
		currentTime:      "2024-08-23 23:05",
		expectedResult:   "", // Too much in the future "2024-08-24 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "23:59",
		mealFrequency:    "180",
		previousMealTime: "2024-08-23 23:00",
		currentTime:      "2024-08-24 07:05",
		expectedResult:   "2024-08-24 08:00", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "23:59",
		mealFrequency:    "60",
		previousMealTime: "2024-08-24 07:45",
		currentTime:      "2024-08-24 08:05",
		expectedResult:   "2024-08-24 08:45", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "23:59",
		mealFrequency:    "60",
		previousMealTime: "2024-08-24 07:25",
		currentTime:      "2024-08-24 07:30",
		expectedResult:   "2024-08-24 08:25", // First meal of the day
	},
	{
		startTime:        "08:00",
		endTime:          "23:59",
		mealFrequency:    "60",
		previousMealTime: "2024-08-24 06:59",
		currentTime:      "2024-08-24 07:30",
		expectedResult:   "2024-08-24 08:00", // First meal of the day
	},
}

func TestCalculateFeeding(t *testing.T) {
	pt := func(s string) time.Time {
		res, err := time.ParseInLocation(ft_timeFormat, s, time.Local)
		if err != nil {
			t.Errorf("Could not convert test data to time: %s", s)
		}
		return res
	}
	pd := func(s string) time.Time {
		res, err := time.ParseInLocation(ft_dateFormat, s, time.Local)
		if err != nil {
			t.Errorf("Could not convert test data to date: %s", s)
		}
		return res
	}
	pf := func(s string) int {
		res, err := strconv.Atoi(s)
		if err != nil {
			t.Errorf("Could not convert test data to int: %s", s)
		}
		return res
	}

	pdu := func(s string) time.Time {
		res, err := time.Parse(ft_dateFormat, s)
		if err != nil {
			t.Errorf("Could not convert test data to date: %s", s)
		}
		return res
	}

	_ = pdu

	for _, test := range feedingTests {
		now := pd(test.currentTime)
		exp := nulls.Time{}
		if test.expectedResult != "" {
			exp = nulls.NewTime(pd(test.expectedResult))
		}

		af := AnimalFeeding{
			FeedingStart:  pt(test.startTime),
			FeedingEnd:    pt(test.endTime),
			FeedingPeriod: pf(test.mealFrequency),
		}
		if test.previousMealTime == "" {
			af.LastFeeding = nulls.Time{}
		} else {
			// time in db are stored utc
			af.LastFeeding = nulls.NewTime(pd(test.previousMealTime))
		}

		result := calculateFeeding(af, now)
		if exp.Valid != result.NextFeeding.Valid || !exp.Time.Equal(result.NextFeeding.Time) {
			t.Errorf("For start: %s, end: %s, frequency: %s, previous: %s, current: %s, expected: %v, got: %v",
				test.startTime, test.endTime, test.mealFrequency, test.previousMealTime, test.currentTime, exp, result.NextFeeding)
		}

	}
}
