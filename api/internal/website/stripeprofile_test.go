package website_test

import (
	"database/sql"
	"testing"

	"doula-cloud/api/internal/testdb"
	"doula-cloud/api/internal/website"
)

// practiceTx opens a transaction scoped to practiceID, the way
// staffauth.Middleware scopes every request, so ReadStripeProfile is
// exercised through the RLS it actually runs behind.
func practiceTx(t *testing.T, db *testdb.DB, practiceID string) *sql.Tx {
	t.Helper()
	tx, err := db.App.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	if _, err := tx.ExecContext(t.Context(),
		`SELECT set_config('app.current_practice_id', $1, true)`, practiceID); err != nil {
		t.Fatalf("set_config: %v", err)
	}
	return tx
}

// TestReadStripeProfile_UndeclaredIsTheGate proves a Practice who has
// not answered comes back undeclared rather than as an error: #442's
// payments handler asks this to decide whether she may be sent to Stripe
// at all, and "she has not answered" is the answer, not a failure.
func TestReadStripeProfile_UndeclaredIsTheGate(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")

	got, err := website.ReadStripeProfile(t.Context(), practiceTx(t, db, practiceID), practiceID)
	if err != nil {
		t.Fatalf("ReadStripeProfile: %v", err)
	}
	if got.Declared {
		t.Fatalf("Declared = true, want false for a Practice with no row")
	}
	if got.URL != "" || got.ProductDescription != "" {
		t.Fatalf("got %+v, want the zero profile", got)
	}
}

// TestReadStripeProfile_HerOwnSite proves the URL she declared is what
// Stripe is told, and that no product description is invented for her --
// #440 never asks a Practice declaring her own site for one.
func TestReadStripeProfile_HerOwnSite(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")
	seedWebsite(t, db, practiceID, "own", "https://facebook.com/rochester-doulas")

	got, err := website.ReadStripeProfile(t.Context(), practiceTx(t, db, practiceID), practiceID)
	if err != nil {
		t.Fatalf("ReadStripeProfile: %v", err)
	}
	if !got.Declared {
		t.Fatalf("Declared = false, want true")
	}
	if got.URL != "https://facebook.com/rochester-doulas" {
		t.Fatalf("URL = %q, want the address she declared", got.URL)
	}
	if got.ProductDescription != "" {
		t.Fatalf("ProductDescription = %q, want empty", got.ProductDescription)
	}
}

// TestReadStripeProfile_HerHostedPage proves the other answer resolves
// to the address the page is published at, composed from the slug 00046
// minted once and never recomputes.
func TestReadStripeProfile_HerHostedPage(t *testing.T) {
	db := testdb.New(t)
	practiceID := seedPractice(t, db, "Rochester Doulas")
	if _, err := db.Admin.ExecContext(t.Context(),
		`INSERT INTO practice_websites (practice_id, mode, service_description, cancellation_policy, slug)
		 VALUES ($1, 'hosted', $2, $3, $4)`,
		practiceID, "Birth and postpartum support across Monroe County.", policyText, firstSlug,
	); err != nil {
		t.Fatalf("seed hosted page: %v", err)
	}

	got, err := website.ReadStripeProfile(t.Context(), practiceTx(t, db, practiceID), practiceID)
	if err != nil {
		t.Fatalf("ReadStripeProfile: %v", err)
	}
	if !got.Declared {
		t.Fatalf("Declared = false, want true")
	}
	if got.URL != "https://doula.cloud/p/"+firstSlug {
		t.Fatalf("URL = %q, want her published page's address", got.URL)
	}
	if got.ProductDescription != "Birth and postpartum support across Monroe County." {
		t.Fatalf("ProductDescription = %q, want what she wrote on her page", got.ProductDescription)
	}
}

// TestHostedPageURL pins the one place the site's address and the slug
// are joined. hugo/hugo.toml carries the same host as its baseURL, and
// the two have to agree: this is what Stripe is told, and Hugo is what
// makes something answer at it.
func TestHostedPageURL(t *testing.T) {
	if got := website.HostedPageURL("rochester-doulas"); got != "https://doula.cloud/p/rochester-doulas" {
		t.Fatalf("HostedPageURL = %q", got)
	}
}
