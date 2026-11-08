package authtoken

import (
	"net/http"
	"strings"

	"doula-cloud/api/internal/apierr"
)

// RequireToken trims the token a spend request carried, in place, and
// refuses an empty one with a 400.
func RequireToken(w http.ResponseWriter, token *string) bool {
	*token = strings.TrimSpace(*token)
	if *token == "" {
		apierr.WriteError(w, "token is required", http.StatusBadRequest)
		return false
	}
	return true
}
