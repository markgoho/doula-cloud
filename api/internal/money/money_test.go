package money

import "testing"

func TestDollars(t *testing.T) {
	for cents, want := range map[int64]string{0: "$0.00", 5: "$0.05", 4274: "$42.74", 150000: "$1,500.00"} {
		if got := Dollars(cents); got != want {
			t.Errorf("Dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}
