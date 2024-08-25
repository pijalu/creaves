package main

import (
	"fmt"
	"time"
)

// calculateNextMealTime calculates the next meal time based on the provided data.
func calculateNextMealTime(startTime, endTime, mealFrequency, previousMealTime, currentTime string) (string, error) {
	const timeFormat = "2006-01-02 15:04" // Date and 24-hour time format

	// Parse the input times into time.Time objects
	current, err := time.Parse(timeFormat, currentTime)
	if err != nil {
		return "", err
	}

	// Parse start and end times with the date from current time
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		return "", err
	}
	end, err := time.Parse("15:04", endTime)
	if err != nil {
		return "", err
	}

	// Set start and end to the same date as current time
	start = time.Date(current.Year(), current.Month(), current.Day(), start.Hour(), start.Minute(), 0, 0, current.Location())
	end = time.Date(current.Year(), current.Month(), current.Day(), end.Hour(), end.Minute(), 0, 0, current.Location())

	// Convert meal frequency from string to time.Duration (in minutes)
	frequency, err := time.ParseDuration(mealFrequency + "m")
	if err != nil {
		return "", err
	}

	var previous time.Time
	if previousMealTime != "" {
		previous, err = time.Parse(timeFormat, previousMealTime)
		if err != nil {
			return "", err
		}
	}

	// Initialize the next meal time
	var nextMealTime time.Time

	if previousMealTime == "" {
		// If there's no previous meal time
		nextMealTime = start
	} else {
		nextMealTime = previous
		// If the calculated next meal time is before the start time, set it to start time
		lastEndtime := end.Add(-24 * time.Hour)
		if nextMealTime.After(end) || nextMealTime.Equal(end) {
			nextMealTime = start.Add(24 * time.Hour)
		} else if nextMealTime.Before(start) && (nextMealTime.After(lastEndtime) || nextMealTime.Equal(lastEndtime)) {
			nextMealTime = start
		} else {
			nextMealTime = previous.Add(frequency)
		}
	}

	return nextMealTime.Format(timeFormat), nil
}

func main() {
	startTime := "08:00"
	endTime := "20:00"
	mealFrequency := "30" // in minutes
	previousMealTime := "2024-08-22 20:45"
	currentTime := "2024-08-22 21:00"

	nextMealTime, err := calculateNextMealTime(startTime, endTime, mealFrequency, previousMealTime, currentTime)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Next meal time:", nextMealTime)
	}
}
