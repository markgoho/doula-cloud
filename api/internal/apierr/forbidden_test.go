package apierr_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
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
// It is the third layer of three, not the whole of the argument that
// every 403 carries a recorded code. TestNoDirectHTTPError already
// forces every refusal in the module through apierr, and Write's only
// other source of a code is CodeForStatus, which answers 403 with
// CodeForbidden. So a 403 reaches the wire either through WriteError --
// recorded by construction -- or through an apierr.Write call site, and
// this walk reads those.
//
// What it does not read is a call site whose status is a variable rather
// than the literal (offer's pre-account read, which forwards a status
// and a code decided several frames down). Those pass a code that is
// already CodeForStatus's own answer when the decision named none, so
// they are covered by the same construction argument; a walk that tried
// to follow them would be a type checker, not a guardrail.
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

func refuse(w http.ResponseWriter) {
	apierr.Write(w, http.StatusForbidden, apierr.CodeConflict, "nope", nil)
	apierr.Write(w, 403, apierr.CodeInternal, "nope", nil)
	apierr.Write(w, http.StatusForbidden, apierr.CodeMFARequired, "fine", nil)
	apierr.Write(w, http.StatusConflict, apierr.CodeConflict, "fine", nil)
}
`
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatalf("write sample: %v", err)
	}

	offenses, err := forbiddenWriteOffenses(root)
	if err != nil {
		t.Fatalf("walk sample: %v", err)
	}
	if len(offenses) != 2 {
		t.Fatalf("offenses = %v, want the two refusals carrying an unrecorded code", offenses)
	}
	for _, offense := range offenses {
		if !strings.Contains(offense, "sample.go") {
			t.Errorf("offense %q does not name the file it was found in", offense)
		}
	}
}

// forbiddenWriteOffenses walks every production Go file under root and
// reports each apierr.Write whose status argument is literally 403 and
// whose code argument is not one of forbiddenCodeIdents. A code that is
// not a plain apierr.Code<Name> selector at all -- a local variable, a
// bare string -- is an offense too: the point of the set is that a
// reader of the call site can see which of the three reasons this is.
func forbiddenWriteOffenses(root string) ([]string, error) {
	var offenses []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 3 || !isAPIErrWrite(call.Fun) || !isForbiddenStatus(call.Args[1]) {
				return true
			}
			if recordedCodeName(call.Args[2]) == "" {
				offenses = append(offenses,
					rel+":"+fset.Position(call.Pos()).String()+": 403 with an unrecorded code")
			}
			return true
		})
		return nil
	})
	if err != nil {
		// coverage:ignore reason: a filesystem walk failure over the module's own source, not reachable from a test
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	return offenses, nil
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
