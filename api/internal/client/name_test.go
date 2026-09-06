package client_test

import (
	"testing"

	"doula-cloud/api/internal/client"
)

// testSarah is shared across this file and handlers_test.go's dedup
// fixtures, per golangci-lint's goconst check.
const testSarah = "Sarah"

// testHaddad, testNewEmail, testMaya and testStub are #814's own shared
// fixtures crossing golangci-lint's goconst threshold across
// collision_test.go, merge_test.go and handlers_test.go.
const (
	testHaddad   = "Haddad"
	testNewEmail = "new@example.com"
	testMaya     = "Maya"
	testStub     = "Stub"
	testNadia    = "Nadia"
	testFletcher = "Fletcher"
)

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
