package client

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"doula-cloud/api/internal/staffauth"
)

const owedTotalsQueryTemplate = `SELECT ge.client_id, sum(gi.amount_cents)::bigint, count(*)` + owedInvoicesFrom + `
	 WHERE ge.client_id IN (%s) AND ` + owedInvoicesWhere + `
	 GROUP BY ge.client_id`

// attachOwedTotals fills OwedCents and OwedInvoiceCount for every row of list
// in one query keyed on the page's Client ids (#2874): what the Client owes in
// all, by the same Invoices the owed order sorts by (owedInvoicesFrom). A
// contractor Doula is never told money (ADR-0008), so her rows carry neither.
func attachOwedTotals(ctx context.Context, tx *sql.Tx, list []ListItem, reader staffauth.Reader) error {
	if len(list) == 0 || reader.IsAmbientContractor() {
		return nil
	}
	placeholders := make([]string, len(list))
	args := make([]any, len(list))
	for i, item := range list {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = item.ClientID
	}
	query := fmt.Sprintf(owedTotalsQueryTemplate, strings.Join(placeholders, ", ")) //nolint:gosec // the one interpolated value is numeric placeholder indices this function computed; the Client ids ride args as bind parameters
	rows, err := tx.QueryContext(ctx, query, args...)                               //nolint:gosec // query is the template above with placeholder indices only
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		return fmt.Errorf("client: owed totals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type owed struct {
		cents int64
		count int
	}
	byClient := map[string]owed{}
	for rows.Next() {
		var clientID string
		var o owed
		if err := rows.Scan(&clientID, &o.cents, &o.count); err != nil {
			// coverage:ignore reason: row scan failure, not exercised by unit tests
			return fmt.Errorf("client: scan owed totals: %w", err)
		}
		byClient[clientID] = o
	}
	if err := rows.Err(); err != nil {
		// coverage:ignore reason: row iteration failure, not exercised by unit tests
		return fmt.Errorf("client: iterate owed totals: %w", err)
	}
	for i := range list {
		o := byClient[list[i].ClientID]
		list[i].OwedCents = &o.cents
		list[i].OwedInvoiceCount = &o.count
	}
	return nil
}
