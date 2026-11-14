package client

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"doula-cloud/api/internal/pagecursor"
)

// The orders the Clients list is read in (#2869). The empty name is the
// view's own: soonest due first, then most recently ended (dueSortJoin).
const (
	orderDue      = ""
	orderName     = "name"
	orderNameDesc = "name-desc"
	orderOwed     = "owed"
)

const (
	msgInvalidOrder        = "The order must be empty, name, name-desc or owed."
	msgOwedOrderContractor = "You contract for this Practice, so you cannot order Clients by what they owe. Money is the Practice's to see."
	msgInvalidCursor       = "invalid cursor"
)

func isKnownOrder(order string) bool {
	switch order {
	case orderDue, orderName, orderNameDesc, orderOwed:
		return true
	}
	return false
}

// nameKeyExpr is the name order's key: personname.Full as the list reads a
// name, lower-cased, so "alma" and "Alma" sort together and ties fall to id.
const nameKeyExpr = `lower(concat_ws(' ', COALESCE(NULLIF(c.preferred_name, ''), c.given_name), NULLIF(c.family_name, '')))`

// owedInvoicesFrom and owedInvoicesWhere are the Invoices a Client owes in
// all (#2874): every open Invoice with no payment on its way, on the live
// (non-voided) Contract of each of her open Engagements. The owed order sorts
// by their sum and the Money cell says their sum and their count, so both read
// this one definition.
const (
	owedInvoicesFrom = `
	    FROM engagements ge
	    JOIN contracts gc ON gc.engagement_id = ge.id AND gc.status <> 'voided'
	    JOIN invoices gi ON gi.contract_id = gc.id`
	owedInvoicesWhere = `ge.status <> 'completed' AND gi.status = 'open' AND gi.payment_processing_since IS NULL`
)

// owedJoin is the owed order's key, joined only when that order is asked for:
// the sum of the Invoices the Client owes in all, 0 for a Client with none.
const owedJoin = `
	LEFT JOIN LATERAL (
	    SELECT COALESCE(sum(gi.amount_cents), 0)::bigint AS owed_cents` + owedInvoicesFrom + `
	    WHERE ge.client_id = c.id AND ` + owedInvoicesWhere + `
	) owed ON true`

// listKeys names the columns the order and the cursor compare, which differ
// between the ambient query and the attached one's outer SELECT.
type listKeys struct {
	name, at, id, owed string
}

var (
	ambientKeys  = listKeys{name: nameKeyExpr, at: "due.sort_at", id: "c.id", owed: "owed.owed_cents"}
	attachedKeys = listKeys{name: "sort_name", at: "sort_at", id: "id", owed: "owed_cents"}
)

// listAfter is the position a keyed cursor decodes to: the key values of the
// last row of the page before, and its id.
type listAfter struct {
	name string
	owed int64
	at   time.Time
	id   string
}

func decodeListCursor(order string) func(string) (listAfter, error) {
	return func(s string) (listAfter, error) {
		c, err := pagecursor.DecodeKeyed(s, order)
		if err != nil {
			return listAfter{}, fmt.Errorf("client: list cursor: %w", err)
		}
		after := listAfter{id: c.ID}
		switch order {
		case orderName, orderNameDesc:
			if len(c.Keys) != 1 {
				return listAfter{}, errors.New("client: list cursor: wrong key count")
			}
			after.name = c.Keys[0]
		case orderOwed:
			if len(c.Keys) != 2 {
				return listAfter{}, errors.New("client: list cursor: wrong key count")
			}
			if after.owed, err = strconv.ParseInt(c.Keys[0], 10, 64); err != nil {
				return listAfter{}, fmt.Errorf("client: list cursor: %w", err)
			}
			if after.at, err = pagecursor.ParseTimeKey(c.Keys[1]); err != nil {
				return listAfter{}, fmt.Errorf("client: list cursor: %w", err)
			}
		default:
			if len(c.Keys) != 1 {
				return listAfter{}, errors.New("client: list cursor: wrong key count")
			}
			if after.at, err = pagecursor.ParseTimeKey(c.Keys[0]); err != nil {
				return listAfter{}, fmt.Errorf("client: list cursor: %w", err)
			}
		}
		return after, nil
	}
}

// cursorKeys is the sort key values of item under order, the cursor's keys.
func (item ListItem) cursorKeys(order string) []string {
	switch order {
	case orderName, orderNameDesc:
		return []string{item.sortName}
	case orderOwed:
		return []string{strconv.FormatInt(item.owedCents, 10), pagecursor.TimeKey(item.sortAt)}
	}
	return []string{pagecursor.TimeKey(item.sortAt)}
}

// withListPage closes a list query with the cursor comparison (joined to what
// precedes it by joiner, AND or WHERE), the order and the page limit. Every
// shape is built from this file's own column names and numeric placeholders;
// `>` ascends and `<` descends over the whole (key, id) tuple, so rows that
// share a key never straddle a page boundary.
func withListPage(query string, args []any, joiner, order string, after *listAfter, k listKeys) (string, []any) {
	var out strings.Builder
	out.WriteString(query)
	bind := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	switch order {
	case orderName, orderNameDesc:
		op, dir := ">", ""
		if order == orderNameDesc {
			op, dir = "<", " DESC"
		}
		if after != nil {
			fmt.Fprintf(&out, " %s (%s, %s) %s (%s, %s)", joiner, k.name, k.id, op, bind(after.name), bind(after.id))
		}
		fmt.Fprintf(&out, " ORDER BY %s%s, %s%s", k.name, dir, k.id, dir)
	case orderOwed:
		if after != nil {
			owed := bind(after.owed)
			fmt.Fprintf(&out, " %s (%s < %s OR (%s = %s AND (%s, %s) > (%s, %s)))", joiner, k.owed, owed, k.owed, owed, k.at, k.id, bind(after.at), bind(after.id))
		}
		fmt.Fprintf(&out, " ORDER BY %s DESC, %s, %s", k.owed, k.at, k.id)
	default:
		if after != nil {
			fmt.Fprintf(&out, " %s (%s, %s) > (%s, %s)", joiner, k.at, k.id, bind(after.at), bind(after.id))
		}
		fmt.Fprintf(&out, " ORDER BY %s, %s", k.at, k.id)
	}
	fmt.Fprintf(&out, " LIMIT %s", bind(pageSize+1))
	return out.String(), args
}
