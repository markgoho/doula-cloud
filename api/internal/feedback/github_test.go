package feedback_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"doula-cloud/api/internal/feedback"
)

// stubDoer is the same no-network shape sitebuild's own tests use for
// GitHubDispatcher and HTTPProber: it never opens a socket, only
// fabricates the *http.Response CreateIssue/AddLabels/ListIssues read.
// #1524's own line: do not call the real GitHub API with any token.
type stubDoer struct {
	status int
	body   string
	err    error
	// requests records every call, in order, so a test can assert on more
	// than the last one (ListIssues then CreateIssue then AddLabels).
	requests []*http.Request
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.requests = append(s.requests, req)
	if s.err != nil {
		return nil, s.err
	}
	rec := httptest.NewRecorder()
	rec.WriteHeader(s.status)
	_, _ = rec.WriteString(s.body)
	return rec.Result(), nil
}

const testRepo = "markgoho/doula-cloud-feedback"

// testGitHubToken is every stubDoer test's stand-in bearer token -- its
// value is never asserted on past the one trim test above, which builds
// its own literal to prove the trim.
const testGitHubToken = "a-token"

// testNotWorkingLabel is feedback.KindNotWorking's own GitHub label name
// (issue_outbox.go's unexported kindLabel map), repeated here because
// that map is this package's own and not exported for a test to read.
const testNotWorkingLabel = "not working"

// testIdeaOrRequestLabel is feedback.KindIdeaOrRequest's own label name,
// repeated here for the same reason.
const testIdeaOrRequestLabel = "idea or request"

func TestGitHubIssueCreator_CreateIssueSendsNoLabelsAndParsesTheNumber(t *testing.T) {
	doer := &stubDoer{status: http.StatusCreated, body: `{"number": 42}`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: " a-token \n", Repo: testRepo}

	number, err := creator.CreateIssue(t.Context(), "Something is not working: /account", "Route: `/account`\n")
	if err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if number != 42 {
		t.Fatalf("number = %d, want 42", number)
	}

	req := doer.requests[0]
	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", req.Method)
	}
	if want := "https://api.github.com/repos/" + testRepo + "/issues"; req.URL.String() != want {
		t.Errorf("url = %q, want %q", req.URL, want)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer a-token" {
		t.Errorf("Authorization = %q, want trimmed token", got)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if strings.Contains(string(body), `"labels"`) {
		t.Errorf("request body = %s, want no labels field", body)
	}
}

func TestGitHubIssueCreator_CreateIssueNon2xxIsAnError(t *testing.T) {
	doer := &stubDoer{status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: "expired", Repo: testRepo}

	if _, err := creator.CreateIssue(t.Context(), "title", "body"); err == nil {
		t.Fatal("CreateIssue err = nil, want an error for a 401")
	}
}

func TestGitHubIssueCreator_AddLabelsSendsTheKindLabel(t *testing.T) {
	doer := &stubDoer{status: http.StatusOK, body: `[]`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if err := creator.AddLabels(t.Context(), 42, []string{testNotWorkingLabel}); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}

	req := doer.requests[0]
	if want := "https://api.github.com/repos/" + testRepo + "/issues/42/labels"; req.URL.String() != want {
		t.Errorf("url = %q, want %q", req.URL, want)
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if !strings.Contains(string(body), testNotWorkingLabel) {
		t.Errorf("request body = %s, want the label", body)
	}
}

func TestGitHubIssueCreator_ListIssuesParsesNumberAndBody(t *testing.T) {
	doer := &stubDoer{status: http.StatusOK, body: `[{"number":7,"body":"hello"},{"number":9,"body":"world"}]`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	issues, err := creator.ListIssues(t.Context(), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if len(issues) != 2 || issues[0].Number != 7 || issues[1].Body != "world" {
		t.Fatalf("issues = %+v, want two decoded issues", issues)
	}

	req := doer.requests[0]
	if req.Method != http.MethodGet {
		t.Errorf("method = %q, want GET", req.Method)
	}
	if !strings.Contains(req.URL.String(), "state=all") {
		t.Errorf("url = %q, want state=all", req.URL)
	}
	if strings.Contains(req.URL.String(), "search") {
		t.Errorf("url = %q, want the plain list endpoint, not search", req.URL)
	}
}

func TestGitHubIssueCreator_NetworkErrorPropagates(t *testing.T) {
	doer := &stubDoer{err: errors.New("dial tcp: i/o timeout")}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if _, err := creator.CreateIssue(t.Context(), "title", "body"); err == nil {
		t.Fatal("CreateIssue err = nil, want an error when the transport fails")
	}
}

func TestGitHubIssueCreator_CreateIssueMalformedResponseIsAnError(t *testing.T) {
	doer := &stubDoer{status: http.StatusCreated, body: `not json`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if _, err := creator.CreateIssue(t.Context(), "title", "body"); err == nil {
		t.Fatal("CreateIssue err = nil, want an error for a malformed response body")
	}
}

func TestGitHubIssueCreator_AddLabelsNetworkErrorPropagates(t *testing.T) {
	doer := &stubDoer{err: errors.New("dial tcp: i/o timeout")}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if err := creator.AddLabels(t.Context(), 42, []string{testNotWorkingLabel}); err == nil {
		t.Fatal("AddLabels err = nil, want an error when the transport fails")
	}
}

func TestGitHubIssueCreator_AddLabelsNon2xxIsAnError(t *testing.T) {
	doer := &stubDoer{status: http.StatusForbidden, body: `{"message":"Resource not accessible"}`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if err := creator.AddLabels(t.Context(), 42, []string{testNotWorkingLabel}); err == nil {
		t.Fatal("AddLabels err = nil, want an error for a 403")
	}
}

func TestGitHubIssueCreator_ListIssuesNetworkErrorPropagates(t *testing.T) {
	doer := &stubDoer{err: errors.New("dial tcp: i/o timeout")}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if _, err := creator.ListIssues(t.Context(), time.Now()); err == nil {
		t.Fatal("ListIssues err = nil, want an error when the transport fails")
	}
}

func TestGitHubIssueCreator_ListIssuesNon2xxIsAnError(t *testing.T) {
	doer := &stubDoer{status: http.StatusInternalServerError, body: `{"message":"oops"}`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if _, err := creator.ListIssues(t.Context(), time.Now()); err == nil {
		t.Fatal("ListIssues err = nil, want an error for a 500")
	}
}

func TestGitHubIssueCreator_ListIssuesMalformedResponseIsAnError(t *testing.T) {
	doer := &stubDoer{status: http.StatusOK, body: `not json`}
	creator := feedback.GitHubIssueCreator{Client: doer, Token: testGitHubToken, Repo: testRepo}

	if _, err := creator.ListIssues(t.Context(), time.Now()); err == nil {
		t.Fatal("ListIssues err = nil, want an error for a malformed response body")
	}
}
