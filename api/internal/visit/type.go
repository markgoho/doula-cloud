package visit

import "time"

// The three Visit type values ADR-0015 names: prenatal before the
// Engagement's pregnancy ended, birth on that date, postpartum after.
// Exported so any future caller elsewhere in the BFF names the same
// values DeriveType returns rather than hand-copying string literals --
// the same reasoning engagement.OutcomeLiveBirth's own doc comment gives.
const (
	TypePrenatal   = "prenatal"
	TypeBirth      = "birth"
	TypePostpartum = "postpartum"
)

// dateLayout matches engagement.dateLayout: the plain YYYY-MM-DD shape
// every date-only column in the BFF is read and compared as (outcome.go,
// detail.go's due_date), so no timezone-bearing time.Time ever enters a
// date comparison by accident.
const dateLayout = "2006-01-02"

// DeriveType is ADR-0015's Visit-type rule, and the one place it is
// written: #281's Key Interfaces name a single derivation that "every
// reader calls" rather than reimplementing, and this is it. Nobody
// stores a Visit's type; every caller derives it fresh from the two
// facts that decide it.
//
// at is the instant this Visit is best known to happen: its own
// scheduled instant, or -- for a Visit nobody has scheduled yet -- when
// it was logged (createdAt), the closest thing to "when" the product has
// for it. Callers pass whichever a Visit actually carries; the coalesce
// is the caller's job, not this function's, since the two BFF surfaces
// that could call this already hold `scheduled_at` and `created_at`
// as separate columns.
//
// pregnancyEndedOn is the Engagement's own recorded pregnancy-end date
// (#293), as YYYY-MM-DD text straight off the `date` column -- nil (or
// empty) means the pregnancy has not ended as far as the product knows,
// so every Visit on that Engagement is prenatal. There is deliberately
// no birth-outcome parameter: postpartum care after a loss is still
// postpartum, and a bereavement Visit types as postpartum for the same
// reason any other Visit after the pivot does, with no branch on how the
// birth went.
//
// zone is the Practice's own timezone (#953, practices.timezone), and it
// is what makes "the same day" mean anything here: pregnancyEndedOn
// comes off a `date` column, which carries no zone at all, so the two
// sides of the comparison are only commensurable once somebody says
// which day the Visit's instant falls on. That somebody is the Practice.
// This used to be UTC, on the reasoning that it was the one zone every
// Practice agreed on implicitly -- but a zone behind UTC crosses
// midnight in the evening, so a Visit worked the same evening as the
// birth typed postpartum, which is not what the Doula who was there saw.
//
// Not the reader's own zone, and not a zone on the Visit. One function
// computes this on every read and no two surfaces may disagree
// (CONTEXT.md, Visit), which a reader-local zone breaks outright: two
// Staff in two zones would read two types for one Visit. A per-Visit
// zone belongs to the richer time model parked on #330.
//
// zone is required, and a nil one panics at In rather than quietly
// standing in UTC for it. Standing in UTC is the exact answer ADR-0036
// forbids -- the wrong day, with nothing said about it -- so a caller
// that has no zone is a programming error and should read as one.
func DeriveType(at time.Time, pregnancyEndedOn *string, zone *time.Location) string {
	if pregnancyEndedOn == nil || *pregnancyEndedOn == "" {
		return TypePrenatal
	}
	atDate := at.In(zone).Format(dateLayout)
	switch {
	case atDate < *pregnancyEndedOn:
		return TypePrenatal
	case atDate == *pregnancyEndedOn:
		return TypeBirth
	default:
		return TypePostpartum
	}
}
