package payments

import (
	"errors"
	"fmt"
	"strconv"

	"doula-cloud/api/internal/pagecursor"
)

// msgInvalidInvoiceCursor is the refusal for a cursor of another view or
// order, or one that does not decode.
const msgInvalidInvoiceCursor = pagecursor.MsgInvalidCursor

// msgInvalidInvoiceOrder is the refusal for an order the list does not
// offer (#2869).
const msgInvalidInvoiceOrder = "Choose how the Invoices are ordered: sent newest or oldest first, payment due earliest or latest first, or largest or smallest amount first."

// The orders a reader can ask the Invoices list for (#2869). The empty
// name is the view's own order: sent, newest first on the whole book, and
// payment due, earliest first on Unpaid and Overdue.
const (
	orderViewDefault    = ""
	orderSentNewest     = "sent-newest"
	orderSentOldest     = "sent-oldest"
	orderDueEarliest    = "due-earliest"
	orderDueLatest      = "due-latest"
	orderAmountLargest  = "amount-largest"
	orderAmountSmallest = "amount-smallest"
)

// invoiceSort is one order resolved to what it sorts on. Every sorted
// column is NOT NULL (created_at, due_at and amount_cents), so no row
// sorts last for want of a value; ties on the key fall to the Invoice id
// in the same direction as the key.
type invoiceSort struct {
	column string
	desc   bool
	kind   invoiceKeyKind
}

type invoiceKeyKind int

const (
	keyInstantSent invoiceKeyKind = iota
	keyInstantDue
	keyAmount
)

// resolveInvoiceOrder turns the order a request named into the sort it
// reads in, for the view it reads: the empty name is the view's own. An
// unknown name reports false.
func resolveInvoiceOrder(order string, n invoiceNarrowing) (invoiceSort, bool) {
	if order == orderViewDefault {
		if n == narrowAll {
			order = orderSentNewest
		} else {
			order = orderDueEarliest
		}
	}
	// A switch of literals, not a lookup keyed by the request's text: the
	// column that reaches the SQL is one of these constants and nothing the
	// caller sent.
	switch order {
	case orderSentNewest:
		return invoiceSort{column: "i.created_at", desc: true, kind: keyInstantSent}, true
	case orderSentOldest:
		return invoiceSort{column: "i.created_at", kind: keyInstantSent}, true
	case orderDueEarliest:
		return invoiceSort{column: "i.due_at", kind: keyInstantDue}, true
	case orderDueLatest:
		return invoiceSort{column: "i.due_at", desc: true, kind: keyInstantDue}, true
	case orderAmountLargest:
		return invoiceSort{column: "i.amount_cents", desc: true, kind: keyAmount}, true
	case orderAmountSmallest:
		return invoiceSort{column: "i.amount_cents", kind: keyAmount}, true
	}
	return invoiceSort{}, false
}

// cursorOrder is the name a cursor is bound to: the view and the order as
// the request named it, so a cursor minted under one view, or one order,
// is refused under another -- including the empty order, whose meaning
// depends on the view.
func cursorOrder(n invoiceNarrowing, order string) string {
	return n.name() + ":" + order
}

func (s invoiceSort) direction() string {
	if s.desc {
		return "DESC"
	}
	return "ASC"
}

func (s invoiceSort) orderBy() string {
	return s.column + " " + s.direction() + ", i.id " + s.direction()
}

// after is the cursor's condition, with the key and the id bound as $key
// and $id: `>` over the whole (key, id) tuple for an ascending order and
// `<` for a descending one.
func (s invoiceSort) after(key, id int) string {
	op := ">"
	if s.desc {
		op = "<"
	}
	return fmt.Sprintf(`(%s, i.id) %s ($%d, $%d)`, s.column, op, key, id)
}

// invoiceAfter is the position a keyed cursor decodes to: the sort key
// value, typed for the column it is compared with, and the Invoice id.
type invoiceAfter struct {
	key any
	id  string
}

// keyOf is the row's sort key as the cursor carries it.
func (s invoiceSort) keyOf(v PracticeInvoiceView) string {
	switch s.kind {
	case keyInstantDue:
		return pagecursor.TimeKey(v.dueAt)
	case keyAmount:
		return strconv.FormatInt(v.AmountCents, 10)
	case keyInstantSent:
	}
	return pagecursor.TimeKey(v.CreatedAt)
}

// decode reads a cursor minted for cursorName under this sort.
func (s invoiceSort) decode(raw, cursorName string) (invoiceAfter, error) {
	c, err := pagecursor.DecodeKeyed(raw, cursorName)
	if err != nil {
		return invoiceAfter{}, fmt.Errorf("payments: invoice cursor: %w", err)
	}
	if len(c.Keys) != 1 {
		return invoiceAfter{}, errors.New("payments: invoice cursor: wrong key count")
	}
	if s.kind == keyAmount {
		cents, err := strconv.ParseInt(c.Keys[0], 10, 64)
		if err != nil {
			return invoiceAfter{}, fmt.Errorf("payments: invoice cursor: amount: %w", err)
		}
		return invoiceAfter{key: cents, id: c.ID}, nil
	}
	at, err := pagecursor.ParseTimeKey(c.Keys[0])
	if err != nil {
		return invoiceAfter{}, fmt.Errorf("payments: invoice cursor: %w", err)
	}
	return invoiceAfter{key: at, id: c.ID}, nil
}

// cut trims a page fetched with one row past size and mints the
// cursor for the row it ends on when that extra row was there.
func (s invoiceSort) cut(items []PracticeInvoiceView, size int, cursorName string) ([]PracticeInvoiceView, *string) {
	items, hasMore := pagecursor.Trim(items, size)
	if !hasMore {
		return items, nil
	}
	last := items[len(items)-1]
	next := pagecursor.EncodeKeyed(cursorName, []string{s.keyOf(last)}, last.ID)
	return items, &next
}
