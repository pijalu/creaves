package actions

import (
	"crypto/sha1"
	"encoding/hex"
	"time"

	"github.com/gobuffalo/nulls"
)

func sha256(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func timeToNullTime(s string) nulls.Time {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return nulls.Time{}
	}
	return nulls.NewTime(time.Date(1, 1, 1, t.Hour(), t.Minute(), 0, 1, time.UTC))
}

func timeToMinutes(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}

func switchTimeZone(t time.Time, l *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), l)
}
