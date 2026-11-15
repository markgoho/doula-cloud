package contracts

import (
	"errors"
	"fmt"

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
	statuses string
	own      string
}

func resolveView(view string) (listView, bool) {
	switch view {
	case viewNotSigned:
		return listView{statuses: `c.status IN ('draft', 'sent')`, own: orderOldest}, true
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
	desc   bool
	byName bool
}

func resolveSort(order string, view listView) (listSort, bool) {
	if order == orderViewOwn {
		order = view.own
	}
	switch order {
	case orderOldest:
		return listSort{}, true
	case orderNewest:
		return listSort{desc: true}, true
	case orderClient:
		return listSort{byName: true}, true
	}
	return listSort{}, false
}

func (s listSort) keyExpr() string {
	if s.byName {
		return nameKeyExpr
	}
	return "c.stepped_at"
}

func (s listSort) direction() string {
	if s.desc {
		return "DESC"
	}
	return "ASC"
}

func (s listSort) orderBy() string {
	return fmt.Sprintf("%s %s, c.id %s", s.keyExpr(), s.direction(), s.direction())
}

func (s listSort) after(keyParam, idParam string) string {
	op := ">"
	if s.desc {
		op = "<"
	}
	return fmt.Sprintf("(%s, c.id) %s (%s, %s)", s.keyExpr(), op, keyParam, idParam)
}

// listAfter is the position a keyed cursor decodes to: the sort key of the
// last row of the page before, and its id.
type listAfter struct {
	key any
	id  string
}

// decode reads a cursor minted for cursorName, refusing one minted under
// another view or order.
func (s listSort) decode(cursorName string) func(string) (listAfter, error) {
	return func(raw string) (listAfter, error) {
		c, err := pagecursor.DecodeKeyed(raw, cursorName)
		if err != nil {
			return listAfter{}, fmt.Errorf("contracts: list cursor: %w", err)
		}
		if len(c.Keys) != 1 {
			return listAfter{}, errors.New("contracts: list cursor: wrong key count")
		}
		if s.byName {
			return listAfter{key: c.Keys[0], id: c.ID}, nil
		}
		at, err := pagecursor.ParseTimeKey(c.Keys[0])
		if err != nil {
			return listAfter{}, fmt.Errorf("contracts: list cursor: %w", err)
		}
		return listAfter{key: at, id: c.ID}, nil
	}
}

// cursorKeys is the item's sort key as its cursor carries it.
func (s listSort) cursorKeys(item ContractListItem) []string {
	if s.byName {
		return []string{item.sortName}
	}
	return []string{pagecursor.TimeKey(item.steppedAt)}
}
