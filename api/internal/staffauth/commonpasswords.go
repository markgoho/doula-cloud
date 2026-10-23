package staffauth

import (
	_ "embed"
	"strings"
)

// commonPasswordsFile is the list of commonly used passwords a Staff
// password is refused for (#1539, ADR-0026's amendment for #1493).
// `bun run --cwd app sync-common-passwords` writes it, and the same bytes
// to app/src/lib/commonPasswords.txt, which the browser checks on the three
// screens that set a password; app/src/lib/commonPasswords.spec.ts fails the
// build if the two differ. Never edit it by hand. Every entry is lowercased
// printable ASCII of 15 characters or more.
//
//go:embed commonpasswords.txt
var commonPasswordsFile string

var commonPasswords = func() map[string]struct{} {
	// The file ends in a newline; the empty line after it is not an entry.
	entries := strings.Split(strings.TrimSuffix(commonPasswordsFile, "\n"), "\n")
	set := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		set[entry] = struct{}{}
	}
	return set
}()

// isPasswordTooCommon reports whether the whole password, in any letter
// case, is on the list. The reset spend is the only BFF path that carries a
// Staff password, so it is the only caller: on signup and invitation
// acceptance the browser gives the password to Identity Platform directly,
// and the browser's own check is the only one there is.
func isPasswordTooCommon(password string) bool {
	_, found := commonPasswords[strings.ToLower(password)]
	return found
}
