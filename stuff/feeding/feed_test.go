package main

import (
	"testing"
)

func TestCalculateNextMealTime(t *testing.T) {
	tests := []struct {
		name             string
		startTime        string
		endTime          string
		mealFrequency    string
		previousMealTime string
		currentTime      string
		expectedResult   string
		expectError      bool
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
			expectedResult:   "2024-08-23 08:00",
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

		// --- Error cases: invalid / empty time formats and frequency ---

		{
			name:             "invalid_current_time_format",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23",
			expectError:      true,
		},
		{
			name:             "empty_current_time",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "",
			expectError:      true,
		},
		{
			name:             "invalid_current_time_garbage",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "not-a-datetime",
			expectError:      true,
		},
		{
			name:             "invalid_start_time_format",
			startTime:        "invalid",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "empty_start_time",
			startTime:        "",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "out_of_range_start_time",
			startTime:        "25:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "invalid_end_time_format",
			startTime:        "08:00",
			endTime:          "not-a-time",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "empty_end_time",
			startTime:        "08:00",
			endTime:          "",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "invalid_frequency_non_numeric",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "abc",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "empty_frequency",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "invalid_previous_meal_time_format",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-23",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},
		{
			name:             "invalid_previous_meal_time_garbage",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "garbage",
			currentTime:      "2024-08-23 09:00",
			expectError:      true,
		},

		// --- Boundary: start == end ---

		{
			name:             "start_equals_end_no_previous",
			startTime:        "08:00",
			endTime:          "08:00",
			mealFrequency:    "120",
			previousMealTime: "",
			currentTime:      "2024-08-23 09:00",
			expectedResult:   "2024-08-23 08:00", // window collapses; no previous -> start
		},
		{
			name:             "start_equals_end_with_previous_in_window",
			startTime:        "08:00",
			endTime:          "08:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-23 07:00",
			currentTime:      "2024-08-23 07:30",
			// heuristicEnd = end - freq/2 = 08:00 - 60m = 07:00; previous (07:00) == heuristicEnd -> tomorrow start
			expectedResult:   "2024-08-24 08:00",
		},

		// --- Different frequency values (branch C: previous + frequency) ---

		{
			name:             "frequency_240_branch_c",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "240",
			previousMealTime: "2024-08-22 10:00",
			currentTime:      "2024-08-22 12:00",
			// heuristicEnd = 18:00 - 120m = 16:00; previous 10:00 < 16:00 and >= start -> previous+freq
			expectedResult:   "2024-08-22 14:00",
		},
		{
			name:             "frequency_30_branch_c",
			startTime:        "06:00",
			endTime:          "22:00",
			mealFrequency:    "30",
			previousMealTime: "2024-08-23 07:00",
			currentTime:      "2024-08-23 07:10",
			// heuristicEnd = 22:00 - 15m = 21:45; previous 07:00 in window -> previous+freq
			expectedResult:   "2024-08-23 07:30",
		},

		// --- Branch boundaries for previous-meal logic ---

		{
			name:             "branch_b_equal_last_endtime_boundary",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-22 17:00",
			currentTime:      "2024-08-23 09:00",
			// lastEndtime = heuristicEnd - 24h = (08-23 17:00) - 24h = 08-22 17:00; previous exactly equal -> today start
			expectedResult:   "2024-08-23 08:00",
		},
		{
			name:             "branch_a_strictly_after_heuristic_end",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-22 17:01",
			currentTime:      "2024-08-22 17:10",
			// heuristicEnd = 17:00; previous 17:01 strictly after -> tomorrow start
			expectedResult:   "2024-08-23 08:00",
		},
		{
			name:             "previous_exactly_at_start_today_branch_c",
			startTime:        "08:00",
			endTime:          "18:00",
			mealFrequency:    "120",
			previousMealTime: "2024-08-22 08:00",
			currentTime:      "2024-08-22 08:00",
			// previous == start (Before is strict, so not branch B) -> previous+freq
			expectedResult:   "2024-08-22 10:00",
		},
	}

	for _, test := range tests {
		result, err := calculateNextMealTime(test.startTime, test.endTime, test.mealFrequency, test.previousMealTime, test.currentTime)

		label := test.name
		if label == "" {
			label = "unnamed"
		}

		if test.expectError {
			if err == nil {
				t.Errorf("%s: expected an error but got none (result: %s)", label, result)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", label, err)
			continue
		}
		if result != test.expectedResult {
			t.Errorf("%s: for start: %s, end: %s, frequency: %s, previous: %s, current: %s, expected: %s, got: %s",
				label, test.startTime, test.endTime, test.mealFrequency, test.previousMealTime, test.currentTime, test.expectedResult, result)
		}
	}
}
