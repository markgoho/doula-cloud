package oncall

import "testing"

func TestCompareDue_PutsNoDueDateLastAndTiesAtEqual(t *testing.T) {
	early, late := "2026-10-01", "2026-10-02"

	tests := []struct {
		name string
		a, b *string
		want int
	}{
		{"two with none", nil, nil, 0},
		{"none against a date", nil, &early, 1},
		{"a date against none", &early, nil, -1},
		{"earlier against later", &early, &late, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareDue(tt.a, tt.b); got != tt.want {
				t.Errorf("compareDue = %d, want %d", got, tt.want)
			}
		})
	}
}
