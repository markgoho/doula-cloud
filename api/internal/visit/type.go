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
// Both instants are compared as calendar days in UTC. Doula Cloud has no
// Practice timezone today (grepped: no timezone column, no per-Practice
// setting anywhere in the schema), so UTC is the one zone every Practice
// already agrees on implicitly. This can misplace a birth logged late in
// the evening in a zone behind UTC across midnight into the following
// UTC day -- filed as #281's own follow-up rather than solved here,
// since a Practice timezone is a new fact plus a settings screen, not
// this derivation's job.
func DeriveType(at time.Time, pregnancyEndedOn *string) string {
	if pregnancyEndedOn == nil || *pregnancyEndedOn == "" {
		return TypePrenatal
	}
	atDate := at.UTC().Format(dateLayout)
	switch {
	case atDate < *pregnancyEndedOn:
		return TypePrenatal
	case atDate == *pregnancyEndedOn:
		return TypeBirth
	default:
		return TypePostpartum
	}
}
