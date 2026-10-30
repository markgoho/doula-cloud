package staffauth

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/cardpayments"
)

// PracticeSessionResponse confirms to the frontend which Practice the
// caller landed on -- and, as a side effect of running through
// Middleware, records it as the Staff member's last-used Practice for
// their next login.
//
// IsContractor carries ADR-0008's employment-type axis alongside Roles.
// It exists because #501's contractor Add-a-Client door needs to branch
// on employment type before ever calling client.SearchHandler -- the same
// UX-only mirror of a BFF role gate this endpoint's Roles field already
// is for the Owner/Admin screens that read it.
//
// It is the bare Reader.IsContractor on purpose, and never
// IsAmbientContractor (#1635). The field reports the Employment type as a
// fact and decides no reach in the BFF. The app joins it with Roles to
// get its own isAmbientContractor (app/src/lib/roles.ts), so an Owner or
// an Admin who is a contractor must still read true here.
type PracticeSessionResponse struct {
	PracticeID string `json:"practiceId"`
	// StaffID is the caller's own Staff id at this Practice (#909).
	// `/api/staff/session` already hands the app the same fact -- #437
	// widened SessionInfo with it on the argument that a second round
	// trip to learn what the first could have carried is a round trip
	// nobody needs -- so this is not a new disclosure. It is that same
	// fact carried here, on the one response every route under this
	// Practice already resolves in a `load` before first paint, so a
	// route reads it without a second call. The screen that needs it is
	// the Add-a-Visit picker, which has to know which roster entry is
	// the caller: the same UX-only mirror of a BFF rule Roles and
	// IsContractor already are, the rule here being visit.assignee's --
	// an absent assignee means the caller.
	StaffID      string   `json:"staffId"`
	PracticeName string   `json:"practiceName"`
	Roles        []string `json:"roles"`
	IsContractor bool     `json:"isContractor"`
	// PendingDeletion is #871's own addition: Middleware still lets this
	// one route through while the Practice is mid-deletion (see
	// isPracticeSessionRoute), so app/'s +layout.ts load needs this flag
	// on the response itself to route an Owner to the restore screen and
	// everyone else to an accurate locked message, rather than every
	// other route under this Practice quietly refusing one at a time.
	PendingDeletion bool `json:"pendingDeletion"`
	// SecondFactor is this session's own second-factor fact, the same one
	// `/api/staff/session` carries (#606), carried here so each of the
	// five screens of SecondFactorActs can say before she tries that the
	// act needs a second factor, without a second call (#1532). A UX-only
	// mirror: the act's own seam is what refuses.
	SecondFactor bool `json:"secondFactor"`
	// AgreementDue is #1549's: the material versions an Owner of this
	// Practice has to agree to before she enters it. Middleware lets this
	// one read through while anything is due (isAgreementExemptRoute), so
	// app/'s +layout.ts load can send her to the screen that asks, and the
	// screen can say what changed. Left out when nothing is due, and
	// always for a Staff member who is not an Owner, who is never asked.
	AgreementDue []DueVersion `json:"agreementDue,omitempty"`
	// HasClients is whether the Practice holds any Client yet (#2183).
	// The Staff top bar reads it on every screen, before first paint, to
	// leave out the six sections that can only be empty without one
	// (Clients, Schedule, On call, Invoices, Contracts, Offers). It is the
	// Practice's yes or no for every caller, never narrowed to a
	// contractor's attachments: a contractor is offered work on the
	// Practice's Engagements before she is attached to any, so narrowing
	// it would hide her Offers. It is not the count client.CountHandler
	// withholds from her -- only whether the Practice has begun.
	HasClients bool `json:"hasClients"`
	// ClientsCanPay is cardpayments.ClientsCanPay's fact (#270). It rides
	// on the session so the Invoices list can say "Clients cannot pay this
	// Practice yet" above its rows from the first frame, before the list's
	// own read lands (#2293). The Practice's answer for every caller, as
	// the Engagement read already gives it; PostInvoiceHandler's gate is
	// what refuses.
	ClientsCanPay bool `json:"clientsCanPay"`
	// BillingMode is practices.billing_mode (#271), "stripe" or "by_hand",
	// and left out until the Practice has chosen one. It rides beside
	// ClientsCanPay for the same first-frame reason (#2200): Stripe not
	// being connected stops nobody paying a Practice that bills by hand,
	// so the Invoices list says so only where card payment is the rail.
	BillingMode string `json:"billingMode,omitempty"`
}

// hasClientsQuery names practice_id itself rather than leaving the
// Practice to the clients RLS policy, the same reason client/count.go's
// queries do; EXISTS stops at the first row. A merged-away record is a
// pointer to a survivor that is still counted, so excluding it changes
// nothing but keeps the read the same as the Client list's.
const hasClientsQuery = `SELECT EXISTS (SELECT 1 FROM clients WHERE practice_id = $1 AND merged_into IS NULL)`

// practiceQuery's third column is the billing mode, empty while none is
// chosen.
const practiceQuery = `SELECT name, ` + cardpayments.ClientsCanPayExpr + `, COALESCE(billing_mode::text, '') FROM practices WHERE id = $1`

// PracticeSessionHandler answers GET .../session: which Practice, which
// roles, which employment type. Moved from routes_practice.go by #836 --
// the handler already used nothing but this package's own exports
// (Tx, PracticeID, ReaderFrom), so it belongs here rather than in main.
func PracticeSessionHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tx, _ := Tx(r.Context())
		practiceID, _ := PracticeID(r.Context())

		var name string
		var clientsCanPay bool
		var billingMode string
		if err := tx.QueryRowContext(r.Context(), practiceQuery, practiceID).Scan(&name, &clientsCanPay, &billingMode); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		var hasClients bool
		if err := tx.QueryRowContext(r.Context(), hasClientsQuery, practiceID).Scan(&hasClients); err != nil {
			// coverage:ignore reason: DB query failure, not exercised by unit tests
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		reader, has := ReaderFrom(r.Context())
		if !has {
			// coverage:ignore reason: Middleware always places a Reader on context before this handler runs
			apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
			return
		}

		staffID, _ := StaffID(r.Context())

		resp := PracticeSessionResponse{
			PracticeID:      practiceID,
			StaffID:         staffID,
			PracticeName:    name,
			Roles:           reader.Roles(),
			IsContractor:    reader.IsContractor(), //nolint:forbidigo // the Employment type as a fact, not reach -- see PracticeSessionResponse
			PendingDeletion: pendingDeletionFrom(r.Context()),
			SecondFactor:    SecondFactor(r.Context()),
			AgreementDue:    agreementDueFrom(r.Context()),
			HasClients:      hasClients,
			ClientsCanPay:   clientsCanPay,
			BillingMode:     billingMode,
		}
		apierr.WriteJSON(w, http.StatusOK, resp)
	})
}
