package sitebuild

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// DispatchEventType is the one repository_dispatch event name the deploy
// workflow listens for.
//
// Half of what keeps the trigger ours: the workflow answers to this name
// and nothing else, and the token that can send it lives only in Secret
// Manager, readable by the Cloud Run service account. Naming it here
// rather than in configuration keeps the workflow and the sender from
// drifting apart silently, which would look exactly like a deploy that
// stopped happening.
const DispatchEventType = "practice-page-published"

// DispatchRepo is the repository whose deploy workflow this fires. A
// constant for the same reason website.SiteBaseURL is one: there is one
// Doula Cloud site, built from one repository, and a per-environment
// value could only point the deploy at somewhere that does not serve
// Practice pages.
const DispatchRepo = "markgoho/doula-cloud"

// GitHubDispatcher fires the deploy workflow through GitHub's
// repository_dispatch endpoint.
//
// The payload is deliberately empty. GitHub allows one, but the build
// reads the database for every published page anyway (#441), so a
// payload could only carry a second, staler copy of what the build is
// about to read for itself.
type GitHubDispatcher struct {
	Client HTTPDoer
	// Token needs GitHub's "Contents: write" repository permission,
	// which is the narrowest level this endpoint accepts -- the same
	// level a GitHub App would need, which is why #443 chose the
	// simpler credential.
	Token string
}

// Dispatch asks GitHub to run the workflow. A non-2xx is an error, so
// the outbox counts the attempt and Cloud Scheduler's cadence retries:
// a lapsed token then shows up as a page that never leaves pending,
// rather than as a deploy that quietly stopped.
func (d GitHubDispatcher) Dispatch(ctx context.Context) error {
	body := fmt.Sprintf(`{"event_type":%q}`, DispatchEventType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.github.com/repos/"+DispatchRepo+"/dispatches", strings.NewReader(body))
	if err != nil {
		// coverage:ignore reason: only a malformed method or URL can fail here
		return fmt.Errorf("sitebuild: build dispatch request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+d.Token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.Client.Do(req)
	if err != nil {
		return fmt.Errorf("sitebuild: dispatch deploy: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("sitebuild: dispatch deploy: github returned %d", resp.StatusCode)
	}
	return nil
}

// HTTPProber fetches a published page from the live site.
type HTTPProber struct {
	Client HTTPDoer
	// BaseURL is the site the pages are published on, without a
	// trailing slash.
	BaseURL string
}

// Probe reports whether doula.cloud/p/<slug> is there.
//
// A page is live when the request comes back 2xx and the body names the
// Practice's own page rather than the host's not-found page -- checked
// because Firebase Hosting serves a 404 page for an unknown path and a
// status code alone would not tell a missing page from a served one on
// every configuration. Anything else is a failure with a few words the
// Practice can read; the underlying error is not shown to her, because
// "dial tcp: i/o timeout" tells her nothing she can act on.
func (p HTTPProber) Probe(ctx context.Context, slug string) PageProbe {
	url := p.BaseURL + "/p/" + slug + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		// coverage:ignore reason: only a malformed method or URL can fail here
		return PageProbe{State: StateFailed, Detail: "the page address could not be requested"}
	}

	resp, err := p.Client.Do(req)
	if err != nil {
		return PageProbe{State: StateFailed, Detail: "the site did not answer"}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return PageProbe{State: StateFailed,
			Detail: "the site answered " + strconv.Itoa(resp.StatusCode) + " for this page"}
	}

	seen, err := io.ReadAll(io.LimitReader(resp.Body, maxProbeBytes))
	if err != nil {
		// coverage:ignore reason: read failure mid-body, not exercised by unit tests
		return PageProbe{State: StateFailed, Detail: "the page could not be read"}
	}
	if !bytes.Contains(seen, []byte(PageMarker)) {
		return PageProbe{State: StateFailed, Detail: "the address answered, but not with this page"}
	}
	return PageProbe{State: StateLive}
}

// PageMarker is the string every generated Practice page carries and
// nothing else on the site does. Read from the page rather than
// inferred from the status code: a host that serves its own not-found
// page with a 200 would otherwise pass.
const PageMarker = `data-practice-page`

// maxProbeBytes is how much of a page is read before deciding. The
// marker sits in the page's own markup near the top, and a probe has no
// reason to pull a whole document -- still less whatever a
// misconfigured host might send instead.
const maxProbeBytes = 64 << 10
