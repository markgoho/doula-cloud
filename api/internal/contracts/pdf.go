package contracts

import (
	"bytes"
	"fmt"

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
		return values[key]
	})
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
