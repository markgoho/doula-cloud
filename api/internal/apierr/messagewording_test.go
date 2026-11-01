package apierr_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

// The gate below is #1884's: APIError.Message is the sentence a person
// reads. The app prints an untargeted refusal's message in an error
// summary as it is (app/src/lib/formErrors.ts's targetedErrors), and a
// program reads `code`, never the prose (docs/api-design.md section 7
// rule 3), so a message has one reader, the person at the screen. Rule 4
// of that section holds it to the Voice section of docs/design/brief.md.

// thirdPersonPronouns are the words the Voice section's rule 1 keeps off
// every screen. A message that names a person names them by the domain
// noun, or says "you" to the reader.
var thirdPersonPronouns = []string{"she", "her", "hers", "herself", "he", "him", "his", "himself"}

// lowercaseMessageCeiling is how many messages the gate reads that still
// open in lowercase, the engineer form ("contract cannot be billed: it
// is voided") that was the rule before #1884. They are reworded on
// #1894; each one reworded lowers this number by one, and a new one
// fails the build. When it reaches 0, the ratchet becomes a plain rule.
const lowercaseMessageCeiling = 286

// TestMessagesUseNoThirdPersonPronoun fails the build on a message that
// says "she", "her" or another third-person pronoun about anyone.
func TestMessagesUseNoThirdPersonPronoun(t *testing.T) {
	offenses := runDetailGate(t, pronounOffenses)
	if len(offenses) > 0 {
		sort.Strings(offenses)
		t.Fatalf("APIError.Message uses a third-person pronoun (docs/design/brief.md, Voice rule 1):\n%s",
			strings.Join(offenses, "\n"))
	}
}

// TestMessagesOpenWithACapital is the ratchet: the count of messages that
// open in lowercase may not grow, and when one is reworded the ceiling
// comes down with it, so the number in this file is always the true one.
func TestMessagesOpenWithACapital(t *testing.T) {
	offenders := runDetailGate(t, lowercaseOpeners)
	sort.Strings(offenders)
	switch {
	case len(offenders) > lowercaseMessageCeiling:
		t.Fatalf("%d APIError messages open in lowercase, over the ceiling of %d. A message is the sentence a person reads (docs/api-design.md section 7 rule 4): open it with a capital.\n%s",
			len(offenders), lowercaseMessageCeiling, strings.Join(offenders, "\n"))
	case len(offenders) < lowercaseMessageCeiling:
		t.Fatalf("%d APIError messages open in lowercase, under the ceiling of %d. Lower lowercaseMessageCeiling to %d.",
			len(offenders), lowercaseMessageCeiling, len(offenders))
	}
}

// messageTexts returns every message the gate can read in file, keyed by
// its position: the message argument of apierr.WriteError, apierr.Write,
// apierr.WriteFieldError and apierr.FieldDetails. A message is read as a
// literal, as a same-package string constant, or as the format string of
// a fmt.Sprintf call. Anything else (a function call such as
// contracts.Transition.Check, a concatenation) is skipped, the same
// restraint stringValue applies; such a message carries its own test
// beside the code that builds it.
func messageTexts(file *ast.File, fset *token.FileSet, rel string, consts map[string]string) map[string]string {
	texts := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		index := -1
		switch apierrCallName(call) {
		case "WriteError", "FieldDetails":
			index = 1
		case "Write":
			index = 3
		case "WriteFieldError":
			index = 4
		}
		if index < 0 || index >= len(call.Args) {
			return true
		}
		expr := call.Args[index]
		if inner, ok := expr.(*ast.CallExpr); ok && isSprintfCall(inner) && len(inner.Args) > 0 {
			expr = inner.Args[0]
		}
		if text, ok := stringValue(expr, consts); ok {
			texts[fmt.Sprintf("%s:%d", rel, fset.Position(expr.Pos()).Line)] = text
		}
		return true
	})
	return texts
}

func isSprintfCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "fmt" && sel.Sel.Name == "Sprintf"
}

func pronounOffenses(file *ast.File, fset *token.FileSet, rel string, consts map[string]string, _ bool) []string {
	var offenses []string
	for at, text := range messageTexts(file, fset, rel, consts) {
		for _, word := range thirdPersonPronouns {
			if containsWord(text, word) {
				offenses = append(offenses, fmt.Sprintf("%s: %q contains %q", at, text, word))
			}
		}
	}
	return offenses
}

func lowercaseOpeners(file *ast.File, fset *token.FileSet, rel string, consts map[string]string, _ bool) []string {
	var offenders []string
	for at, text := range messageTexts(file, fset, rel, consts) {
		if opensInLowercase(text) {
			offenders = append(offenders, fmt.Sprintf("%s: %q", at, text))
		}
	}
	return offenders
}

// opensInLowercase reports whether text's first character is a lowercase
// letter. A message that opens with a format verb ("%s ...") is not
// counted: what it opens with is only known at run time.
func opensInLowercase(text string) bool {
	first, _ := utf8.DecodeRuneInString(strings.TrimSpace(text))
	return unicode.IsLower(first)
}

// TestMessageGateReadsEveryForm is the two gates' own proof: without it a
// pass would look the same as a gate that reads nothing. Each message
// form the gate resolves is here, beside forms it must skip.
func TestMessageGateReadsEveryForm(t *testing.T) {
	const src = `package p

const msgBad = "a contractor does not search where she works"

func f(w W) {
	apierr.WriteError(w, "this client has no email -- add one before invoicing her", 422)
	apierr.Write(w, 409, CodeConflict, msgBad, nil)
	apierr.WriteError(w, fmt.Sprintf("contract cannot be %s", s), 409)
	apierr.WriteFieldError(w, 422, CodeInvalidArgument, "email", "Enter his email address")
	apierr.FieldDetails("name", "Enter a name")
	apierr.WriteError(w, fmt.Sprintf("%s cannot be found", s), 404)
	apierr.WriteError(w, check(s), 409)
	apierr.WriteError(w, "The history of this Practice", 409)
	other.WriteError(w, "skipped, not apierr", 409)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	consts := map[string]string{}
	collectStringConsts(file, consts)

	assertSorted(t, "pronouns", pronounOffenses(file, fset, "p.go", consts, true), []string{
		`p.go:9: "Enter his email address" contains "his"`,
		`p.go:6: "this client has no email -- add one before invoicing her" contains "her"`,
		`p.go:7: "a contractor does not search where she works" contains "she"`,
	})
	assertSorted(t, "lowercase openers", lowercaseOpeners(file, fset, "p.go", consts, true), []string{
		`p.go:6: "this client has no email -- add one before invoicing her"`,
		`p.go:7: "a contractor does not search where she works"`,
		`p.go:8: "contract cannot be %s"`,
	})
}

func assertSorted(t *testing.T, name string, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}
