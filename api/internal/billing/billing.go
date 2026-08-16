// Package billing owns a Practice's credit_ledger: the append-only record
// of billing credits granted to and consumed by a Practice. This ticket
// (#74) only establishes the schema, its RLS policy, and the signup-bonus
// grant (staffauth.signup inserts that row transactionally with the
// Practice). Reading a derived balance, consuming a credit, and the Stripe
// purchase webhook are separate tickets (see #73) that will add exported
// functions here.
package billing
