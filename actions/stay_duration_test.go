package actions

import (
	"testing"

	"github.com/gobuffalo/nulls"
)

// TestStayDurationHours: human-friendly stay duration — plain hours up to
// 48h, days-hours above; NULL renders empty.
func TestStayDurationHours(t *testing.T) {
	cases := []struct {
		name  string
		hours nulls.Int
		day   string
		hour  string
		want  string
	}{
		{"invalid renders empty", nulls.Int{}, "d", "h", ""},
		{"zero", nulls.NewInt(0), "d", "h", "0 h"},
		{"one hour", nulls.NewInt(1), "d", "h", "1 h"},
		{"47h stays hours", nulls.NewInt(47), "d", "h", "47 h"},
		{"48h stays hours", nulls.NewInt(48), "d", "h", "48 h"},
		{"49h switches to days", nulls.NewInt(49), "d", "h", "2 d - 1 h"},
		{"whole days", nulls.NewInt(72), "d", "h", "3 d - 0 h"},
		{"123h example", nulls.NewInt(123), "d", "h", "5 d - 3 h"},
		{"french day unit", nulls.NewInt(123), "j", "h", "5 j - 3 h"},
		{"german day unit", nulls.NewInt(50), "T", "h", "2 T - 2 h"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stayDurationHours(tc.hours, tc.day, tc.hour); got != tc.want {
				t.Errorf("stayDurationHours(%v, %q, %q) = %q, want %q",
					tc.hours, tc.day, tc.hour, got, tc.want)
			}
		})
	}
}
