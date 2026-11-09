package authtoken

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"doula-cloud/api/internal/apierr"
)

// DecodeSpend decodes the body of a spend request and refuses one whose
// token is blank, trimming the token in place. token picks the token field
// out of the decoded request. It writes the 400 itself.
func DecodeSpend[T any](w http.ResponseWriter, r *http.Request, token func(*T) *string) (req T, ok bool) {
	if !apierr.DecodeJSON(w, r, &req) {
		return req, false
	}
	t := token(&req)
	*t = strings.TrimSpace(*t)
	if *t == "" {
		apierr.WriteError(w, "token is required", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// MsgLinkUnusable is what a spend answers for a token that is unknown,
// already spent, expired or minted for another purpose, so the four cannot
// be told apart.
const MsgLinkUnusable = "this link is invalid or has expired -- ask for a new one"

// SpendOrRefuse spends token for purpose and returns the identity it was
// minted for. It writes the 400 LINK_UNUSABLE for a token Spend refuses and
// the 500 for a failed lookup, so a caller that gets ok false has answered.
func SpendOrRefuse(w http.ResponseWriter, r *http.Request, q Querier, token string, purpose Purpose, now time.Time) (identityUID string, ok bool) {
	identityUID, err := Spend(r.Context(), q, token, purpose, now)
	if errors.Is(err, ErrInvalid) {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeLinkUnusable, MsgLinkUnusable, nil)
		return "", false
	}
	if err != nil {
		// coverage:ignore reason: DB query failure, not exercised by unit tests
		apierr.WriteError(w, apierr.MsgInternalError, http.StatusInternalServerError)
		return "", false
	}
	return identityUID, true
}
