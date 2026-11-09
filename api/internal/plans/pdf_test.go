package plans_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/fixture"
	"doula-cloud/api/internal/reqtest"
)

// requirePDF proves a download is named birth-plan.pdf and is a PDF.
func requirePDF(t *testing.T, pdf reqtest.PDF) {
	t.Helper()
	if !strings.Contains(pdf.Disposition, "birth-plan.pdf") {
		t.Fatalf("Content-Disposition = %q, want it to name birth-plan.pdf", pdf.Disposition)
	}
	if !bytes.HasPrefix(pdf.Body, []byte("%PDF-")) {
		t.Fatalf("body does not start with the PDF magic header: %q", pdf.Body[:min(16, len(pdf.Body))])
	}
}

// TestGetBirthPlanPDFHandler_Success proves Staff can download a rendered
// PDF of a Plan Instance they can already view as JSON (#306).
func TestGetBirthPlanPDFHandler_Success(t *testing.T) {
	e := newEnv(t)
	e.seedBirthPlan()

	requirePDF(t, reqtest.ExpectPDF(t, http.MethodGet, e.instanceURL(birthPlanType, "/pdf"), e.signedIn))
}

// TestGetBirthPlanPDFHandler_Refusals proves the Staff route 404s, 401s or
// refuses what it should: a Care Plan, because the route stays scoped to
// Birth Plan (#306's decision) even though it is mounted generically over
// :planType, the same way GetInstanceHandler's own JSON route is; an
// Engagement with no Plan Instance yet, same as GetInstanceHandler; an
// unattached contractor Doula, to whom ADR-0008's attachment rule applies
// to the PDF the same way it does to the JSON read; and a request with no
// credential, which 401s before ever reaching the handler.
func TestGetBirthPlanPDFHandler_Refusals(t *testing.T) {
	cases := []struct {
		name      string
		opts      []envOption
		planType  string
		seed      func(*env)
		signedOut bool
		want      int
	}{
		{"a Care Plan", nil, carePlanType, func(e *env) { e.seedInstance(carePlanType, f1Fields, `{"f1":"Jamie"}`) }, false, http.StatusNotFound},
		{"no instance yet", nil, birthPlanType, func(*env) {}, false, http.StatusNotFound},
		{"an unattached contractor", []envOption{signedInAs(fixture.Contractor())}, birthPlanType, (*env).seedBirthPlan, false, http.StatusNotFound},
		{"no credential", nil, birthPlanType, func(*env) {}, true, http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, c.opts...)
			c.seed(e)

			session := e.signedIn
			if c.signedOut {
				session = reqtest.Session("")
			}

			reqtest.ExpectStatus(t, http.MethodGet, e.instanceURL(c.planType, "/pdf"), c.want, session)
		})
	}
}

// TestGetBirthPlanPDFHandler_CrossPracticeRejected proves a Staff member
// at Practice A can't download the PDF for an Engagement at Practice B.
func TestGetBirthPlanPDFHandler_CrossPracticeRejected(t *testing.T) {
	e := newEnv(t)
	other := fixture.Engagement(t, e.db, fixture.Practice(t, e.db, fixture.PracticeName("Other Practice")))
	seedInstance(t, e.db, other.ID, birthPlanType, locationNoOptions, `{}`)

	reqtest.ExpectStatus(t, http.MethodGet, e.instanceURLOf(other.ID, birthPlanType, "/pdf"), http.StatusNotFound, e.signedIn)
}

// TestClientGetBirthPlanPDFHandler_Success proves a Client can download a
// rendered PDF of her own Birth Plan (#306).
func TestClientGetBirthPlanPDFHandler_Success(t *testing.T) {
	e := newEnv(t, withPortalLogin())
	e.seedBirthPlan()

	requirePDF(t, reqtest.ExpectPDF(t, http.MethodGet, e.portalURL("/pdf"), e.asClient))
}
