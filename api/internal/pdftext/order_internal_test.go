package pdftext

import "testing"

// #2538: a line stored in the order it is drawn reads back in the order
// it was typed: each right-to-left stretch reversed with its marks, and
// the stretches of a right-to-left line put back in order.
func TestTypedOrder_ReadsADrawnLineAsTyped(t *testing.T) {
	cases := map[string]struct{ drawn, typed string }{
		"left to right only":         {"Ada Lovelace", "Ada Lovelace"},
		"a name after a label":       {"Name: دادحلا ةراس", "Name: سارة الحداد"},
		"marks stay on their letter": {"Name: مسبِ", "Name: بِسم"},
		"a right-to-left line":       {"بب .اا", "اا. بب"},
		"digits in a right-to-left":  {"12 عراش", "شارع 12"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := typedOrder(tc.drawn); got != tc.typed {
				t.Fatalf("typedOrder(%q) = %q, want %q", tc.drawn, got, tc.typed)
			}
		})
	}
}
