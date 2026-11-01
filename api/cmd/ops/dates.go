package main

import "time"

// longDateLayout is how ops writes a day a person reads: "January 2, 2006".
const longDateLayout = "January 2, 2006"

// businessZone is where the business keeps its books, so a month and a
// day start at midnight there and not at midnight UTC.
var businessZone = mustLoadZone("America/New_York")

func mustLoadZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// coverage:ignore reason: the zone is embedded by time/tzdata, so it always loads
		panic(err)
	}
	return loc
}

// businessDay writes the New York day an instant falls on.
func businessDay(at time.Time) string {
	return at.In(businessZone).Format(longDateLayout)
}

// businessToday is midnight, New York time, of the day now falls on.
func businessToday(now time.Time) time.Time {
	now = now.In(businessZone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, businessZone)
}
