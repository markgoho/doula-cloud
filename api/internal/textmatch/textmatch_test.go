package textmatch_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/textmatch"
)

func TestContains(t *testing.T) {
	cases := map[string]string{
		"":         "",
		"Quinn":    "%Quinn%",
		"50%_off":  `%50\%\_off%`,
		`back\one`: `%back\\one%`,
	}
	for text, want := range cases {
		if got := textmatch.Contains(text); got != want {
			t.Errorf("Contains(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestClientNameExpr_BindsEveryNameToOneParameter(t *testing.T) {
	expr := textmatch.ClientNameExpr("cl", 7)
	if got := strings.Count(expr, "ILIKE $7"); got != 5 {
		t.Fatalf("ILIKE $7 appears %d times, want 5:\n%s", got, expr)
	}
	if strings.Contains(expr, " c.") || !strings.Contains(expr, "cl.family_name") {
		t.Fatalf("expr does not read the alias it was given:\n%s", expr)
	}
}
