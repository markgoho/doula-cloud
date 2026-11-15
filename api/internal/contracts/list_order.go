package contracts

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"doula-cloud/api/internal/pagecursor"
)

// The views the Contracts list is read in (#2749). The empty name is the
// default: every Contract not yet signed, a Draft or a Sent one.
const (
	viewNotSigned = ""
	viewDraft     = "draft"
	viewSent      = "sent"
	viewSigned    = "signed"
	viewVoided    = "voided"
	viewAll       = "all"
)

// The orders the list is read in. The empty name is the view's own.
const (
	orderViewOwn = ""
	orderOldest  = "oldest"
	orderNewest  = "newest"
	orderClient  = "client"
)

const (
	msgInvalidView  = "Choose which Contracts to show."
	msgInvalidOrder = "Choose how the Contracts are ordered: oldest or newest first, or by Client A to Z."
	msgSearchInGet  = "Send the search in a POST to /contracts/list, not in the address."
	msgSearchLength = "Use 100 characters or fewer for the search."
)

// maxSearchRunes bounds the search: a name is short, and the bound keeps the
// pattern the database matches against short too.
const maxSearchRunes = 100

// listView is one view resolved to the statuses it holds ("" for every
// status) and the order "" means in it.
type listView struct {
	statuses   string
	own        string
	draftsLast bool
}

func resolveView(view string) (listView, bool) {
	switch view {
	case viewNotSigned:
		return listView{statuses: `c.status IN ('draft', 'sent')`, own: orderOldest, draftsLast: true}, true
	case viewDraft:
		return listView{statuses: `c.status = 'draft'`, own: orderOldest}, true
	case viewSent:
		return listView{statuses: `c.status = 'sent'`, own: orderOldest}, true
	case viewSigned:
		return listView{statuses: `c.status = 'signed'`, own: orderNewest}, true
	case viewVoided:
		return listView{statuses: `c.status = 'voided'`, own: orderNewest}, true
	case viewAll:
		return listView{own: orderNewest}, true
	}
	return listView{}, false
}

// nameKeyExpr is the client order's key: personname.Full as the list reads a
// name, lower-cased, so "alma" and "Alma" sort together and ties fall to id.
const nameKeyExpr = `lower(concat_ws(' ', COALESCE(NULLIF(cl.preferred_name, ''), cl.given_name), NULLIF(cl.family_name, '')))`

// listSort is one order resolved to what it sorts on.
type listSort struct {
	desc       bool
	byName     bool
	draftsLast bool
}

func resolveSort(order string, view listView) (listSort, bool) {
	if order == orderViewOwn {
		order = view.own
	}
	switch order {
	case orderOldest:
		return listSort{draftsLast: view.draftsLast}, true
	case orderNewest:
		return listSort{desc: true}, true
	case orderClient:
		return listSort{byName: true}, true
	}
	return listSort{}, false
}

const draftsLastKeyExpr = `(c.status = 'draft'), c.stepped_at`

func (s listSort) keyExpr() string {
	if s.byName {
		return nameKeyExpr
	}
	if s.draftsLast {
		return draftsLastKeyExpr
	}
	return "c.stepped_at"
}

func (s listSort) keyCount() int {
	if s.draftsLast {
		return 2
	}
	return 1
}

func (s listSort) direction() string {
	if s.desc {
		return "DESC"
	}
	return "ASC"
}

func (s listSort) orderBy() string {
	if s.draftsLast {
		return "(c.status = 'draft') ASC, c.stepped_at ASC, c.id ASC"
	}
	return fmt.Sprintf("%s %s, c.id %s", s.keyExpr(), s.direction(), s.direction())
}

// after is the condition for the rows past a cursor, its key values bound
// from parameter first on and its id the one after them.
func (s listSort) after(first int) string {
	op := ">"
	if s.desc {
		op = "<"
	}
	params := make([]string, 0, s.keyCount()+1)
	for i := range s.keyCount() + 1 {
		params = append(params, fmt.Sprintf("$%d", first+i))
	}
	return fmt.Sprintf("(%s, c.id) %s (%s)", s.keyExpr(), op, strings.Join(params, ", "))
}

// listAfter is the position a keyed cursor decodes to: the sort key of the
// last row of the page before, and its id.
type listAfter struct {
	keys []any
	id   string
}

// decode reads a cursor minted for cursorName, refusing one minted under
// another view or order.
func (s listSort) decode(cursorName string) func(string) (listAfter, error) {
	return func(raw string) (listAfter, error) {
		c, err := pagecursor.DecodeKeyed(raw, cursorName)
		if err != nil {
			return listAfter{}, fmt.Errorf("contracts: list cursor: %w", err)
		}
		if len(c.Keys) != s.keyCount() {
			return listAfter{}, errors.New("contracts: list cursor: wrong key count")
		}
		if s.byName {
			return listAfter{keys: []any{c.Keys[0]}, id: c.ID}, nil
		}
		keys := make([]any, 0, len(c.Keys))
		if s.draftsLast {
			isDraft, err := strconv.ParseBool(c.Keys[0])
			if err != nil {
				return listAfter{}, fmt.Errorf("contracts: list cursor: %w", err)
			}
			keys = append(keys, isDraft)
		}
		at, err := pagecursor.ParseTimeKey(c.Keys[len(c.Keys)-1])
		if err != nil {
			return listAfter{}, fmt.Errorf("contracts: list cursor: %w", err)
		}
		return listAfter{keys: append(keys, at), id: c.ID}, nil
	}
}

// cursorKeys is the item's sort key as its cursor carries it.
func (s listSort) cursorKeys(item ContractListItem) []string {
	if s.byName {
		return []string{item.sortName}
	}
	at := pagecursor.TimeKey(item.steppedAt)
	if s.draftsLast {
		return []string{strconv.FormatBool(item.Status == string(StatusDraft)), at}
	}
	return []string{at}
}
