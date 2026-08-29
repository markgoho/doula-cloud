package sitebuild_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"doula-cloud/api/internal/sitebuild"
)

// livePage is the smallest body that counts as a page: the marker the
// Hugo layout carries and nothing else on the site does.
const livePage = `<main data-practice-page><h1>Rochester Doulas</h1></main>`

// siteBase is the host every prober test is pointed at, named once
// because four tests assert against the URL built from it.
const siteBase = "https://doula.cloud"

func TestHTTPProber_LivePage(t *testing.T) {
	doer := &stubDoer{status: http.StatusOK, body: livePage}
	prober := sitebuild.HTTPProber{Client: doer, BaseURL: siteBase}

	got := prober.Probe(t.Context(), "rochester-doulas")

	if got.State != sitebuild.StateLive {
		t.Fatalf("state = %q, want %q", got.State, sitebuild.StateLive)
	}
	if got.Detail != "" {
		t.Fatalf("detail = %q, want empty", got.Detail)
	}
	if want := "https://doula.cloud/p/rochester-doulas/"; doer.last.URL.String() != want {
		t.Fatalf("requested %q, want %q", doer.last.URL, want)
	}
}

func TestHTTPProber_MissingPage(t *testing.T) {
	prober := sitebuild.HTTPProber{
		Client:  &stubDoer{status: http.StatusNotFound, body: "not found"},
		BaseURL: siteBase,
	}

	got := prober.Probe(t.Context(), "rochester-doulas")

	if got.State != sitebuild.StateFailed {
		t.Fatalf("state = %q, want %q", got.State, sitebuild.StateFailed)
	}
	if !strings.Contains(got.Detail, "404") {
		t.Fatalf("detail = %q, want it to name the status", got.Detail)
	}
}

// The reason the marker exists: a host that serves its own not-found
// page with a 200 must not pass for the Practice's page.
func TestHTTPProber_WrongPageWithA200(t *testing.T) {
	prober := sitebuild.HTTPProber{
		Client:  &stubDoer{status: http.StatusOK, body: "<h1>Page not found</h1>"},
		BaseURL: siteBase,
	}

	got := prober.Probe(t.Context(), "rochester-doulas")

	if got.State != sitebuild.StateFailed {
		t.Fatalf("state = %q, want %q for a 200 that is not the page", got.State, sitebuild.StateFailed)
	}
}

func TestHTTPProber_SiteUnreachable(t *testing.T) {
	prober := sitebuild.HTTPProber{
		Client:  &stubDoer{err: errors.New("dial tcp: i/o timeout")},
		BaseURL: siteBase,
	}

	got := prober.Probe(t.Context(), "rochester-doulas")

	if got.State != sitebuild.StateFailed {
		t.Fatalf("state = %q, want %q", got.State, sitebuild.StateFailed)
	}
	// She is told the site did not answer, not "dial tcp: i/o timeout",
	// which names nothing she can act on.
	if strings.Contains(got.Detail, "dial tcp") {
		t.Fatalf("detail = %q, want words rather than the transport error", got.Detail)
	}
}

func TestGitHubDispatcher_FiresTheNamedEvent(t *testing.T) {
	doer := &stubDoer{status: http.StatusNoContent}
	dispatcher := sitebuild.GitHubDispatcher{Client: doer, Token: "a-token"}

	if err := dispatcher.Dispatch(t.Context()); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	if want := "https://api.github.com/repos/markgoho/doula-cloud/dispatches"; doer.last.URL.String() != want {
		t.Fatalf("posted to %q, want %q", doer.last.URL, want)
	}
	if got := doer.last.Header.Get("Authorization"); got != "Bearer a-token" {
		t.Fatalf("Authorization = %q", got)
	}
	body, err := io.ReadAll(doer.last.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	// The named event type is half of what keeps the trigger ours: the
	// workflow answers to this and to nothing else.
	if !strings.Contains(string(body), sitebuild.DispatchEventType) {
		t.Fatalf("body = %q, want the %q event", body, sitebuild.DispatchEventType)
	}
}

// A lapsed token is the case this has to report rather than swallow --
// the outbox counts the attempt and the page stays pending.
func TestGitHubDispatcher_ReportsARefusal(t *testing.T) {
	dispatcher := sitebuild.GitHubDispatcher{
		Client: &stubDoer{status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`},
		Token:  "a-stale-token",
	}

	err := dispatcher.Dispatch(t.Context())

	if err == nil {
		t.Fatal("Dispatch succeeded on a 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("error = %q, want it to name the status", err)
	}
}

func TestGitHubDispatcher_ReportsATransportFailure(t *testing.T) {
	dispatcher := sitebuild.GitHubDispatcher{
		Client: &stubDoer{err: errors.New("dial tcp: i/o timeout")},
		Token:  "a-token",
	}

	if err := dispatcher.Dispatch(t.Context()); err == nil {
		t.Fatal("Dispatch succeeded when the request never left")
	}
}
