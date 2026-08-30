package pagecursor_test

import (
	"testing"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	at := time.Date(2026, 8, 29, 14, 5, 6, 123456789, time.UTC)
	const id = "11111111-1111-1111-1111-111111111111"

	got, err := pagecursor.Decode(pagecursor.Encode(at, id))
	if err != nil {
		t.Fatalf("Decode after Encode: %v", err)
	}
	if !got.At.Equal(at) {
		t.Errorf("At = %v, want %v", got.At, at)
	}
	if got.ID != id {
		t.Errorf("ID = %q, want %q", got.ID, id)
	}
}

// A cursor is opaque on purpose: nothing about the encoded form should let
// a caller read a position out of it or write one in.
func TestEncodeIsOpaque(t *testing.T) {
	at := time.Date(2026, 8, 29, 14, 5, 6, 0, time.UTC)
	encoded := pagecursor.Encode(at, "abc")
	if encoded == at.Format(time.RFC3339Nano)+"|abc" {
		t.Error("Encode returned the raw pair, not an encoded one")
	}
}

func TestDecodeRejectsMalformed(t *testing.T) {
	for name, input := range map[string]string{
		"not base64":     "not-base-64-!!",
		"no separator":   "MjAyNi0wOC0yOVQxNDowNTowNlo", // a bare timestamp
		"bad timestamp":  "bm90LWEtdGltZXxhYmM=",        // "not-a-time|abc"
		"empty":          "IA==",                        // " "
		"separator only": "fA==",                        // "|"
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := pagecursor.Decode(input); err == nil {
				t.Errorf("Decode(%q) = nil error, want an error", input)
			}
		})
	}
}
