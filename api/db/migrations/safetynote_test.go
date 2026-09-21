package migrations

import "testing"

// TestParseSafetyNoteScopesToTheQuotedStatement proves the bug #1251
// reports is fixed: a heading with one quoted statement does not excuse
// a second, unquoted statement of the same class.
func TestParseSafetyNoteScopesToTheQuotedStatement(t *testing.T) {
	note := "# 00999_example.sql\n\n" +
		"## CREATE UNIQUE INDEX\n\n" +
		"```sql\n" +
		"CREATE UNIQUE INDEX a_key ON a (x)\n" +
		"```\n\n" +
		"No duplicate can exist because x is new.\n"

	quoted := parseSafetyNote(note)

	proven := normalizeStatement("CREATE UNIQUE INDEX a_key ON a (x)")
	unproven := normalizeStatement("CREATE UNIQUE INDEX b_key ON b (y)")

	if !statementQuoted(quoted[classCreateUniqueIndex], proven) {
		t.Errorf("parseSafetyNote(%q)[%q] = %v, want it to contain the quoted statement", note, classCreateUniqueIndex, quoted[classCreateUniqueIndex])
	}
	if statementQuoted(quoted[classCreateUniqueIndex], unproven) {
		t.Errorf("parseSafetyNote(%q)[%q] = %v, want it to NOT contain a statement the note never quotes -- a note must excuse only the statement it quotes", note, classCreateUniqueIndex, quoted[classCreateUniqueIndex])
	}
}

// TestParseSafetyNoteKeepsAnInlineCommentOnANonFinalLine proves a block
// carries its newlines into normalizeStatement, so an inline -- comment
// in the middle of a multi-line block doesn't swallow the line after
// it. Joining block lines with a space instead of a newline would make
// SplitStatements' line-comment scan run to the end of the block.
func TestParseSafetyNoteKeepsAnInlineCommentOnANonFinalLine(t *testing.T) {
	note := "## CREATE UNIQUE INDEX\n\n" +
		"```sql\n" +
		"CREATE UNIQUE INDEX a_key -- the key\n" +
		"    ON a (x)\n" +
		"```\n"

	quoted := parseSafetyNote(note)
	want := normalizeStatement("CREATE UNIQUE INDEX a_key ON a (x)")

	if !statementQuoted(quoted[classCreateUniqueIndex], want) {
		t.Errorf("parseSafetyNote(%q)[%q] = %v, want it to contain %q -- an inline comment on a non-final line must not swallow the rest of the block", note, classCreateUniqueIndex, quoted[classCreateUniqueIndex], want)
	}
}

// TestParseSafetyNoteIgnoresABlockOutsideAnyHeading proves a fenced
// block before the first "## " heading cannot excuse anything: it is
// keyed to the empty class, which RowDependent never reports.
func TestParseSafetyNoteIgnoresABlockOutsideAnyHeading(t *testing.T) {
	note := "# 00999_example.sql\n\n" +
		"```sql\n" +
		"CREATE UNIQUE INDEX a_key ON a (x)\n" +
		"```\n\n" +
		"## CREATE UNIQUE INDEX\n\n" +
		"Nothing quoted here proves this heading.\n"

	quoted := parseSafetyNote(note)

	if got := quoted[classCreateUniqueIndex]; len(got) != 0 {
		t.Errorf("parseSafetyNote(%q)[%q] = %v, want none -- a block before any heading excuses nothing", note, classCreateUniqueIndex, got)
	}
	if got := quoted[""]; len(got) != 1 {
		t.Errorf("parseSafetyNote(%q)[\"\"] = %v, want the one block that arrived before a heading", note, got)
	}
}

// TestNormalizeStatementMatchesRegardlessOfFormatting proves
// normalizeStatement puts a hand-copied block through the same pipeline
// Finding.Statement is built with, so case, whitespace, a trailing
// semicolon and an inline comment never break a match a human would
// consider exact.
func TestNormalizeStatementMatchesRegardlessOfFormatting(t *testing.T) {
	canonical := normalizeStatement("CREATE UNIQUE INDEX a_key ON a (x)")

	variants := []string{
		"create unique index a_key on a (x)",
		"CREATE UNIQUE INDEX a_key ON a (x);",
		"CREATE UNIQUE INDEX a_key\n    ON a (x)",
		"CREATE UNIQUE INDEX a_key ON a (x) -- no duplicate can exist\n",
	}
	for _, v := range variants {
		if got := normalizeStatement(v); got != canonical {
			t.Errorf("normalizeStatement(%q) = %q, want %q", v, got, canonical)
		}
	}
}

// TestNormalizeStatementEmpty proves an empty or comment-only block
// normalizes to "" rather than panicking -- SplitStatements can return
// no statement at all.
func TestNormalizeStatementEmpty(t *testing.T) {
	if got := normalizeStatement("-- just a comment\n"); got != "" {
		t.Errorf("normalizeStatement(comment-only) = %q, want empty", got)
	}
}

// TestClassNamedScopesToTheNamedClass proves a grandfathered entry
// exempts only the class its classes list names -- a class the same
// migration raises that is not in the list is not excused by it.
func TestClassNamedScopesToTheNamedClass(t *testing.T) {
	classes := []string{classAddConstraintCheck}
	if !classNamed(classes, classAddConstraintCheck) {
		t.Errorf("classNamed(%v, %q) = false, want true", classes, classAddConstraintCheck)
	}
	if classNamed(classes, classCreateUniqueIndex) {
		t.Errorf("classNamed(%v, %q) = true, want false -- an entry exempts only the class it names", classes, classCreateUniqueIndex)
	}
}
