package outbox_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"doula-cloud/api/internal/outbox"
)

// statusWriteAllowed names the one Go file outside this package that may
// write an outbox table's status column itself, and why. Anything else
// that does is a transition this package's declaration never sees: the
// login deletion's hand-written "mark sent" was one, and it skipped the
// exit duty that clears a plaintext token.
var statusWriteAllowed = map[string]string{
	"internal/sitebuild/outbox.go": "site_build_outbox is its own machine, declared at outbox.SiteBuildStatus: one dispatch for every pending row at once, written set-based",
}

// outboxUpdate finds an UPDATE of an outbox table and captures its SET
// list, up to the clause that ends it.
var outboxUpdate = regexp.MustCompile(`(?is)\bUPDATE\s+\w+_outbox\b(?:\s+(?:AS\s+)?\w+)?\s+SET\b(.*?)(?:\bWHERE\b|\bFROM\b|\bRETURNING\b|$)`)

// outboxUpsert finds an INSERT into an outbox table whose conflict arm
// updates, and captures that arm's SET list.
var outboxUpsert = regexp.MustCompile(`(?is)\bINSERT\s+INTO\s+\w+_outbox\b.*?\bDO\s+UPDATE\s+SET\b(.*?)(?:\bWHERE\b|\bRETURNING\b|$)`)

// setsStatus is a SET list that assigns status, bare or qualified.
var setsStatus = regexp.MustCompile(`(?i)(?:^|[\s,])(?:\w+\.)?status\s*=`)

// restatedClaim is the claim predicate spelled out rather than named as
// outbox.ClaimDue or outbox.ClaimDueAliasO.
var restatedClaim = regexp.MustCompile(`(?i)\bstatus\s*=\s*'pending'\s+AND\s+(?:\w+\.)?next_attempt_at\s*<=\s*now\(\)`)

// writesOutboxStatus reports whether one SQL string moves an outbox row's
// status itself.
func writesOutboxStatus(sql string) bool {
	for _, re := range []*regexp.Regexp{outboxUpdate, outboxUpsert} {
		for _, m := range re.FindAllStringSubmatch(sql, -1) {
			if setsStatus.MatchString(m[1]) {
				return true
			}
		}
	}
	return false
}

// TestWritesOutboxStatus_RecognizesTheShapes keeps the guardrail below
// honest: the statements it exists to refuse -- each one a writer this
// package once had to take back -- must read as status writes, and the
// legitimate outbox UPDATEs that touch other columns must not.
func TestWritesOutboxStatus_RecognizesTheShapes(t *testing.T) {
	refused := []string{
		`UPDATE staff_token_mail_outbox   SET status = 'sent', sent_at = $1 WHERE identity_uid = $2 AND status = 'pending'`,
		"UPDATE portal_invite_outbox SET status = $1\n WHERE id = (SELECT o.id FROM portal_invite_outbox o)",
		"UPDATE portal_invite_outbox o\n SET status = 'dead_lettered', dead_letter_reason = $3\n FROM client_portal_users pu WHERE o.id = pu.id",
		`UPDATE payout_outbox SET last_error = NULL, o.status = 'sent' WHERE id = $1`,
		`INSERT INTO billing_outbox (id) VALUES ($1) ON CONFLICT (id) DO UPDATE SET status = 'pending'`,
	}
	for _, sql := range refused {
		if !writesOutboxStatus(sql) {
			t.Errorf("writesOutboxStatus(%q) = false, want true", sql)
		}
	}
	allowed := []string{
		`UPDATE staff_invite_outbox SET invite_token = $2 WHERE invitation_id = $1 AND status = 'pending'`,
		`UPDATE welcome_credits_outbox SET notified_owner_staff_ids = $2::uuid[] WHERE id = $1`,
		`INSERT INTO staff_invite_outbox (invitation_id) VALUES ($1) ON CONFLICT (invitation_id) WHERE status = 'pending' DO UPDATE SET invite_token = $2, attempt_count = 0`,
		`UPDATE invoices SET status = 'paid' WHERE id = $1`,
	}
	for _, sql := range allowed {
		if writesOutboxStatus(sql) {
			t.Errorf("writesOutboxStatus(%q) = true, want false", sql)
		}
	}
}

// TestNoOutboxStatusIsWrittenOutsideTheDeclaration is the lifecycle's
// guardrail, in the shape of contracts' TestTransitions_DeclaresEvery-
// ContractRoute: a source scan over every non-test Go file in api/,
// reading each string literal the way the compiler will. Outside this
// package, no literal may assign an outbox table's status, and none may
// restate the claim predicate rather than name ClaimDue: a move of an
// outbox row is one of lifecycle.go's Transitions, made through a
// Worker method, or it is not made.
func TestNoOutboxStatusIsWrittenOutsideTheDeclaration(t *testing.T) {
	root := filepath.Join("..", "..")
	scanned, mentionsOutbox := 0, 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			// coverage:ignore reason: path came from walking root, so it is always relative to it
			return fmt.Errorf("relative path of %s: %w", path, err)
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/outbox/") {
			return nil
		}
		scanned++
		literals := stringLiterals(t, path)
		for _, lit := range literals {
			if strings.Contains(lit, "_outbox") {
				mentionsOutbox++
			}
			if writesOutboxStatus(lit) {
				if _, ok := statusWriteAllowed[rel]; !ok {
					t.Errorf("%s writes an outbox table's status itself (%q) -- move the row through an outbox.Worker method, the declared Transition, instead", rel, collapse(lit))
				}
			}
			if restatedClaim.MatchString(lit) {
				t.Errorf("%s restates the claim predicate (%q) -- name outbox.ClaimDue or outbox.ClaimDueAliasO instead", rel, collapse(lit))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	if scanned == 0 || mentionsOutbox == 0 {
		t.Fatalf("scanned %d files, %d literals naming an outbox -- the walk is not reaching api/'s source", scanned, mentionsOutbox)
	}
	for rel := range statusWriteAllowed {
		src := stringLiterals(t, filepath.Join(root, filepath.FromSlash(rel)))
		found := false
		for _, lit := range src {
			found = found || writesOutboxStatus(lit)
		}
		if !found {
			t.Errorf("%s is allowed to write an outbox status but no longer does -- take it off statusWriteAllowed", rel)
		}
	}
}

// stringLiterals is every string literal in the Go file at path, each
// unquoted, so a raw string and an interpreted one read alike.
func stringLiterals(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		s, err := strconv.Unquote(lit.Value)
		if err != nil {
			// coverage:ignore reason: a literal the Go parser accepted always unquotes
			t.Fatalf("unquote %s in %s: %v", lit.Value, path, err)
		}
		out = append(out, s)
		return true
	})
	return out
}

func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TestTransitions_TableIsComplete pins the declared moves: each one's
// from and to, and which of them is an exit from pending -- the moves on
// which a row's plaintext secrets are cleared.
func TestTransitions_TableIsComplete(t *testing.T) {
	type move struct {
		from, to      outbox.Status
		leavesPending bool
	}
	want := map[string]move{
		outbox.TransitionSend.Name:       {outbox.StatusPending, outbox.StatusSent, true},
		outbox.TransitionSkip.Name:       {outbox.StatusPending, outbox.StatusSkipped, true},
		outbox.TransitionRetry.Name:      {outbox.StatusPending, outbox.StatusPending, false},
		outbox.TransitionDeadLetter.Name: {outbox.StatusPending, outbox.StatusDeadLettered, true},
		outbox.TransitionBounce.Name:     {outbox.StatusSent, outbox.StatusBounced, false},
		outbox.TransitionComplain.Name:   {outbox.StatusSent, outbox.StatusComplained, false},
	}
	if len(outbox.Transitions) != len(want) {
		t.Fatalf("len(Transitions) = %d, want %d", len(outbox.Transitions), len(want))
	}
	for _, tr := range outbox.Transitions {
		w, known := want[tr.Name]
		if !known {
			t.Errorf("Transitions has unexpected entry %+v", tr)
			continue
		}
		if got := (move{tr.From, tr.To, tr.LeavesPending()}); got != w {
			t.Errorf("Transition %q = %+v, want %+v", tr.Name, got, w)
		}
	}
}
