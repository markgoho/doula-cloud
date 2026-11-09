package reqtest_test

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"doula-cloud/api/internal/reqtest"
)

type echo struct {
	Method      string `json:"method"`
	Cookie      string `json:"cookie"`
	ContentType string `json:"contentType"`
	Custom      string `json:"custom"`
	Body        string `json:"body"`
}

func echoFrom(t *testing.T, send func(*testing.T, string, ...reqtest.Option) *http.Response, url string, opts ...reqtest.Option) echo {
	t.Helper()
	resp := send(t, url, opts...)
	defer resp.Body.Close()
	return reqtest.Decode[echo](t, resp)
}

func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.MarshalWrite(w, echo{
			Method:      r.Method,
			Cookie:      r.Header.Get("Cookie"),
			ContentType: r.Header.Get("Content-Type"),
			Custom:      r.Header.Get("X-Custom"),
			Body:        string(body),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestVerbs_SendTheirMethod(t *testing.T) {
	srv := echoServer(t)
	cases := map[string]func(*testing.T, string, ...reqtest.Option) *http.Response{
		http.MethodGet:    reqtest.Get,
		http.MethodPost:   reqtest.Post,
		http.MethodPut:    reqtest.Put,
		http.MethodDelete: reqtest.Delete,
	}
	for method, send := range cases {
		t.Run(method, func(t *testing.T) {
			got := echoFrom(t, send, srv.URL)
			if got.Method != method {
				t.Fatalf("method = %q, want %q", got.Method, method)
			}
		})
	}
}

func TestSession_CarriesTheCookieOnlyWhenNonEmpty(t *testing.T) {
	srv := echoServer(t)

	signedIn := echoFrom(t, reqtest.Get, srv.URL, reqtest.Session("tok"))
	if signedIn.Cookie != "__session=tok" {
		t.Fatalf("cookie = %q, want __session=tok", signedIn.Cookie)
	}

	signedOut := echoFrom(t, reqtest.Get, srv.URL, reqtest.Session(""))
	if signedOut.Cookie != "" {
		t.Fatalf("cookie = %q, want none", signedOut.Cookie)
	}
}

func TestJSON_SendsTheMarshaledBodyAsJSON(t *testing.T) {
	srv := echoServer(t)

	got := echoFrom(t, reqtest.Post, srv.URL, reqtest.JSON(map[string]string{"reason": "why"}))

	if got.ContentType != "application/json" || got.Body != `{"reason":"why"}` {
		t.Fatalf("got content type %q body %q", got.ContentType, got.Body)
	}
}

func TestRawJSON_SendsTheBytesUnderAJSONContentType(t *testing.T) {
	srv := echoServer(t)

	got := echoFrom(t, reqtest.Post, srv.URL, reqtest.RawJSON([]byte("not json")))

	if got.ContentType != "application/json" || got.Body != "not json" {
		t.Fatalf("got content type %q body %q", got.ContentType, got.Body)
	}
}

func TestBody_SendsThePayloadUnderTheGivenContentType(t *testing.T) {
	srv := echoServer(t)

	got := echoFrom(t, reqtest.Put, srv.URL, reqtest.Body("text/plain", []byte("not json")))

	if got.ContentType != "text/plain" || got.Body != "not json" {
		t.Fatalf("got content type %q body %q", got.ContentType, got.Body)
	}
}

func TestHeader_SetsTheHeader(t *testing.T) {
	srv := echoServer(t)

	got := echoFrom(t, reqtest.Get, srv.URL, reqtest.Header("X-Custom", "yes"))

	if got.Custom != "yes" {
		t.Fatalf("X-Custom = %q, want yes", got.Custom)
	}
}

func TestHeaderIf_SetsTheHeaderOnlyWhenTheConditionHolds(t *testing.T) {
	srv := echoServer(t)

	with := echoFrom(t, reqtest.Get, srv.URL, reqtest.HeaderIf(true, "X-Custom", "yes"))
	without := echoFrom(t, reqtest.Get, srv.URL, reqtest.HeaderIf(false, "X-Custom", "yes"))

	if with.Custom != "yes" || without.Custom != "" {
		t.Fatalf("X-Custom = %q when true, %q when false", with.Custom, without.Custom)
	}
}

func TestReadBody_ReturnsTheBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))
	t.Cleanup(srv.Close)

	resp := reqtest.Get(t, srv.URL)
	defer resp.Body.Close()
	if got := string(reqtest.ReadBody(t, resp)); got != "hello" {
		t.Fatalf("body = %q, want hello", got)
	}
}

func TestRequireStatus_PassesOnAMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	resp := reqtest.Get(t, srv.URL)
	defer resp.Body.Close()
	reqtest.RequireStatus(t, resp, http.StatusTeapot)
}

func TestDecodeStatus_ChecksTheStatusThenReadsTheBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.MarshalWrite(w, echo{Method: "made"})
	}))
	t.Cleanup(srv.Close)

	gotResp := reqtest.Get(t, srv.URL)
	defer gotResp.Body.Close()
	got := reqtest.DecodeStatus[echo](t, gotResp, http.StatusCreated)
	if got.Method != "made" {
		t.Fatalf("method = %q, want made", got.Method)
	}
}

func TestExpect_ReturnsTheDecodedBodyOfTheExpectedStatus(t *testing.T) {
	srv := echoServer(t)
	got := reqtest.Expect[echo](t, http.MethodPut, srv.URL, http.StatusOK,
		reqtest.Session("tok"), reqtest.JSON(map[string]string{"a": "b"}))
	if got.Method != http.MethodPut || got.Body != `{"a":"b"}` {
		t.Fatalf("echo = %+v, want the PUT and its JSON body back", got)
	}
}

func TestExpectBody_ReturnsTheRawBytes(t *testing.T) {
	srv := echoServer(t)
	body := reqtest.ExpectBody(t, http.MethodGet, srv.URL, http.StatusOK)
	if !strings.Contains(string(body), `"method":"GET"`) {
		t.Fatalf("body = %s, want the echo of a GET", body)
	}
}

func TestExpectStatus_PassesOnTheExpectedStatus(t *testing.T) {
	srv := echoServer(t)
	reqtest.ExpectStatus(t, http.MethodDelete, srv.URL, http.StatusOK)
}

func TestExpectPDF_ReturnsTheDispositionAndTheBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `attachment; filename="plan.pdf"`)
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))
	t.Cleanup(srv.Close)

	got := reqtest.ExpectPDF(t, http.MethodGet, srv.URL)

	if got.Disposition != `attachment; filename="plan.pdf"` || string(got.Body) != "%PDF-1.4" {
		t.Fatalf("pdf = %+v, want the plan.pdf download and its bytes", got)
	}
}

func TestRequirePage_PassesOnAFullAndOnALastPage(t *testing.T) {
	cursor := "next"
	reqtest.RequirePage(t, "first", 30, true, &cursor, 30, true)
	reqtest.RequirePage(t, "last", 1, false, nil, 1, false)
}
