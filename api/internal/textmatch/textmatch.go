// Package textmatch is the typed-text search two lists share: the Clients
// list's Name field and the Invoices list's "Reference or Client" field
// (#1829, #2746). Both match what a person typed anywhere in a value,
// case-insensitively, through ILIKE with a bound pattern, never through text
// spliced into the SQL.
package textmatch

import (
	"fmt"
	"strings"
)

// likeEscaper escapes the three characters LIKE reads as more than
// themselves, so text typed with "%" or "_" matches only itself.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Contains is the ILIKE pattern that finds text anywhere in a value, and ""
// for no text, which ClientNameExpr reads as "no narrowing".
func Contains(text string) string {
	if text == "" {
		return ""
	}
	return "%" + likeEscaper.Replace(text) + "%"
}

// ClientNameExpr is the condition that a pattern from Contains, bound as
// $param, matches a Client's given, preferred or family name, or her full
// name as a list reads it (personname.Full) or as her record spells it.
// alias is the clients table's alias in the caller's query.
func ClientNameExpr(alias string, param int) string {
	return fmt.Sprintf(`(
	%[1]s.given_name ILIKE $%[2]d OR %[1]s.preferred_name ILIKE $%[2]d OR %[1]s.family_name ILIKE $%[2]d
	OR concat_ws(' ', COALESCE(NULLIF(%[1]s.preferred_name, ''), %[1]s.given_name), %[1]s.family_name) ILIKE $%[2]d
	OR concat_ws(' ', %[1]s.given_name, %[1]s.family_name) ILIKE $%[2]d
)`, alias, param)
}
