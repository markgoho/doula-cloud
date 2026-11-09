package apierr_test

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/apierr"
	"doula-cloud/api/internal/apierrtest"
)

// TestAPIError_CodeSerializesAsAPlainJSONString holds the wire format
// still where #811 left it. Code is typed apierr.Code rather than string
// so a call site can compare against the enumerated constants without a
// conversion, and Code's underlying type is string, so the encoded body
// stays byte-identical to what a plain string field produced -- a quoted
// JSON string under the "code" key, no wrapper object and no number. An
// accidental MarshalJSON on Code, or a change of its underlying type,
// would move the contract every caller in app/ reads.
func TestAPIError_CodeSerializesAsAPlainJSONString(t *testing.T) {
	encoded, err := json.Marshal(apierr.APIError{Code: apierr.CodeConflict, Message: "already sent"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"code":"CONFLICT","message":"already sent"}`
	if string(encoded) != want {
		t.Fatalf("encoded = %s, want %s", encoded, want)
	}

	// And it reads back the same way, so a caller decoding the envelope
	// gets the enumerated value rather than an empty Code. json.Unmarshal
	// directly rather than apierrtest.Decode, which every other test in
	// api/ now uses: this half asserts on the literal bytes above, and
	// there is no *http.Response here to hand the shared reader.
	var out apierr.APIError
	if err := json.Unmarshal([]byte(want), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Code != apierr.CodeConflict {
		t.Fatalf("decoded code = %q, want %q", out.Code, apierr.CodeConflict)
	}
}

func TestCodeForStatus(t *testing.T) {
	tests := []struct {
		status int
		want   apierr.Code
	}{
		{http.StatusBadRequest, apierr.CodeInvalidArgument},
		{http.StatusUnprocessableEntity, apierr.CodeInvalidArgument},
		{http.StatusUnauthorized, apierr.CodeUnauthorized},
		{http.StatusForbidden, apierr.CodeForbidden},
		{http.StatusNotFound, apierr.CodeNotFound},
		{http.StatusPaymentRequired, apierr.CodePaymentRequired},
		{http.StatusConflict, apierr.CodeConflict},
		{http.StatusRequestEntityTooLarge, apierr.CodePayloadTooLarge},
		{http.StatusTooManyRequests, apierr.CodeRateLimited},
		{http.StatusInternalServerError, apierr.CodeInternal},
		{http.StatusTeapot, apierr.CodeInternal},
	}
	for _, tt := range tests {
		if got := apierr.CodeForStatus(tt.status); got != tt.want {
			t.Errorf("CodeForStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestWrite(t *testing.T) {
	t.Run("without details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		apierr.Write(rec, http.StatusConflict, apierr.CodeFailedPrecondition, "website required", nil)

		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
		}
		if got := rec.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		out := apierrtest.Decode(t, rec.Result())
		if out.Code != apierr.CodeFailedPrecondition || out.Message != "website required" || out.Details != nil {
			t.Fatalf("body = %+v, want {FAILED_PRECONDITION website required <nil>}", out)
		}
	})

	t.Run("with details", func(t *testing.T) {
		rec := httptest.NewRecorder()
		apierr.Write(rec, http.StatusBadRequest, apierr.CodeInvalidArgument, "invalid request body",
			map[string]string{"ownUrl": "Enter a web address in the correct format"})

		out := apierrtest.Decode(t, rec.Result())
		if out.Details["ownUrl"] != "Enter a web address in the correct format" {
			t.Fatalf("details = %+v, want ownUrl entry", out.Details)
		}
	})
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	apierr.WriteError(rec, "engagement not found", http.StatusNotFound)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	out := apierrtest.Decode(t, rec.Result())
	if out.Code != apierr.CodeNotFound || out.Message != "engagement not found" {
		t.Fatalf("body = %+v, want {NOT_FOUND engagement not found}", out)
	}
}

func TestWriteFieldError(t *testing.T) {
	rec := httptest.NewRecorder()
	apierr.WriteFieldError(rec, http.StatusBadRequest, apierr.CodeInvalidArgument, "reason", "reason cannot be blank")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	out := apierrtest.Decode(t, rec.Result())
	if out.Code != apierr.CodeInvalidArgument || out.Message != "reason cannot be blank" {
		t.Fatalf("body = %+v, want {INVALID_ARGUMENT reason cannot be blank ...}", out)
	}
	if len(out.Details) != 1 || out.Details["reason"] != "reason cannot be blank" {
		t.Fatalf("details = %+v, want a single reason entry matching message", out.Details)
	}
}

// testPayloadName is the name value TestWriteJSON and TestDecodeJSON's
// round-trip bodies carry -- one literal shared across both so goconst
// does not flag it as three independent copies.
const testPayloadName = "offer"

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	type payload struct {
		Name string `json:"name"`
	}
	apierr.WriteJSON(rec, http.StatusCreated, payload{Name: testPayloadName})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var out payload
	if err := json.UnmarshalRead(rec.Body, &out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if out.Name != testPayloadName {
		t.Fatalf("body = %+v, want {%s}", out, testPayloadName)
	}
}

type reqBody struct {
	Name string `json:"name"`
}

// decoders are the two readers of a request body. They differ only in an
// empty body, so every other case runs against both.
var decoders = map[string]func(http.ResponseWriter, *http.Request, any) bool{
	"DecodeJSON":         apierr.DecodeJSON,
	"DecodeJSONOptional": apierr.DecodeJSONOptional,
}

// decodeWith feeds body to decode and returns what it reported, what it
// decoded and what it wrote.
func decodeWith(t *testing.T, decode func(http.ResponseWriter, *http.Request, any) bool, body io.Reader) (ok bool, out reqBody, rec *httptest.ResponseRecorder) {
	t.Helper()
	rec = httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", body)
	return decode(rec, req, &out), out, rec
}

func TestDecoders_AcceptAValidBodyAndIgnoreKeysNoFieldNames(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"a valid body", `{"name":"` + testPayloadName + `"}`, testPayloadName},
		{"a key whose case differs from the tag", `{"Name":"x"}`, ""},
		{"a key no field names", `{"name":"` + testPayloadName + `","extra":"x"}`, testPayloadName},
	}
	for name, decode := range decoders {
		for _, tc := range cases {
			t.Run(name+" accepts "+tc.name, func(t *testing.T) {
				ok, out, _ := decodeWith(t, decode, strings.NewReader(tc.body))

				if !ok || out.Name != tc.want {
					t.Fatalf("%s = %v, %+v, want true and {%s}", name, ok, out, tc.want)
				}
			})
		}
	}
}

func TestDecoders_RefuseWhatTheyCannotRead(t *testing.T) {
	oversized := `{"name":"` + strings.Repeat("a", apierr.MaxRequestBodyBytes+1) + `"}`
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   apierr.Code
	}{
		{"malformed JSON", `{not json`, http.StatusBadRequest, apierr.CodeInvalidArgument},
		{"a duplicate key", `{"name":"a","name":"b"}`, http.StatusBadRequest, apierr.CodeInvalidArgument},
		{"invalid UTF-8 in a string", "{\"name\":\"a\xffb\"}", http.StatusBadRequest, apierr.CodeInvalidArgument},
		{"data after the closing brace", `{"name":"a"}{"name":"b"}`, http.StatusBadRequest, apierr.CodeInvalidArgument},
		{"an oversized body", oversized, http.StatusRequestEntityTooLarge, apierr.CodePayloadTooLarge},
	}
	for name, decode := range decoders {
		for _, tc := range cases {
			t.Run(name+" refuses "+tc.name, func(t *testing.T) {
				ok, _, rec := decodeWith(t, decode, strings.NewReader(tc.body))

				if ok {
					t.Fatalf("%s returned true, want false", name)
				}
				if rec.Code != tc.wantStatus {
					t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
				}
				if got := apierrtest.Decode(t, rec.Result()); got.Code != tc.wantCode {
					t.Fatalf("code = %q, want %q", got.Code, tc.wantCode)
				}
			})
		}
	}
}

func TestDecodeJSON_RefusalIsTheSharedInvalidBodyMessage(t *testing.T) {
	_, _, rec := decodeWith(t, apierr.DecodeJSON, strings.NewReader(`{not json`))

	if got := apierrtest.Decode(t, rec.Result()); got.Message != "invalid request body" {
		t.Fatalf("body = %+v, want {INVALID_ARGUMENT invalid request body}", got)
	}
}

func TestDecodeJSONOptional_EmptyBodyLeavesTheZeroValueAndReportsSuccess(t *testing.T) {
	ok, out, _ := decodeWith(t, apierr.DecodeJSONOptional, http.NoBody)

	if !ok || out.Name != "" {
		t.Fatalf("DecodeJSONOptional = %v, %+v, want true and the zero value", ok, out)
	}
}
