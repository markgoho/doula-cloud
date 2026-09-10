package apierr_test

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
)

// apierrPackage is this package's own name as a call site outside it
// spells it -- the qualifier both AST matchers below look for, and the
// one usage_test.go's envelope-package skip already needed, so goconst
// sees one literal rather than three.
const apierrPackage = "apierr"

// apierrTestPackage is the shared-decoder package #811 landed beside this
// one. Its own literal rather than apierrPackage + "test", so grepping
// the repository for the package name still finds the exemption that
// names it.
const apierrTestPackage = "apierrtest"

// forbiddenCodeIdents names the constants apierr.ForbiddenCodes holds,
// spelled the way a call site spells them. The AST walk below sees an
// identifier, never a value, so it needs the names; the values are here
// too so TestForbiddenCodesAreTheRecordedSet can prove this table and
// apierr.ForbiddenCodes still describe the same three codes rather than
// drifting apart the first time one of them is added to alone.
var forbiddenCodeIdents = map[string]apierr.Code{
	"CodeForbidden":               apierr.CodeForbidden,
	"CodePracticePendingDeletion": apierr.CodePracticePendingDeletion,
	"CodeMFARequired":             apierr.CodeMFARequired,
}

// TestForbiddenCodesAreTheRecordedSet is the cross-check that keeps the
// table above honest: every name in it resolves to a code the recorded
// set holds, and the two are the same size, so adding a fourth 403
// reason to apierr without teaching this guardrail its name fails here
// rather than silently widening what the walk accepts.
func TestForbiddenCodesAreTheRecordedSet(t *testing.T) {
	if len(forbiddenCodeIdents) != len(apierr.ForbiddenCodes) {
		t.Fatalf("forbiddenCodeIdents has %d entries, apierr.ForbiddenCodes has %d",
			len(forbiddenCodeIdents), len(apierr.ForbiddenCodes))
	}
	for name, code := range forbiddenCodeIdents {
		if !apierr.ForbiddenCodes[code] {
			t.Errorf("apierr.%s = %q, which apierr.ForbiddenCodes does not hold", name, code)
		}
	}
}

// TestEveryForbiddenWriteCarriesARecordedCode is #918's guardrail, beside
// TestNoDirectHTTPError and TestNoDirectJSONUsage: a 403 written with a
// code outside apierr.ForbiddenCodes fails the build.
//
// It reads the two shapes a 403's code is actually chosen in. The first
// is an apierr.Write call site naming the status literally. The second
// is a struct literal that carries a refusal around before something
// else writes it -- sessionmint.Refusal and offer's pre-account read
// both do this, deciding a Status and an optional Code several frames
// from the Write that eventually forwards them, so a walk that read only
// call sites would miss a Code set there.
//
// It is the last layer of an argument rather than the whole of it.
// TestNoDirectHTTPError already forces every refusal in the module
// through apierr, and Write's only other source of a code is
// CodeForStatus, which answers 403 with CodeForbidden. So a 403 reaches
// the wire through WriteError (recorded by construction), through a
// Write whose code came from CodeForStatus (the same), or through a code
// somebody wrote down -- and every place a code is written down beside a
// literal 403 is one of the two shapes above.
func TestEveryForbiddenWriteCarriesARecordedCode(t *testing.T) {
	offenses, err := forbiddenWriteOffenses(apiModuleRoot)
	if err != nil {
		t.Fatalf("walk api module: %v", err)
	}
	if len(offenses) > 0 {
		t.Fatalf("403 written with a code outside apierr.ForbiddenCodes:\n%s",
			strings.Join(offenses, "\n"))
	}
}

// TestForbiddenWriteOffensesCatchesAViolation exercises the walk against
// a file that breaks the rule, so a guardrail that had quietly stopped
// matching anything -- a renamed helper, a changed argument order --
// could not go on passing the test above by finding nothing anywhere.
func TestForbiddenWriteOffensesCatchesAViolation(t *testing.T) {
	root := t.TempDir()
	source := `package sample

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
)

type refusal struct {
	Status  int
	Code    apierr.Code
	Message string
}

func refuse(w http.ResponseWriter) {
	apierr.Write(w, http.StatusForbidden, apierr.CodeConflict, "nope", nil)
	apierr.Write(w, 403, apierr.CodeInternal, "nope", nil)
	apierr.Write(w, http.StatusForbidden, apierr.CodeMFARequired, "fine", nil)
	apierr.Write(w, http.StatusConflict, apierr.CodeConflict, "fine", nil)
	_ = refusal{Status: http.StatusForbidden, Code: apierr.CodeOfferCodeExhausted, Message: "nope"}
	_ = refusal{Status: http.StatusForbidden, Code: apierr.CodeForbidden, Message: "fine"}
	_ = refusal{Status: http.StatusForbidden, Message: "fine, CodeForStatus decides"}
	_ = refusal{Status: http.StatusConflict, Code: apierr.CodeConflict, Message: "fine"}
}
`
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	offenses, err := forbiddenWriteOffenses(root)
	if err != nil {
		t.Fatalf("walk sample: %v", err)
	}
	if len(offenses) != 3 {
		t.Fatalf("offenses = %v, want the three refusals carrying an unrecorded code", offenses)
	}
	for _, offense := range offenses {
		if !strings.Contains(offense, "sample.go") {
			t.Errorf("offense %q does not name the file it was found in", offense)
		}
	}
}

// forbiddenWriteOffenses walks every production Go file under root and
// reports each place a literal 403 is paired with a code outside
// forbiddenCodeIdents -- in an apierr.Write call, or in a struct literal
// carrying a Status and a Code. A code that is not a plain
// apierr.Code<Name> selector at all -- a local variable, a bare string
// -- is an offense too: the point of the set is that a reader of the
// call site can see which of the three reasons this is. A struct literal
// that sets a 403 Status and no Code at all is fine, because whatever
// writes it falls back to CodeForStatus.
func forbiddenWriteOffenses(root string) ([]string, error) {
	var offenses []string
	err := walkProductionFiles(root, func(rel string, fset *token.FileSet, file *ast.File) {
		report := func(pos token.Pos) {
			offenses = append(offenses,
				rel+":"+fset.Position(pos).String()+": 403 with an unrecorded code")
		}

		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if len(node.Args) >= 3 && isAPIErrWrite(node.Fun) && isForbiddenStatus(node.Args[1]) &&
					recordedCodeName(node.Args[2]) == "" {
					report(node.Pos())
				}
			case *ast.CompositeLit:
				status, code := keyedField(node, "Status"), keyedField(node, "Code")
				if status != nil && code != nil && isForbiddenStatus(status) && recordedCodeName(code) == "" {
					report(node.Pos())
				}
			}
			return true
		})
	})
	if err != nil {
		// coverage:ignore reason: a filesystem walk failure over the module's own source, not reachable from a test
		return nil, fmt.Errorf("collect 403 offenses: %w", err)
	}
	return offenses, nil
}

// keyedField returns the value a struct literal gives the named field, or
// nil when the literal is positional or leaves that field out.
func keyedField(lit *ast.CompositeLit, name string) ast.Expr {
	for _, element := range lit.Elts {
		pair, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := pair.Key.(*ast.Ident); ok && key.Name == name {
			return pair.Value
		}
	}
	return nil
}

// isAPIErrWrite reports whether fun names apierr.Write. WriteError is
// deliberately not matched: it has no code argument at all, and takes
// the one CodeForStatus decides.
func isAPIErrWrite(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Write" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == apierrPackage
}

// isForbiddenStatus reports whether expr is 403 written either way a
// call site can write it.
func isForbiddenStatus(expr ast.Expr) bool {
	switch status := expr.(type) {
	case *ast.BasicLit:
		return status.Kind == token.INT && status.Value == "403"
	case *ast.SelectorExpr:
		pkg, ok := status.X.(*ast.Ident)
		return ok && pkg.Name == "http" && status.Sel.Name == "StatusForbidden"
	default:
		return false
	}
}

// recordedCodeName returns the constant name expr names, when expr is an
// apierr.Code<Name> selector for a code the recorded set holds, and ""
// for anything else.
func recordedCodeName(expr ast.Expr) string {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != apierrPackage {
		return ""
	}
	if _, recorded := forbiddenCodeIdents[sel.Sel.Name]; !recorded {
		return ""
	}
	return sel.Sel.Name
}
