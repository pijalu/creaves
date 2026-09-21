package actions

import (
	"crypto/sha1"
	"encoding/hex"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
)

func sha256(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// landingTabAnchor returns the landing-page tab anchor ("#t-<hash>") matching
// the grouping key (zone name or animal type name, "?" when unset) exactly as
// the landing template builds it (issue #199-9).
func landingTabAnchor(key string) string {
	if key == "" {
		key = "?"
	}
	return "#t-" + sha256(key)
}

// landingBackTarget resolves the "Back to animals in care" target (issue
// #199-9): honor a safe `back` param (landing passes its tab anchor through
// it), else default to the animal's zone tab in the default landing view.
func landingBackTarget(c buffalo.Context, animal *models.Animal) string {
	if c.Param("back") != "" {
		return safeBackParam(c)
	}
	return "/" + landingTabAnchor(animal.Zone.String)
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
