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

	var offenses []string
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
			offenses = append(offenses, rel+":"+fset.Position(call.Pos()).String())
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk api module: %v", err)
	}

	if len(offenses) > 0 {
		t.Fatalf("http.Error called directly instead of apierr.Write/WriteError:\n%s",
			strings.Join(offenses, "\n"))
	}
}

// jsonUsageExceptions is #859's own audited list of production call sites
// that legitimately don't route through apierr.WriteJSON/DecodeJSON,
// because they aren't a fresh JSON encode or decode at all: idempotency's
// cache-hit replay writes a previously-encoded body byte-for-byte, and
// the three webhook handlers read the raw body for an HMAC check before
// calling json.Unmarshal on the already-read bytes (never
// json.NewDecoder). A new match anywhere else is a regression to fix,
// not a file to add here -- adding an entry needs the same kind of
// reasoning #859's own PR body recorded for these.
var jsonUsageExceptions = map[string]bool{
	filepath.Join("internal", "idempotency", "idempotency.go"): true,
}

// TestNoDirectJSONUsage is #842's own AST check (TestNoDirectHTTPError,
// above) extended per #848: a handler outside apierr that sets the JSON
// Content-Type, encodes with json.NewEncoder, or decodes with
// json.NewDecoder is a copy of apierr.WriteJSON/DecodeJSON's own job,
// which #859 swept away -- this is what stops that sweep from silently
// regressing one file at a time.
func TestNoDirectJSONUsage(t *testing.T) {
	root := "../.."

	var offenses []string
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
		if jsonUsageExceptions[rel] {
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

			// json.NewEncoder(...) / json.NewDecoder(...)
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "json" &&
				(sel.Sel.Name == "NewEncoder" || sel.Sel.Name == "NewDecoder") {
				offenses = append(offenses, rel+":"+fset.Position(call.Pos()).String()+": json."+sel.Sel.Name)
				return true
			}

			// anything.Header().Set("Content-Type", "application/json") --
			// a response header. sel.X must itself be a call (Header()),
			// which is what distinguishes a ResponseWriter's header from
			// an outgoing http.Request's Header field (req.Header.Set,
			// a field access with no call in between, e.g.
			// sitebuild.GitHubDispatcher building an outbound request).
			if _, isCall := sel.X.(*ast.CallExpr); sel.Sel.Name == "Set" && isCall && len(call.Args) == 2 {
				key, ok := call.Args[0].(*ast.BasicLit)
				if !ok || key.Kind != token.STRING || key.Value != `"Content-Type"` {
					return true
				}
				value, ok := call.Args[1].(*ast.BasicLit)
				if !ok || value.Kind != token.STRING || value.Value != `"application/json"` {
					return true
				}
				offenses = append(offenses, rel+":"+fset.Position(call.Pos()).String()+": Content-Type: application/json")
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk api module: %v", err)
	}

	if len(offenses) > 0 {
		t.Fatalf("JSON encode/decode/Content-Type set outside apierr.WriteJSON/DecodeJSON:\n%s",
			strings.Join(offenses, "\n"))
	}
}
