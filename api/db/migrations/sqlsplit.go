package migrations

import "strings"

const (
	upMarker   = "+goose Up"
	downMarker = "+goose Down"
)

// UpSection returns everything between the goose Up annotation and the
// Down annotation, or the whole body when there is no Down. Only the Up
// section can meet rows that are already there; a Down runs against
// whatever its own Up left behind and never runs on trunk at all.
func UpSection(body string) string {
	// Cutting on the marker also steps past it, so the first statement
	// does not arrive with "+goose Up" glued to its front -- a statement
	// recognized by what it starts with would never match.
	_, rest, found := strings.Cut(body, upMarker)
	if !found {
		return body
	}
	up, _, found := strings.Cut(rest, downMarker)
	if found {
		return up
	}
	return rest
}

// SplitStatements breaks a section of SQL into its top-level statements,
// dropping line comments (goose's own `-- +goose ...` annotations
// included) and keeping single-quoted strings and dollar-quoted bodies
// intact -- a plpgsql function body is full of semicolons that do not
// end the CREATE FUNCTION containing it.
func SplitStatements(sql string) []string {
	var out []string
	var cur strings.Builder

	for i := 0; i < len(sql); {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			i += skipLineComment(sql[i:])
		case sql[i] == '\'':
			n := skipQuoted(sql[i:])
			cur.WriteString(sql[i : i+n])
			i += n
		case dollarTag(sql[i:]) != "":
			n := skipDollarQuoted(sql[i:], dollarTag(sql[i:]))
			cur.WriteString(sql[i : i+n])
			i += n
		case sql[i] == ';':
			out = appendStatement(out, cur.String())
			cur.Reset()
			i++
		default:
			cur.WriteByte(sql[i])
			i++
		}
	}
	return appendStatement(out, cur.String())
}

// appendStatement adds a statement unless it is blank -- the text
// between two semicolons is often nothing but the comment that was just
// stripped.
func appendStatement(out []string, stmt string) []string {
	if strings.TrimSpace(stmt) == "" {
		return out
	}
	return append(out, stmt)
}

// skipLineComment returns the length of the comment starting at s,
// including its newline, so the newline still separates the tokens
// around it.
func skipLineComment(s string) int {
	if end := strings.IndexByte(s, '\n'); end >= 0 {
		return end
	}
	return len(s)
}

// skipQuoted returns the length of the single-quoted string starting at
// s, treating a doubled quote as an escape rather than a close.
func skipQuoted(s string) int {
	for i := 1; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '\'' {
			i++
			continue
		}
		return i + 1
	}
	return len(s)
}

// dollarTag returns the dollar-quote opener at the start of s ("$$",
// "$body$"), or "" when s does not start with one.
func dollarTag(s string) string {
	if len(s) == 0 || s[0] != '$' {
		return ""
	}
	for i := 1; i < len(s); i++ {
		if s[i] == '$' {
			return s[:i+1]
		}
		if !isTagByte(s[i]) {
			return ""
		}
	}
	return ""
}

// isTagByte reports whether c may appear in a dollar-quote tag.
func isTagByte(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}

// skipDollarQuoted returns the length of the dollar-quoted body opened
// by tag at the start of s, including both delimiters.
func skipDollarQuoted(s, tag string) int {
	if end := strings.Index(s[len(tag):], tag); end >= 0 {
		return len(tag) + end + len(tag)
	}
	return len(s)
}
