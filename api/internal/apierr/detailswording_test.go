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
	offenses := runDetailGate(t, detailOffenses)
	if len(offenses) > 0 {
		sort.Strings(offenses)
		t.Fatalf("APIError.Details message uses a word GOV.UK's error-message rules forbid:\n%s",
			strings.Join(offenses, "\n"))
	}
}

// TestDetailsOpenWithThePersonsNoun is #1189's own gate: a Details value
// is read beside a control on someone's screen, and the noun that
// belongs there is the one printed on that control, not the request
// DTO's own JSON identifier -- "paidOn cannot be in the future" opens
// with the wire's name for the field; "The date cannot be in the future"
// opens with the person's. TestDetailsWording's banned-word list catches
// a wrong register; this catches the narrower, mechanical case #1189
// found repeated across api/internal -- a value that opens with its own
// key verbatim, case aside, because it was written by echoing the key
// rather than reading the screen.
func TestDetailsOpenWithThePersonsNoun(t *testing.T) {
	offenses := runDetailGate(t, identifierOffenses)
	if len(offenses) > 0 {
		sort.Strings(offenses)
		t.Fatalf("APIError.Details value opens with the wire's own name for the field, not the person's:\n%s",
			strings.Join(offenses, "\n"))
	}
}

// runDetailGate walks every non-test .go file under apiModuleRoot,
// grouped by directory so each package's own top-level string constants
// resolve within it, and hands every file to collect -- the one piece
// that differs between TestDetailsWording and
// TestDetailsOpenWithThePersonsNoun.
//
// dirImportsApierr is decided per directory, not per file: a package can
// spread apierr.Write's own inputs across files the way website does --
// website.go builds the Details map, and only handler.go, a different
// file in the same package, imports apierr to send it -- so asking a
// single file whether it imports apierr would miss that site.
func runDetailGate(t *testing.T, collect func(file *ast.File, fset *token.FileSet, rel string, consts map[string]string, dirImportsApierr bool) []string) []string {
	t.Helper()
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
		dirImportsApierr := false
		for _, file := range files {
			collectStringConsts(file, consts)
			if importsApierr(file) {
				dirImportsApierr = true
			}
		}

		for i, file := range files {
			rel, err := filepath.Rel(root, paths[i])
			if err != nil {
				t.Fatalf("rel %s: %v", paths[i], err)
			}
			offenses = append(offenses, collect(file, fset, rel, consts, dirImportsApierr)...)
		}
	}
	return offenses
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
// file: the values of a map[string]string composite literal, the
// right-hand side of an assignment into one built up key by key, and the
// message argument of an apierr.WriteFieldError call -- WriteFieldError
// turns that one string into the single Details entry itself (#1188), so
// a call site that moves onto it carries no map[string]string literal of
// its own for this gate to read the old way.
func detailOffenses(file *ast.File, fset *token.FileSet, rel string, consts map[string]string, _ bool) []string {
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
		case *ast.CallExpr:
			if isWriteFieldErrorCall(node) && len(node.Args) >= 5 {
				check(node.Args[4])
			}
		}
		return true
	})
	return offenses
}

// identifierOffenses reports every Details value in file that opens with
// its own key as a bare identifier -- the wire's own name for the field,
// which docs/api-design.md section 7 rule 4 reserves for Message, not
// Details (#1189). Reads the same three shapes detailOffenses does, this
// time keeping the key beside the value: the values of a
// map[string]string composite literal, an assignment into one built up
// key by key, and the field/message pair of an apierr.WriteFieldError
// call. A key this cannot resolve -- contracts/send.go's per-merge-field
// key, built from a loop variable rather than a literal -- is skipped
// rather than guessed at, the same restraint stringValue already applies
// to a value it cannot read.
//
// The composite-literal and keyed-assignment shapes are read from
// function bodies only, gated on dirImportsApierr (decided per package
// directory by runDetailGate, not per file, since a package can spread
// apierr.Write's own inputs across files the way website does), and
// never from a package-level var: clientfieldtemplate/validate.go's own
// structuralFieldNames is a map[string]string too, matching a Practice's
// typed label back to the ADR-0017 structural fact it shadows, and nine
// of its entries read back their own key by construction ("given name"
// maps to structuralGivenName, itself "given name") -- but it is a
// package-level lookup table declared once, not a per-request Details
// map a handler builds, and every real Details map in api/internal is
// the latter: a local variable inside a function body, or a literal
// passed inline as a call argument. Restricting the walk to function
// bodies reads structuralFieldNames as what it is, a var declaration
// nothing here inspects, rather than nine Details entries that happen to
// name their own field. The WriteFieldError call shape needs no such
// restriction: it can only ever appear inside a function body already.
func identifierOffenses(file *ast.File, fset *token.FileSet, rel string, consts map[string]string, dirImportsApierr bool) []string {
	var offenses []string
	check := func(keyExpr, valueExpr ast.Expr) {
		key, ok := stringValue(keyExpr, consts)
		if !ok {
			return
		}
		text, ok := stringValue(valueExpr, consts)
		if !ok {
			return
		}
		if opensWithIdentifier(text, key) {
			offenses = append(offenses,
				fmt.Sprintf("%s:%d: %q opens with %q, its own key", rel, fset.Position(valueExpr.Pos()).Line, text, key))
		}
	}

	visit := func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if !dirImportsApierr || !isStringMapType(node.Type) {
				return true
			}
			for _, elt := range node.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					check(kv.Key, kv.Value)
				}
			}
		case *ast.AssignStmt:
			if !dirImportsApierr {
				return true
			}
			for i, lhs := range node.Lhs {
				index, ok := lhs.(*ast.IndexExpr)
				if !ok || i >= len(node.Rhs) {
					continue
				}
				check(index.Index, node.Rhs[i])
			}
		case *ast.CallExpr:
			if isWriteFieldErrorCall(node) && len(node.Args) >= 5 {
				check(node.Args[3], node.Args[4])
			}
		}
		return true
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, visit)
	}
	return offenses
}

// importsApierr reports whether file imports apierr, by import path
// rather than its local name, so an aliased import still counts.
func importsApierr(file *ast.File) bool {
	const apierrPath = `"doula-cloud/api/internal/apierr"`
	for _, imp := range file.Imports {
		if imp.Path.Value == apierrPath {
			return true
		}
	}
	return false
}

// opensWithIdentifier reports whether text opens with key as a bare
// word, case aside -- "paidOn cannot be in the future" opens with
// "paidOn"; "Enter the date received..." does not, and neither does
// "Timezone must be..." open with "time" (a prefix of "timezone" is not
// the whole key, so it is not a match; "timezone" itself would be).
func opensWithIdentifier(text, key string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	lowerKey := strings.ToLower(key)
	if lowerKey == "" || !strings.HasPrefix(lower, lowerKey) {
		return false
	}
	return len(lower) == len(lowerKey) || !isWordByte(lower[len(lowerKey)])
}

// isWriteFieldErrorCall reports whether call is apierr.WriteFieldError(...).
func isWriteFieldErrorCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "WriteFieldError" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "apierr"
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
	apierr.WriteFieldError(w, 400, CodeInvalidArgument, "netDays", "Enter a valid number of days")
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

	got := detailOffenses(file, fset, "p.go", consts, true)
	sort.Strings(got)
	want := []string{
		`p.go:10: "Please select a role" contains "please"`,
		`p.go:13: "Enter a valid number of days" contains "valid"`,
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

// TestIdentifierOffensesCatchesABareIdentifierOpener is
// TestDetailsOpenWithThePersonsNoun's own gate proof, the counterpart to
// TestDetailOffensesCatchesABannedWord above. It covers all three shapes
// identifierOffenses reads, a same-package const resolved through
// consts, a key it cannot resolve (skipped rather than guessed at), and
// a near-miss that must not report: a value that merely contains its key
// after the first word, and one whose opening word only shares a prefix
// with the key ("time" is not "timezone").
func TestIdentifierOffensesCatchesABareIdentifierOpener(t *testing.T) {
	const src = `package p

import "doula-cloud/api/internal/apierr"

const MsgBad = "netDays is out of range"

func f(w W) {
	Write(w, map[string]string{"paidOn": "paidOn cannot be in the future"})
	Write(w, map[string]string{"netDays": MsgBad})
	Write(w, map[string]string{"timezone": "time to choose a real one"})
	Write(w, map[string]string{"note": "Enter a note for the record, note it well"})
	details := map[string]string{}
	details["mode"] = "mode must be \"own\" or \"hosted\""
	details[dynamicKey] = "reason cannot be blank"
	apierr.WriteFieldError(w, 400, CodeInvalidArgument, "reason", "reason cannot be blank")
	apierr.WriteFieldError(w, 400, CodeInvalidArgument, "reason", "Enter a reason for reversing this payment")
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	consts := map[string]string{}
	collectStringConsts(file, consts)

	got := identifierOffenses(file, fset, "p.go", consts, true)
	sort.Strings(got)
	want := []string{
		`p.go:13: "mode must be \"own\" or \"hosted\"" opens with "mode", its own key`,
		`p.go:15: "reason cannot be blank" opens with "reason", its own key`,
		`p.go:8: "paidOn cannot be in the future" opens with "paidOn", its own key`,
		`p.go:9: "netDays is out of range" opens with "netDays", its own key`,
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

	// dirImportsApierr=false is website.go's own shape: a directory
	// where some other file, not this one, imports apierr and sends what
	// this one builds. The map/assignment offenses go dark -- there is
	// nothing here to tell this map from clientfieldtemplate's
	// structuralFieldNames -- but a WriteFieldError call still can't
	// exist in a file that doesn't import apierr, so that shape is
	// unaffected either way.
	gotUnimported := identifierOffenses(file, fset, "p.go", consts, false)
	sort.Strings(gotUnimported)
	wantUnimported := []string{
		`p.go:15: "reason cannot be blank" opens with "reason", its own key`,
	}
	if len(gotUnimported) != len(wantUnimported) || gotUnimported[0] != wantUnimported[0] {
		t.Fatalf("offenses with dirImportsApierr=false = %v, want %v", gotUnimported, wantUnimported)
	}
}
