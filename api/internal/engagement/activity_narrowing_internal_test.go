package engagement

import (
	"database/sql"
	"testing"
)

// TestEntryDetail_NarrowingNamesTheDoulaAndTheDays pins #2991's sentence:
// two narrowing entries on one Engagement are told apart by whose days
// changed and what they are now.
func TestEntryDetail_NarrowingNamesTheDoulaAndTheDays(t *testing.T) {
	bethany := sql.NullString{String: "Bethany Kroll", Valid: true}
	cases := []struct {
		name  string
		diff  string
		doula sql.NullString
		want  string
	}{
		{"both ends", `{"before":{"from":null,"to":null},"after":{"from":"2026-11-20","to":"2026-12-04"}}`, bethany, "Doulas on call changed: Bethany Kroll, Nov 20, 2026 to Dec 4, 2026"},
		{"a start only", `{"after":{"from":"2026-11-20","to":null}}`, bethany, "Doulas on call changed: Bethany Kroll, from Nov 20, 2026"},
		{"an end only", `{"after":{"from":null,"to":"2026-12-04"}}`, bethany, "Doulas on call changed: Bethany Kroll, through Dec 4, 2026"},
		{"cleared", `{"before":{"from":"2026-11-20","to":null},"after":{"from":null,"to":null}}`, bethany, "Doulas on call changed: Bethany Kroll, the whole window"},
		{"a day that is not one", `{"after":{"from":"soon","to":null}}`, bethany, "Doulas on call changed: Bethany Kroll, the whole window"},
		{"a Doula who has gone", `{"after":{"from":null,"to":null}}`, sql.NullString{}, "Doulas on call changed: a former colleague, the whole window"},
		{unreadableDiff, notJSON, bethany, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entryDetail("on_call_narrowing_changed", []byte(tc.diff), sql.NullString{}, tc.doula); got != tc.want {
				t.Fatalf("detail = %q, want %q", got, tc.want)
			}
		})
	}
}
