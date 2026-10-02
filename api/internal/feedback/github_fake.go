package feedback

import (
	"context"
	"slices"
	"sync"
	"time"
)

// FakeIssue is one issue FakeIssueCreator holds -- Issue's two fields
// plus the two a test needs to assert the rest of what #1524's AC asks
// for: the title (kind + route pattern, never free text) and the labels
// AddLabels attached.
type FakeIssue struct {
	Number int
	Title  string
	Body   string
	Labels []string
}

// FakeLabelCall records one AddLabels call, in the order it happened.
type FakeLabelCall struct {
	IssueNumber int
	Labels      []string
}

// FakeIssueCreator is the IssueCreator every test uses -- #1524's own
// line, never call the real GitHub API with any token -- in the
// stripe_fake.go shape: an in-memory stub recording every call so a
// test can assert what would have reached GitHub without it ever
// leaving the process. main.go wires this up whenever
// GITHUB_FEEDBACK_TOKEN is unset, which is every environment but
// Deployed once a human has attached the real token (#1500's own
// resolution: "an in-memory stub the test stack asserts against, never
// a real GitHub call").
type FakeIssueCreator struct {
	mu sync.Mutex

	// Issues holds every issue this fake has "created", keyed by the
	// number it minted, so ListIssues can round-trip a retry's own
	// marker check the same way the real list endpoint would.
	Issues map[int]FakeIssue

	// LabelCalls records every AddLabels call, in order.
	LabelCalls []FakeLabelCall

	CreateIssueErr error
	AddLabelsErr   error
	ListIssuesErr  error

	nextNumber int
}

// NewFakeIssueCreator returns a FakeIssueCreator with no issues yet.
func NewFakeIssueCreator() *FakeIssueCreator {
	return &FakeIssueCreator{Issues: map[int]FakeIssue{}}
}

// CreateIssue records title and body under a freshly minted number, or
// returns CreateIssueErr if a test set one.
func (f *FakeIssueCreator) CreateIssue(_ context.Context, title, body string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.CreateIssueErr != nil {
		return 0, f.CreateIssueErr
	}
	f.nextNumber++
	f.Issues[f.nextNumber] = FakeIssue{Number: f.nextNumber, Title: title, Body: body}
	return f.nextNumber, nil
}

// AddLabels records the call and attaches labels to the issue it names,
// or returns AddLabelsErr if a test set one. A label the issue already
// has is not attached a second time, the real endpoint's own behavior
// (#1587: the worker labels on every attempt, an adopted issue included).
func (f *FakeIssueCreator) AddLabels(_ context.Context, issueNumber int, labels []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.LabelCalls = append(f.LabelCalls, FakeLabelCall{IssueNumber: issueNumber, Labels: labels})
	if f.AddLabelsErr != nil {
		return f.AddLabelsErr
	}
	issue := f.Issues[issueNumber]
	for _, label := range labels {
		if !slices.Contains(issue.Labels, label) {
			issue.Labels = append(issue.Labels, label)
		}
	}
	f.Issues[issueNumber] = issue
	return nil
}

// ListIssues returns every issue this fake holds, or ListIssuesErr if a
// test set one. since is not filtered against -- a test seeds exactly
// the issues it wants a retry to find, so the fake's own contract is
// simpler than the real endpoint's.
func (f *FakeIssueCreator) ListIssues(_ context.Context, _ time.Time) ([]Issue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListIssuesErr != nil {
		return nil, f.ListIssuesErr
	}
	out := make([]Issue, 0, len(f.Issues))
	for _, issue := range f.Issues {
		out = append(out, Issue{Number: issue.Number, Body: issue.Body})
	}
	return out, nil
}
