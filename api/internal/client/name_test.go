package client_test

import (
	"testing"

	"doula-cloud/api/internal/client"
)

// testSarah is shared across this file and handlers_test.go's dedup
// fixtures, per golangci-lint's goconst check.
const testSarah = "Sarah"

func TestLegalName(t *testing.T) {
	cases := []struct {
		givenName, familyName, want string
	}{
		{testSarah, "Beck", "Sarah Beck"},
		{testSarah, "", testSarah},
	}
	for _, c := range cases {
		if got := client.LegalName(c.givenName, c.familyName); got != c.want {
			t.Fatalf("LegalName(%q, %q) = %q, want %q", c.givenName, c.familyName, got, c.want)
		}
	}
}

func TestPreferredName(t *testing.T) {
	cases := []struct {
		givenName, preferredName, want string
	}{
		{testSarah, "Sadie", "Sadie"},
		{testSarah, "", testSarah},
	}
	for _, c := range cases {
		if got := client.PreferredName(c.givenName, c.preferredName); got != c.want {
			t.Fatalf("PreferredName(%q, %q) = %q, want %q", c.givenName, c.preferredName, got, c.want)
		}
	}
}
