package clientauth_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"doula-cloud/api/internal/apierr"
)

func requireErrorCode(t *testing.T, resp *http.Response, want apierr.Code) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	if body.Code != string(want) {
		t.Fatalf("code = %q, want %q", body.Code, want)
	}
}
