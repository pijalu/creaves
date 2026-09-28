package actions

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
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

// landingBackLabelKey picks the back-button label i18n key for a resolved
// target (bugs.md U16): the label must say where the button actually goes —
// "Back to the day plan" when the user came from /care_plan — instead of a
// static "Back to animals in care" that lies about the destination.
func landingBackLabelKey(target string) string {
	switch {
	case strings.HasPrefix(target, "/care_plan"):
		return "animals.back.to_day_plan"
	case strings.HasPrefix(target, "/reports/care_schedule"):
		return "animals.back.to_care_schedule"
	default:
		return "animals.back.to_in_care"
	}
}

// timeToNullTime parses the legacy "15:04" feeding-time format. Legacy
// feeding columns are frozen read-only (bugs.md H3) — kept for the guest
// page conversion semantics and its tests.
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
