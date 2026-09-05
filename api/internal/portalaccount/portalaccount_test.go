package portalaccount_test

import (
	"strings"
	"testing"

	"doula-cloud/api/internal/portalaccount"
)

// firebaseUIDAlphabet is every character Identity Platform's own
// documentation says an auto-generated uid is drawn from -- see
// portalaccount.Prefix's doc comment for the source. Prefix must contain
// at least one character outside it for the non-collision claim to hold.
const firebaseUIDAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func TestPrefix_ContainsACharacterOutsideTheFirebaseUIDAlphabet(t *testing.T) {
	for _, r := range portalaccount.Prefix {
		if !strings.ContainsRune(firebaseUIDAlphabet, r) {
			return
		}
	}
	t.Fatalf("Prefix %q is drawn entirely from the Firebase uid alphabet -- it could collide with a real Identity Platform uid", portalaccount.Prefix)
}

func TestNewIdentifier_CarriesThePrefix(t *testing.T) {
	id := portalaccount.NewIdentifier()
	if !strings.HasPrefix(id, portalaccount.Prefix) {
		t.Fatalf("NewIdentifier() = %q, want prefix %q", id, portalaccount.Prefix)
	}
	if len(id) == len(portalaccount.Prefix) {
		t.Fatalf("NewIdentifier() = %q, has no uuid after the prefix", id)
	}
}

func TestNewIdentifier_IsUnique(t *testing.T) {
	first := portalaccount.NewIdentifier()
	second := portalaccount.NewIdentifier()
	if first == second {
		t.Fatal("two calls to NewIdentifier() returned the same value")
	}
}
