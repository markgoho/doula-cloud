package clientstanding_test

import (
	"regexp"
	"strings"
	"testing"

	"doula-cloud/api/internal/clientstanding"
	"doula-cloud/api/internal/lifecycletest"
)

// internalDir is api/internal, the tree every scan below walks.
const internalDir = ".."

// gatedRoutes names, for each route file, the Acts it must consult. It is
// the declaration TestActs_EachRouteConsultsItsAct holds each file to, the
// same shape contracts' lifecycle guardrail takes: a route that stops
// calling its Act, or never calls one, fails here rather than in a
// status-code test that would pass if a column comparison came back.
var gatedRoutes = map[string][]string{
	"client/edit.go":               {"ActEdit"},
	"client/erase.go":              {"ActErase"},
	"client/merge.go":              {"ActMerge", "ActMergeInto"},
	"engagementrequest/request.go": {"ActRequestEngagement"},
	"engagementrequest/approve.go": {"ActApproveRequest"},
	"visit/create.go":              {"ActAddVisit"},
	"message/create.go":            {"ActSendMessage"},
	"contracts/contract.go":        {"ActCreateContract"},
	"contracts/send.go":            {"ActSendContract"},
	"plans/instance.go":            {"ActStartPlan"},
	"portalinvite/invite.go":       {"ActInviteToPortal"},
	"offer/create.go":              {"ActOfferWork"},
	"payments/invoice.go":          {"ActCreateInvoice"},
}

// TestActs_EachRouteConsultsItsAct proves every declared Act has a route
// and every route still calls the Act declared for it.
func TestActs_EachRouteConsultsItsAct(t *testing.T) {
	named := 0
	for file, acts := range gatedRoutes {
		src := readSource(t, file)
		for _, act := range acts {
			named++
			if !strings.Contains(src, "clientstanding."+act+".") {
				t.Errorf("%s does not call clientstanding.%s -- a write on or under a Client record must consult the gate declared for it", file, act)
			}
		}
	}
	if named != len(clientstanding.Acts) {
		t.Errorf("gatedRoutes names %d Acts, clientstanding.Acts declares %d -- a new Act needs the route that consults it named here", named, len(clientstanding.Acts))
	}
}

// writesUnderAClient are the statements that write a Client record or add
// something beneath it. A file holding one must consult some Act.
var writesUnderAClient = []string{
	"UPDATE clients SET",
	"redact_client_record(",
	"INSERT INTO engagement_requests",
	"INSERT INTO engagements",
	"INSERT INTO visits",
	"INSERT INTO messages",
	"INSERT INTO contracts",
	"INSERT INTO plan_instances",
	"INSERT INTO client_portal_users",
	"INSERT INTO engagement_offers",
	"INSERT INTO invoices",
}

// gatedElsewhere are files that hold such a statement and are reached
// only through a route that has already consulted its Act, or are not a
// route at all.
var gatedElsewhere = map[string]string{
	"client/db.go":       "updateRecord is EditHandler's write, and edit.go consults ActEdit before it",
	"testdb/seed.go":     "test scaffolding: seeds rows directly, outside any route",
	"fixture/fixture.go": "test scaffolding: builds records directly, outside any route",
}

// TestWrites_EveryWriterConsultsTheGate is self-updating: a new file that
// writes a Client record or adds beneath one fails here the moment it is
// added, until it consults an Act or says why it need not.
func TestWrites_EveryWriterConsultsTheGate(t *testing.T) {
	scanned := 0
	walkSources(t, func(file, src string) {
		writes := ""
		for _, w := range writesUnderAClient {
			if strings.Contains(src, w) {
				writes = w
				break
			}
		}
		if writes == "" {
			return
		}
		scanned++
		if _, ok := gatedElsewhere[file]; ok {
			return
		}
		if !strings.Contains(src, "clientstanding.Act") {
			t.Errorf("%s writes %q but consults no clientstanding Act -- declare one and call it before the write, or name the file in gatedElsewhere with the reason", file, writes)
		}
	})
	if scanned == 0 {
		t.Fatal("no file under api/internal writes a Client record -- did the scan stop finding the statements it looks for?")
	}
	for file := range gatedElsewhere {
		readSource(t, file)
	}
}

// columnStanding is a predicate that asks merged_into or erased_at
// directly instead of the standing they derive.
var columnStanding = regexp.MustCompile(`\b(erased_at|merged_into)\s+IS\s+(NOT\s+)?NULL\b`)

// askedOfTheColumns are files allowed to ask the columns, each with why.
// It is empty: every reader asks the standing.
var askedOfTheColumns = map[string]string{}

// TestReaders_AskTheStanding fails a reader that goes back to the two
// columns instead of the one derivation, so a difference between two
// readers is a named choice of standings rather than an accident of which
// column each remembered. Comments are not reads, so they are skipped.
func TestReaders_AskTheStanding(t *testing.T) {
	walkSources(t, func(file, src string) {
		if _, ok := askedOfTheColumns[file]; ok {
			return
		}
		for i, line := range strings.Split(src, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if m := columnStanding.FindString(line); m != "" {
				t.Errorf("%s:%d asks %q -- ask clients.standing (clientstanding's four values) instead", file, i+1, m)
			}
		}
	})
	for file := range askedOfTheColumns {
		readSource(t, file)
	}
}

// walkSources calls fn with every non-test Go file under api/internal,
// by its path relative to it.
func walkSources(t *testing.T, fn func(file, src string)) {
	t.Helper()
	for file, src := range lifecycletest.Sources(t, internalDir, ".") {
		fn(file, src)
	}
}

func readSource(t *testing.T, file string) string {
	t.Helper()
	return lifecycletest.Read(t, internalDir, file)
}
