// Package money writes an amount of cents the way a person reads it.
package money

import (
	"fmt"

	"github.com/dustin/go-humanize"
)

// Dollars renders cents as dollars with a thousands separator and two
// decimal places: 150000 is "$1,500.00". cents is never negative here.
func Dollars(cents int64) string {
	return fmt.Sprintf("$%s.%02d", humanize.Comma(cents/100), cents%100)
}
