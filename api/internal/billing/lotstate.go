package billing

// LotState is a Credit lot's state: the credit_lot_state enum that
// credit_lots (00176_credit_lot_state.sql) derives from the lot and the
// draws against it. That function is the one derivation of a lot's
// remainder; every reader, Go or ops SQL, reads it rather than summing
// the draws again.
type LotState string

// The two states a lot is in, matching credit_lot_state.
const (
	// LotOpen is a lot with at least one Credit left in it.
	LotOpen LotState = "open"
	// LotDrained is a lot every Credit has left: spent, refunded or
	// forfeited. Nothing draws on it again.
	LotDrained LotState = "drained"
)

// LotAct is one way a Credit leaves a lot, the shape contracts.Transition
// gives a Contract: the credit_ledger origin of the draw row it writes,
// and what the lot must be for the act to draw on it. Every writer of a
// draw row picks its lots through its act's from, and 00176's trigger
// refuses a draw that would take a lot below zero whoever writes it --
// lotstate_test.go's source scan fails a draw writer that skips the
// declaration.
type LotAct struct {
	// Origin is the credit_ledger origin of the row the act writes.
	Origin string
	// Requires is the state the lot must be in.
	Requires LotState
	// RefundableOnly narrows Requires to lots credit_lots calls
	// refundable at the instant asked: purchases inside RefundWindowYears.
	// The window is decided in SQL, where ops_record_refund decides it
	// too, so the quote and the write cannot disagree.
	RefundableOnly bool
}

// ActConsume is ConsumeCredit's act: one Credit, from the oldest open lot.
var ActConsume = LotAct{Origin: "consumption", Requires: LotOpen}

// ActRefund is Refund's act: Credits back from the oldest open purchase
// still inside its window.
var ActRefund = LotAct{Origin: "refund", Requires: LotOpen, RefundableOnly: true}

// ActForfeit is Forfeit's act: everything left in every open lot, at a
// Practice's deletion (ADR-0031).
var ActForfeit = LotAct{Origin: "forfeit", Requires: LotOpen}

// LotActs is every declared act, walked by lotstate_test.go.
var LotActs = []LotAct{ActConsume, ActRefund, ActForfeit}

// admits reports whether a may draw on l.
func (a LotAct) admits(l lot) bool {
	return l.state == a.Requires && (!a.RefundableOnly || l.refundable)
}

// from is the subset of lots a may draw on, in the order given -- oldest
// first, as credit_lots returns them.
func (a LotAct) from(lots []lot) []lot {
	admitted := []lot{}
	for _, l := range lots {
		if a.admits(l) {
			admitted = append(admitted, l)
		}
	}
	return admitted
}
