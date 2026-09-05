package apierr_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoDirectHTTPError is the Go-side equivalent of app/src/lib's
// formErrors.usage.spec.ts and tokens.usage.spec.ts: it walks every
// production source file in the api module (root-level route wiring
// under package main included, not just api/internal) and fails if any
// handler outside this package calls http.Error directly instead of
// Write or WriteError. Test files are excluded -- the handful that still
// call http.Error are mock handlers standing in for a third party
// (mailgun) or for middleware under test, not production API responses.
func TestNoDirectHTTPError(t *testing.T) {
	root := "../.."

	var offences []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("rel %s: %w", path, err)
		}
		if strings.HasPrefix(rel, filepath.Join("internal", "apierr")+string(filepath.Separator)) {
			return nil
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok || ident.Name != "http" || sel.Sel.Name != "Error" {
				return true
			}
			offences = append(offences, rel+":"+fset.Position(call.Pos()).String())
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk api module: %v", err)
	}

	if len(offences) > 0 {
		t.Fatalf("http.Error called directly instead of apierr.Write/WriteError:\n%s",
			strings.Join(offences, "\n"))
	}
}
