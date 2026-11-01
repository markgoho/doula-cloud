package main

import "time"

const longDateLayout = "January 2, 2006"

var businessZone = mustLoadZone("America/New_York")

func mustLoadZone(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		// coverage:ignore reason: the zone is embedded by time/tzdata, so it always loads
		panic(err)
	}
	return loc
}

func businessDay(at time.Time) string {
	return at.In(businessZone).Format(longDateLayout)
}

func businessToday(now time.Time) time.Time {
	now = now.In(businessZone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, businessZone)
}
