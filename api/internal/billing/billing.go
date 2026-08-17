// Package billing owns a Practice's credit_ledger: the append-only record
// of billing credits granted to and consumed by a Practice. #74 established
// the schema, its RLS policy, and the signup-bonus grant (staffauth.signup
// inserts that row transactionally with the Practice). #75 adds Balance,
// the derived-balance read used by GetBalanceHandler and available to other
// packages. Consuming a credit and the Stripe purchase webhook are separate
// tickets (see #73) that will add exported functions here.
package billing
