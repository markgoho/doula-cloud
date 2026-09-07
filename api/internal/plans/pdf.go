package plans

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-pdf/fpdf"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/clientauth"
	"doula-cloud/api/internal/engagement"
)

// contentTypePDF is the Birth Plan PDF's Content-Type -- this package's
// own constant, since Go doesn't let one package reuse an unexported
// const from another (contracts.contentTypePDF holds the identical
// string).
const contentTypePDF = "application/pdf"

// birthPlanPDFHeading and birthPlanPDFFilename are fixed, not
// parameterized by plan type (#306 scoped this ticket's PDF to Birth
// Plan only -- Care Plan has no Client-facing surface to extend it
// from, so nothing here should be able to render one).
const (
	birthPlanPDFHeading  = "Birth Plan"
	birthPlanPDFFilename = "birth-plan.pdf"
)

// renderPlanPDF renders a Plan Instance's field list and answers to PDF
// bytes (#306) -- built fresh on every request, never stored, since a
// Plan Instance has no "final" event the way a signed Contract does and
// stays mutable for the life of the Engagement. Mirrors
// BirthPlanView.svelte's rendering rules exactly: a section_header field
// starts a new headed group, a blank answer renders as "—" rather than an
// empty line, a checkbox as "Yes"/"No", and a multi_select as its
// selected options joined by ", ".
func renderPlanPDF(heading string, fields []Field, answers Answers) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.MultiCell(0, 10, tr(heading), "", "", false)
	pdf.Ln(4)

	for _, field := range fields {
		if field.Type == fieldTypeSectionHeader {
			pdf.SetFont("Arial", "B", 13)
			pdf.MultiCell(0, 8, tr(field.Label), "", "", false)
			pdf.Ln(2)
			continue
		}
		pdf.SetFont("Arial", "B", 12)
		pdf.MultiCell(0, 6, tr(field.Label), "", "", false)
		pdf.SetFont("Arial", "", 12)
		pdf.MultiCell(0, 6, tr(planFieldAnswerText(field, answers)), "", "", false)
		pdf.Ln(3)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		// coverage:ignore reason: fpdf.Output only fails on an internal encoding error from content this package itself builds, not exercised by unit tests
		return nil, fmt.Errorf("plans: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// planFieldAnswerText formats field's answer the way BirthPlanView.svelte's
// textValue/checkboxValue/selectedOptions do -- an unanswered field never
// renders as a blank line, always as "—".
func planFieldAnswerText(field Field, answers Answers) string {
	switch field.Type {
	case fieldTypeCheckbox:
		if checked, _ := answers[field.ID].(bool); checked {
			return "Yes"
		}
		return "No"
	case fieldTypeMultiSelect:
		raw, _ := answers[field.ID].([]any)
		options := make([]string, 0, len(raw))
		for _, v := range raw {
			if s, ok := v.(string); ok {
				options = append(options, s)
			}
		}
		if len(options) == 0 {
			return "—"
		}
		return strings.Join(options, ", ")
	default: // short_text, long_text, single_select
		if s, ok := answers[field.ID].(string); ok && s != "" {
			return s
		}
		return "—"
	}
}

// serveBirthPlanPDF renders fields/answers as the Birth Plan PDF and
// writes it as a download response. Shared by GetBirthPlanPDFHandler and
// ClientGetBirthPlanPDFHandler.
func serveBirthPlanPDF(w http.ResponseWriter, fields []Field, answers Answers) {
	pdfBytes, err := renderPlanPDF(birthPlanPDFHeading, fields, answers)
	if err != nil {
		// coverage:ignore reason: renderPlanPDF only fails on an internal fpdf encoding error, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentTypePDF)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", birthPlanPDFFilename))
	// coverage:ignore reason: response write failure, not exercised by unit tests
	if _, err := w.Write(pdfBytes); err != nil {
		return
	}
}

// GetBirthPlanPDFHandler streams a rendered PDF of the Birth Plan
// Instance for :engagementId to the calling Staff member -- the
// Practice-side half of #306's "same document, same mechanism" rule,
// sharing loadInstanceForStaff (instance.go) with GetInstanceHandler's
// JSON read so the two can never drift on who can reach an instance.
// planType is read off the URL only to be checked against birthPlanType:
// Care Plan has no Client-facing surface for this PDF to mirror, so this
// handler refuses it the same way a nonexistent instance 404s, rather
// than rendering one. Must be mounted behind staffauth.Middleware.
func GetBirthPlanPDFHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, engagementID, planType, ok := resolveInstanceRequest(w, r)
		if !ok {
			return
		}
		if planType != birthPlanType {
			apierr.WriteError(w, "no plan instance found for this engagement and plan type", http.StatusNotFound)
			return
		}

		fields, answers, _, ok := loadInstanceForStaff(w, r, tx, engagementID, planType)
		if !ok {
			return
		}

		serveBirthPlanPDF(w, fields, answers)
	})
}

// ClientGetBirthPlanPDFHandler mirrors ClientGetBirthPlanHandler,
// rendering the same Birth Plan as a PDF instead of JSON (#306) --
// hardcoded to birth_plan the same way ClientGetBirthPlanHandler is, so
// Care Plan stays structurally unreachable from the Client-portal
// population. Must be mounted behind clientauth.Middleware.
func ClientGetBirthPlanPDFHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, has := clientauth.Tx(r.Context())
		// coverage:ignore reason: clientauth.Middleware always sets a tx before this handler runs
		if !has {
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		engagementID, _ := clientauth.EngagementID(r.Context())

		kind, err := fetchEngagementKind(r.Context(), tx, engagementID)
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests -- clientauth.Middleware already confirmed the row exists
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}
		if !engagement.OffersBirthPlan(engagement.BirthPlanInputs{Kind: kind}) {
			// ADR-0015, mirroring ClientGetBirthPlanHandler: refused at the
			// API independently of the portal's own nav/hub gating (#311).
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}

		fields, answers, _, err := fetchInstance(r.Context(), tx, engagementID, birthPlanType)
		if errors.Is(err, sql.ErrNoRows) {
			apierr.WriteError(w, "no birth plan found for this engagement", http.StatusNotFound)
			return
		}
		if err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		serveBirthPlanPDF(w, fields, answers.nonEmpty())
	})
}
