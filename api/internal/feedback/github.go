package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Issue is the one shape ListIssues reports back: an issue's number and
// its body, which is all IssueWorker's retry check ever reads (the
// marker embedded in it) -- not its title, its labels, or its state,
// none of which that check needs.
type Issue struct {
	Number int
	Body   string
}

// IssueCreator is the GitHub surface the outbox worker needs: creating
// an issue with no labels, adding labels afterward through the separate
// endpoint that needs no more than Issues: write (#1500's research --
// the create-issue endpoint's own labels field is silently dropped for
// a caller without push access), and listing issues since a point in
// time so a retry can find its own earlier issue by the marker embedded
// in its body rather than opening a second one. Closing an issue is
// #1525's addition: what a Client's Erasure does to the issue of a piece
// it destroyed.
//
// A narrow interface this package declares, satisfied by
// GitHubIssueCreator for real and FakeIssueCreator in tests -- the same
// shape client.StripeEraser takes for payments.Client.
type IssueCreator interface {
	ListIssues(ctx context.Context, since time.Time) ([]Issue, error)
	CreateIssue(ctx context.Context, title, body string) (number int, err error)
	AddLabels(ctx context.Context, issueNumber int, labels []string) error
	CloseIssue(ctx context.Context, issueNumber int) error
}

// HTTPDoer is the http.Client seam GitHubIssueCreator takes, the same
// shape sitebuild.HTTPDoer already gives GitHubDispatcher -- both real
// GitHub callers in this codebase are tested against a stub, never the
// network (#1524's own line: never call the real GitHub API with any
// token).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// GitHubIssueCreator is the real IssueCreator: a fine-grained personal
// access token scoped to Issues: write on one private repo (#1500's
// resolution), not a GitHub App and not a widened GITHUB_DISPATCH_TOKEN.
type GitHubIssueCreator struct {
	Client HTTPDoer
	// Token is trimmed on every call, the same reason
	// sitebuild.GitHubDispatcher's is: a Secret Manager value written by
	// hand can carry a trailing newline, and Go's own header validation
	// refuses a header value containing one outright.
	Token string
	// Repo is "owner/repo", e.g. "markgoho/doula-cloud-feedback"
	// (GITHUB_FEEDBACK_REPO).
	Repo string
}

func (c GitHubIssueCreator) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			// coverage:ignore reason: only a value this package built itself could fail to marshal
			return nil, fmt.Errorf("feedback: marshal github request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.github.com/repos/"+c.Repo+path, reader)
	if err != nil {
		// coverage:ignore reason: only a malformed method or URL can fail here
		return nil, fmt.Errorf("feedback: build github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feedback: call github: %w", err)
	}
	return resp, nil
}

// CreateIssue opens an issue carrying no labels field -- #1500's
// research: a fine-grained PAT's labels are silently dropped otherwise
// -- and returns the number GitHub assigned it.
func (c GitHubIssueCreator) CreateIssue(ctx context.Context, title, body string) (int, error) {
	resp, err := c.do(ctx, http.MethodPost, "/issues", map[string]string{"title": title, "body": body})
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return 0, fmt.Errorf("feedback: create issue: github returned %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// coverage:ignore reason: read failure mid-body, not exercised by unit tests
		return 0, fmt.Errorf("feedback: read created issue: %w", err)
	}
	var out struct {
		Number int `json:"number"`
	}
	// json.Unmarshal on an already-read body, not json.NewDecoder -- this
	// is GitHub's response, never our own BFF envelope, so it is not
	// apierr.WriteJSON/DecodeJSON's job to parse (apierr's own
	// TestNoDirectJSONUsage, api/internal/apierr/usage_test.go).
	if err := json.Unmarshal(respBody, &out); err != nil {
		return 0, fmt.Errorf("feedback: decode created issue: %w", err)
	}
	return out.Number, nil
}

// AddLabels adds labels to an issue already created -- the second call
// #1500's research found necessary, since a fine-grained PAT needs no
// extra permission for this endpoint, unlike the ambiguous "push
// access" the create-issue endpoint's own labels field asks for.
func (c GitHubIssueCreator) AddLabels(ctx context.Context, issueNumber int, labels []string) error {
	resp, err := c.do(ctx, http.MethodPost, "/issues/"+strconv.Itoa(issueNumber)+"/labels", map[string][]string{"labels": labels})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("feedback: add labels: github returned %d", resp.StatusCode)
	}
	return nil
}

// CloseIssue closes an issue (#1525). Issues: write covers it, the same
// permission CreateIssue and AddLabels already use, and closing an issue
// that is already closed answers 200 -- so a retry after a failure
// between this call and the outbox row being marked done is harmless.
func (c GitHubIssueCreator) CloseIssue(ctx context.Context, issueNumber int) error {
	resp, err := c.do(ctx, http.MethodPatch, "/issues/"+strconv.Itoa(issueNumber), map[string]string{"state": "closed"})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("feedback: close issue: github returned %d", resp.StatusCode)
	}
	return nil
}

// ListIssues lists every issue touched since since, state=all -- the
// plain REST list, not the Search API, which lags and carries a
// tighter rate limit (#1500's research). One page (up to 100) is read;
// at pilot volume with since narrowed to one feedback item's own
// sent_at, this is never exceeded.
func (c GitHubIssueCreator) ListIssues(ctx context.Context, since time.Time) ([]Issue, error) {
	path := "/issues?state=all&per_page=100&since=" + url.QueryEscape(since.UTC().Format(time.RFC3339))
	resp, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("feedback: list issues: github returned %d", resp.StatusCode)
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// coverage:ignore reason: read failure mid-body, not exercised by unit tests
		return nil, fmt.Errorf("feedback: read issue list: %w", err)
	}
	var out []struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
	}
	// json.Unmarshal, not json.NewDecoder -- see CreateIssue's own comment.
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("feedback: decode issue list: %w", err)
	}
	issues := make([]Issue, len(out))
	for i, one := range out {
		issues[i] = Issue{Number: one.Number, Body: one.Body}
	}
	return issues, nil
}
