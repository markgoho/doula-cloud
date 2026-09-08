package apierr_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// bannedDetailWords is app/src/lib/formErrors.usage.spec.ts's own list,
// which is GOV.UK's: an error message says what to do, starts with the
// field's own noun, and never reaches for one of these. "invalid" is
// listed beside "valid" rather than left to be caught by it, because the
// match below is whole-word.
var bannedDetailWords = []string{"please", "valid", "invalid", "required"}

// apiModuleRoot is the api module root relative to this package's own
// directory -- what every AST gate in this package walks.
const apiModuleRoot = "../.."

// TestDetailsWording is #488's Go half of the wording gate
// app/src/lib/formErrors.usage.spec.ts already runs over the client.
//
// Scope is APIError.Details, not Message, and the line is drawn there on
// purpose. A Details entry is written to be shown beside a control and
// read by a person -- it is the only server string the app prints as a
// field-level error (app/src/lib/formErrors.ts's refusalErrors), so it is
// held to the client's own rules. Message is the summary a caller reads,
// it names JSON fields and formats ("workState is required, and must be a
// two-letter US state abbreviation"), and 64 call sites across
// api/internal write one that way today; rewording those is a separate
// piece of work from making Details real, which is the half #488's own
// acceptance criteria asks a Go test for.
//
// Resolution is deliberately shallow: a string literal is read directly,
// and an identifier is resolved against its own package's top-level
// string constants -- which is how every site writes one today
// (staffauth's fielderrors.go, clientauth's MsgAddress*). A value this
// cannot resolve (a function call, a cross-package selector) is skipped
// rather than guessed at, so the gate only ever reports a string it
// actually read.
func TestDetailsWording(t *testing.T) {
	root := apiModuleRoot

	byDir := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		byDir[dir] = append(byDir[dir], path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk api module: %v", err)
	}

	var offenses []string
	for _, paths := range byDir {
		fset := token.NewFileSet()
		files := make([]*ast.File, 0, len(paths))
		for _, path := range paths {
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			files = append(files, file)
		}

		consts := map[string]string{}
		for _, file := range files {
			collectStringConsts(file, consts)
		}

		for i, file := range files {
			rel, err := filepath.Rel(root, paths[i])
			if err != nil {
				t.Fatalf("rel %s: %v", paths[i], err)
			}
			offenses = append(offenses, detailOffenses(file, fset, rel, consts)...)
		}
	}

	if len(offenses) > 0 {
		sort.Strings(offenses)
		t.Fatalf("APIError.Details message uses a word GOV.UK's error-message rules forbid:\n%s",
			strings.Join(offenses, "\n"))
	}
}

// collectStringConsts records every top-level `const name = "literal"` in
// file, so an identifier used as a Details value can be read back.
func collectStringConsts(file *ast.File, into map[string]string) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i >= len(value.Values) {
					continue
				}
				if text, ok := stringValue(value.Values[i], nil); ok {
					into[name.Name] = text
				}
			}
		}
	}
}

// detailOffenses reports every banned word in a Details value written in
// file: the values of a map[string]string composite literal, and the
// right-hand side of an assignment into one built up key by key.
func detailOffenses(file *ast.File, fset *token.FileSet, rel string, consts map[string]string) []string {
	var offenses []string
	check := func(expr ast.Expr) {
		text, ok := stringValue(expr, consts)
		if !ok {
			return
		}
		for _, word := range bannedDetailWords {
			if containsWord(text, word) {
				offenses = append(offenses,
					fmt.Sprintf("%s:%d: %q contains %q", rel, fset.Position(expr.Pos()).Line, text, word))
			}
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if !isStringMapType(node.Type) {
				return true
			}
			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					check(kv.Value)
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if _, ok := lhs.(*ast.IndexExpr); !ok || i >= len(node.Rhs) {
					continue
				}
				check(node.Rhs[i])
			}
		}
		return true
	})
	return offenses
}

// isStringMapType reports whether expr is the type `map[string]string` --
// the one shape APIError.Details takes.
func isStringMapType(expr ast.Expr) bool {
	mapType, ok := expr.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mapType.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return false
	}
	value, ok := mapType.Value.(*ast.Ident)
	return ok && value.Name == "string"
}

// stringValue reads expr as the string it will be at run time, for the
// two forms a Details value takes today: a literal, and an identifier
// naming one of its own package's string constants. A nil consts map
// reads literals only, which is what collectStringConsts needs while it
// is still building that map.
func stringValue(expr ast.Expr, consts map[string]string) (string, bool) {
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(node.Value)
		return unquoted, err == nil
	case *ast.Ident:
		text, ok := consts[node.Name]
		return text, ok
	}
	return "", false
}

// containsWord reports whether text holds word as a whole word, ignoring
// case -- so "invalid" is not reported as "valid", and a word that merely
// starts with one of them is not reported at all.
func containsWord(text, word string) bool {
	lower := strings.ToLower(text)
	for i := 0; i+len(word) <= len(lower); i++ {
		if lower[i:i+len(word)] != word {
			continue
		}
		if i > 0 && isWordByte(lower[i-1]) {
			continue
		}
		if i+len(word) < len(lower) && isWordByte(lower[i+len(word)]) {
			continue
		}
		return true
	}
	return false
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// TestDetailOffensesCatchesABannedWord is the gate's own proof: without
// it TestDetailsWording passing would be indistinguishable from it
// reading nothing at all. Both value forms the gate resolves are here,
// alongside a whole-word near-miss it must not report.
func TestDetailOffensesCatchesABannedWord(t *testing.T) {
	const src = `package p

const MsgBad = "Password is required"

func f(w W) {
	Write(w, map[string]string{"email": "Enter a valid address"})
	Write(w, map[string]string{"name": MsgBad})
	Write(w, map[string]string{"ok": "Enter the invalidation date"})
	details := map[string]string{}
	details["roles"] = "Please select a role"
	other := map[string]int{"n": 1}
	_ = other
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	consts := map[string]string{}
	collectStringConsts(file, consts)
	if consts["MsgBad"] != "Password is required" {
		t.Fatalf("consts = %v, want MsgBad resolved", consts)
	}

	got := detailOffenses(file, fset, "p.go", consts)
	sort.Strings(got)
	want := []string{
		`p.go:10: "Please select a role" contains "please"`,
		`p.go:6: "Enter a valid address" contains "valid"`,
		`p.go:7: "Password is required" contains "required"`,
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("offenses = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("offenses = %v, want %v", got, want)
		}
	}
}
