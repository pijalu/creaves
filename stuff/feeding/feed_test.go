package main

import (
	"testing"
)

func TestCalculateNextMealTime(t *testing.T) {
	tests := []struct {
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
			expectedResult:   "2024-08-23 08:00", // First meal of the day
		},
		{
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 19:00",
			expectedResult:   "2024-08-23 08:00", // First meal of the day
		},
		{
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-22 17:00",
			currentTime:      "2024-08-23 09:00",
			expectedResult:   "2024-08-22 19:00", // previous day meal
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
			previousMealTime: "2024-08-22 17:00",
			currentTime:      "2024-08-22 17:05",
			expectedResult:   "2024-08-22 19:00",
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
			expectedResult:   "2024-08-24 08:00", // First meal of the day
		},
	}

	for _, test := range tests {
		result, err := calculateNextMealTime(test.startTime, test.endTime, test.mealFrequency, test.previousMealTime, test.currentTime)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != test.expectedResult {
			t.Errorf("For start: %s, end: %s, frequency: %s, previous: %s, current: %s, expected: %s, got: %s",
				test.startTime, test.endTime, test.mealFrequency, test.previousMealTime, test.currentTime, test.expectedResult, result)
		}
	}
}
