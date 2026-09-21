package oncall_test

import (
	"slices"
	"testing"

	"doula-cloud/api/internal/oncall"
)

func TestUnstaffedDays(t *testing.T) {
	window := oncall.Window{Start: "2026-10-09", End: "2026-11-13"}
	october := oncall.Window{Start: "2026-10-01", End: "2026-10-31"}

	tests := []struct {
		name       string
		rng        oncall.Window
		effectives []oncall.Window
		want       []oncall.Window
	}{
		{
			name:       "one Doula on call for the whole window leaves no hole",
			rng:        october,
			effectives: []oncall.Window{window},
			want:       nil,
		},
		{
			name: "a primary until week 39 and a backup after it meet with no hole",
			rng:  october,
			effectives: []oncall.Window{
				{Start: "2026-10-09", End: "2026-10-22"},
				{Start: "2026-10-23", End: "2026-11-13"},
			},
			want: nil,
		},
		{
			name: "two narrowings that leave days between them leave exactly those days",
			rng:  october,
			effectives: []oncall.Window{
				{Start: "2026-10-09", End: "2026-10-15"},
				{Start: "2026-10-20", End: "2026-11-13"},
			},
			want: []oncall.Window{{Start: "2026-10-16", End: "2026-10-19"}},
		},
		{
			name:       "a hole outside the chosen range is not reported",
			rng:        oncall.Window{Start: "2026-10-01", End: "2026-10-12"},
			effectives: []oncall.Window{{Start: "2026-10-09", End: "2026-10-15"}},
			want:       nil,
		},
		{
			name:       "a hole at the window's tail is clipped to the range",
			rng:        october,
			effectives: []oncall.Window{{Start: "2026-10-09", End: "2026-10-27"}},
			want:       []oncall.Window{{Start: "2026-10-28", End: "2026-10-31"}},
		},
		{
			name:       "nobody on call at all is the whole overlap",
			rng:        october,
			effectives: nil,
			want:       []oncall.Window{{Start: "2026-10-09", End: "2026-10-31"}},
		},
		{
			name:       "a range that misses the window has nothing to report",
			rng:        oncall.Window{Start: "2026-12-01", End: "2026-12-31"},
			effectives: nil,
			want:       nil,
		},
		{
			name: "a narrowing that ends before the range is passed over",
			rng:  oncall.Window{Start: "2026-10-20", End: "2026-10-31"},
			effectives: []oncall.Window{
				{Start: "2026-10-09", End: "2026-10-12"},
				{Start: "2026-10-20", End: "2026-11-13"},
			},
			want: nil,
		},
		{
			name:       "a narrowing that starts after the range leaves the range's tail, clipped",
			rng:        oncall.Window{Start: "2026-10-01", End: "2026-10-12"},
			effectives: []oncall.Window{{Start: "2026-10-20", End: "2026-11-13"}},
			want:       []oncall.Window{{Start: "2026-10-09", End: "2026-10-12"}},
		},
		{
			name: "overlapping narrowings are one coverage",
			rng:  october,
			effectives: []oncall.Window{
				{Start: "2026-10-15", End: "2026-11-13"},
				{Start: "2026-10-09", End: "2026-10-20"},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := oncall.UnstaffedDays(window, tt.rng, tt.effectives)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("unstaffed = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestWindowOverlaps(t *testing.T) {
	w := oncall.Window{Start: "2026-10-09", End: "2026-11-13"}
	if !w.Overlaps(oncall.Window{Start: "2026-11-13", End: "2026-11-20"}) {
		t.Error("a range starting on the window's last day overlaps it")
	}
	if w.Overlaps(oncall.Window{Start: "2026-11-14", End: "2026-11-20"}) {
		t.Error("a range starting the day after the window does not overlap it")
	}
	if w.Overlaps(oncall.Window{Start: "2026-10-01", End: "2026-10-08"}) {
		t.Error("a range ending the day before the window does not overlap it")
	}
}
