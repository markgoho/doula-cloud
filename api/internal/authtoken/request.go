package authtoken

import (
	"net/http"
	"strings"

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
