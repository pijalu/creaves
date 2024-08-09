package models

import (
	"time"

	"github.com/gobuffalo/pop/v6"
)

// DateTimeFormat is dateformat
const DateTimeFormat = "2006/01/02 15:04"
const DateFormat = "2006/01/02"

// NowOffset return the time in UTC... with offset to match current time (bug in Pop.... using UTC as default parse...)
func NowOffset() time.Time {
	// Get the current local time
	localNow := time.Now()

	_, sec := localNow.Zone()

	// Get the current time in UTC
	utcNow := localNow.UTC()

	// Adjust the input timestamp by the precalculated timeOffset
	oftime := utcNow.Add(time.Duration(sec) * time.Second)

	return oftime
}

func init() {
	pop.SetNowFunc(func() time.Time {
		return NowOffset()
	})
}
