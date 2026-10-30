package contracts

import (
	"bytes"
	"fmt"
	"time"

	"github.com/go-pdf/fpdf"
)

// contentTypePDF is the Signed PDF's Content-Type, shared by the Sign
// transition's store.Put call and both retrieval handlers' responses.
const contentTypePDF = "application/pdf"

// fillProse substitutes every {{merge_field_key}} placeholder in prose
// with its filled value, leaving an unfilled placeholder blank -- mirrors
// app/src/lib/contract.ts's fillProse exactly, since this is what a
// Client actually saw and agreed to when they signed, not the raw
// template prose.
func fillProse(prose string, values MergeFieldValues) string {
	return mergeFieldPattern.ReplaceAllStringFunc(prose, func(match string) string {
		key := mergeFieldPattern.FindStringSubmatch(match)[1]
		return fillValue(key, values[key])
	})
}

// dateMergeFields are the merge fields that hold a calendar day -- the
// Staff form asks for each with a date control, which sends YYYY-MM-DD.
// Mirrors DATE_MERGE_FIELDS in app/src/lib/contractTemplate.ts.
var dateMergeFields = map[string]bool{
	mergeFieldStartDate: true,
	mergeFieldEndDate:   true,
}

// The two date merge fields' keys, as the seeded template writes them.
const (
	mergeFieldStartDate = "engagement_start_date"
	mergeFieldEndDate   = "engagement_end_date"
)

// calendarDayLayout prints a day the way app/src/lib/dates.ts's
// formatCalendarDay does ("Oct 27, 2026"), so the signed PDF says what
// the portal page and the Staff preview say (#2222). A calendar day
// carries no zone, so nothing here can move it.
const calendarDayLayout = "Jan 2, 2006"

// fillValue is what one merge field prints as: a date field holding a
// real YYYY-MM-DD day reads as a calendar day, and anything else -- a
// value typed before the field was a date control, or any other field --
// prints as it was entered.
func fillValue(key, value string) string {
	if !dateMergeFields[key] {
		return value
	}
	day, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return value
	}
	return day.Format(calendarDayLayout)
}

// renderContractPDF renders a Contract's filled prose/merge-field
// snapshot (see fillProse) to PDF bytes -- the permanent record stored
// alongside a Contract's sent -> signed transition. Uses cp1252 (fpdf's
// UnicodeTranslatorFromDescriptor default), which covers the accented
// Latin characters and typographic punctuation (em dash, curly quotes)
// expected in Client names and free-text prose.
func renderContractPDF(filledProse string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 8, tr(filledProse), "", "", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		// coverage:ignore reason: fpdf.Output only fails on an internal encoding error from content this package itself builds, not exercised by unit tests
		return nil, fmt.Errorf("contracts: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}
