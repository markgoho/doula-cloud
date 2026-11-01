package main

import (
	"testing"

	"doula-cloud/api/internal/legal"
)

// TestLegalHistory_ReadsDepsHistory is #1833's wiring: the routes read
// Deps.LegalDocuments when main sets it (the end-to-end stack's history,
// with a version the shipped one lacks), and the shipped history when it
// does not (Cloud Run).
func TestLegalHistory_ReadsDepsHistory(t *testing.T) {
	terms := legal.Terms
	terms.Versions = append(append([]legal.Version(nil), legal.Terms.Versions...),
		legal.Version{Effective: "2026-12-01", Material: true, Change: "Says what a test changed."})
	appended := []legal.Document{terms, legal.Privacy}

	deps := testDeps()
	deps.LegalDocuments = appended
	if got := termsIn(deps.legalHistory()); len(got.Versions) != len(legal.Terms.Versions)+1 {
		t.Fatalf("Terms read from the stack's history has %d versions, want %d", len(got.Versions), len(legal.Terms.Versions)+1)
	}
	if got := termsIn(testDeps().legalHistory()); len(got.Versions) != len(legal.Terms.Versions) {
		t.Fatalf("Terms read with no history set has %d versions, want the shipped %d", len(got.Versions), len(legal.Terms.Versions))
	}
}
