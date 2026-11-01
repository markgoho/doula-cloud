package contracts

import (
	"bytes"
	"fmt"
	"time"

	"doula-cloud/api/internal/pdffont"
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

// signConsentLine is the checkbox the Client checks on the signing form,
// word for word: SIGN_CONSENT_LABEL in app/src/lib/clientRegister.ts,
// which TestSignConsentLine_IsTheSigningFormsCheckbox holds this to. The
// one checkbox is both her attestation that she read the Contract and her
// consent to sign it electronically, so the PDF quotes it once (#2214).
const signConsentLine = "I have read this contract and I am signing it electronically"

// signedOnLayout prints the signing instant the way app/src/lib/dates.ts's
// formatInstantWithClock does ("Oct 27, 2026, 2:05pm"), followed by the
// zone's abbreviation. The page reads an instant in its reader's own zone;
// a printed Contract has no reader's zone, so it names the Practice's
// (ADR-0036), and anyone holding the paper elsewhere can still tell the
// hour it was signed.
const signedOnLayout = "Jan 2, 2006, 3:04pm MST"

// signature is what the Sign transition prints below a Contract's prose:
// the full legal name as the Client typed it, and the instant stored as
// signed_at, already moved into the Practice's zone.
type signature struct {
	fullLegalName string
	signedAt      time.Time
}

// lines are the signature block's printed lines, in order: that it was
// signed electronically and by whom, when, and what she confirmed.
func (s signature) lines() []string {
	return []string{
		"Signed electronically by: " + s.fullLegalName,
		"Signed on: " + s.signedAt.Format(signedOnLayout),
		"Confirmed: " + signConsentLine,
	}
}

// renderContractPDF renders a Contract's filled prose/merge-field
// snapshot (see fillProse), then its signature block, to PDF bytes -- the
// permanent record stored alongside a Contract's sent -> signed
// transition. It is rendered once, at signing, and never again: a PDF
// stored before #2214 added the signature block keeps the prose it was
// stored with rather than being re-rendered, because the stored file is
// the record of what was produced at that moment (#71). Text goes
// through pdffont's embedded Unicode faces, so a name or prose in any
// script prints as typed (#2359).
func renderContractPDF(filledProse string, sig signature) ([]byte, error) {
	pdf := pdffont.New()
	pdf.Paragraph(pdffont.Regular, 12, 8, filledProse)
	pdf.Ln(8)
	for _, line := range sig.lines() {
		pdf.Paragraph(pdffont.Regular, 12, 8, line)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		// coverage:ignore reason: fpdf.Output only fails on an internal encoding error from content this package itself builds, not exercised by unit tests
		return nil, fmt.Errorf("contracts: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}
