package pagecursor

import (
	"net/http"

	"doula-cloud/api/internal/apierr"
)

// FromQuery reads the request's "cursor" query parameter and decodes it
// with decode. An absent cursor is the first page: it returns (nil, true).
// A cursor decode refuses writes a 400 carrying message and returns
// ok=false, the caller's signal to return without doing anything else.
func FromQuery[T any](w http.ResponseWriter, r *http.Request, decode func(string) (T, error), message string) (after *T, ok bool) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return nil, true
	}
	c, err := decode(raw)
	if err != nil {
		apierr.WriteError(w, message, http.StatusBadRequest)
		return nil, false
	}
	return &c, true
}
