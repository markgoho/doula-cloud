package pdffont

import "testing"

// A noncharacter no face maps stays in the face it follows, or the first
// face when nothing comes before it.
func TestFirstFace_KeepsWhatNoFaceHasWhereItIs(t *testing.T) {
	const none = '\uFFFF'
	if got := firstFace(none, 2); got != 2 {
		t.Fatalf("firstFace after face 2 = %d, want 2", got)
	}
	if got := firstFace(none, -1); got != 0 {
		t.Fatalf("firstFace with nothing before = %d, want 0", got)
	}
}

// An empty cell draws nothing: a CellFormat of width 0 would run to the
// right margin and push the next word onto a line of its own.
func TestCell_DrawsNothingForEmptyText(t *testing.T) {
	d := New()
	x := d.GetX()
	d.cell("", 0, 8)
	if d.GetX() != x {
		t.Fatalf("x moved from %v to %v", x, d.GetX())
	}
}
