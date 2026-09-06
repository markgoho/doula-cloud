package activityfeed

import "doula-cloud/api/internal/staffauth"

// Mount registers the Practice-wide feed read (#486). AnyStaff, the same
// as engagement.ListActivityHandler -- the gate applied per row (each
// subject kind's own visibility), not this mount, decides what a reader
// actually sees.
func Mount(g *staffauth.GatedRouter) {
	g.Get("/api/practices/{practiceId}/activity", staffauth.AnyStaff, PracticeHandler())
}
