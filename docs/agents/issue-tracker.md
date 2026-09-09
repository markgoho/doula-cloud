# Issue tracker: GitHub

Issues and specs for this repo live as GitHub issues. Use the `gh` CLI for all operations.

## Conventions

- **Create an issue**: `gh issue create --title "..." --body "..."`. Use a heredoc for multi-line bodies.
- **Read an issue**: `gh issue view <number> --comments`, filtering comments by `jq` and also fetching labels.
- **List issues**: `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'` with appropriate `--label` and `--state` filters.
- **Comment on an issue**: `gh issue comment <number> --body "..."`
- **Apply / remove labels**: `gh issue edit <number> --add-label "..."` / `--remove-label "..."`
- **Close**: `gh issue close <number> --comment "..."`

Infer the repo from `git remote -v` — `gh` does this automatically when run inside a clone.

**Triage state lives on the Project, not on a label.** `--add-label`/`--remove-label` above still apply to `journey-gap`, `bug`, `enhancement` and the `wayfinder:*` labels — they say what an issue *is*. What state it's *in* (needs triage, ready for agent, etc.) is the Project's Status field; see the next section.

## The Project: triage state

Every open issue is an item on the **Doula Cloud Project** — https://github.com/users/markgoho/projects/5, project number `5`, owner `markgoho`. Its **Status** field replaced the four triage labels ([#621](https://github.com/markgoho/doula-cloud/issues/621)); the role-to-value mapping is in `docs/agents/triage-labels.md`.

- **Read an item's Status**: `gh project item-list 5 --owner markgoho --format json --limit 400 --jq '.items[] | select(.content.number == {n}) | .status'` — `--limit` defaults to 30, well under the ~171-item project, so pass it explicitly or the query silently returns nothing. Or open the issue and check the Status column in the Table view.

  **Call this once per session, never once per issue.** It costs 404 GraphQL points — 8% of the hourly budget — and the cost does not fall when you filter with `--jq`, because the filter runs after the whole board is fetched. Reading Status for nine issues costs 404 points if you fetch the board once and index it locally, and 3,636 points if you loop. Save the JSON to a variable or a file and read every issue out of that.
- **Write an item's Status**: one field per invocation. **Use the node-id form** — it costs nothing, where the by-name form costs 103 points per write. Resolve the three ids once:

  ```sh
  PID=$(gh project view 5 --owner markgoho --format json --jq .id)
  FJ=$(gh project field-list 5 --owner markgoho --format json --limit 50)
  FID=$(echo "$FJ" | jq -r '.fields[]|select(.name=="Status")|.id')
  OPT=$(echo "$FJ" | jq -r '.fields[]|select(.name=="Status")|.options[]|select(.name=="Ready for agent")|.id')
  ```

  Then every write is free, however many you do:

  ```sh
  gh project item-edit --id "$ITEM" --field-id "$FID" --project-id "$PID" --single-select-option-id "$OPT"
  ```

  `$ITEM` is the item's node id, which comes out of the one board fetch above (`.items[] | select(.content.number == {n}) | .id`) — not the issue number and not its URL.

  The by-name form below is readable and correct, and fine for a single write when you are not already holding the ids. It is never right in a loop.

  ```sh
  gh project item-edit 5 --owner markgoho --url https://github.com/markgoho/doula-cloud/issues/632 --field "Status" --value "In progress"
  ```

  `--value` must be one of the six option strings: `Needs triage`, `Needs info`, `Ready for agent`, `Ready for human`, `In progress`, `Done`. `--url` is the *issue's* URL, not the project's.
- **Target date**: same command, `--field "Target date" --date "YYYY-MM-DD"` (see `gh project item-edit --help`; date fields take `--date`, not `--value`).
- **When Target date gets set, and by whom** ([#653](https://github.com/markgoho/doula-cloud/issues/653)): only an issue whose Status is `Ready for agent`, `Ready for human`, or `In progress` gets one — that's the earliest point a real deliverable and rough timing both exist, so a `Needs triage`/`Needs info` issue stays unset rather than carry a guessed date. Wayfinder maps, other `wayfinder:*` tickets, and the Capability parent issues are exempt outright: none has a natural single ship date. Use the first of the expected month (e.g. `2026-11-01`), never a specific day — nothing in this project's process supports day-level confidence. Whoever is triaging sets it at the same moment Status moves into an eligible value, the same way that session already sets Status; it's a proposal the maintainer can correct at any time, not a locked commitment.
- **What these commands cost, and why it matters.** GitHub's GraphQL budget is **5,000 points per hour, per user** — shared by every session, subagent and background agent on the machine, because they all authenticate as the same `gh` OAuth app. Points are not requests: a query's cost rises with the number of nodes it asks for, so `gh project` commands are two to three orders of magnitude more expensive than everything else in this file. Measured on this project, 2026-09-08:

  | command | points |
  | --- | --- |
  | `gh project item-list 5 --limit 400` | **404** |
  | `gh project item-edit` by field **name** | **103** |
  | `gh project field-list` | 101 |
  | `gh project item-list 5 --limit 30` | 30 |
  | `gh project item-edit` by **node id** | **0** |
  | `gh issue view N --json ...` | 0 |
  | `gh issue view N --comments` | 1 |
  | `gh issue list --search ...` | 1 |
  | any `gh api {REST path}` | 0 |

  `item-list` costs about one point per item requested. Filing nine issues and setting Status and Target date on each, the naive way — nine adds, 27 by-name edits, nine read-backs — costs roughly **7,300 points**, which is one and a half times the whole hourly allowance for a single routine triage pass. Two sessions doing that in the same hour is guaranteed exhaustion, and it is what kept happening before anyone measured it.

  Three rules follow: fetch the board once per session and index it locally; write by node id; and reach for `gh api {REST path}` whenever REST can do the job, since it costs no GraphQL points at all and draws on a separate 5,000-per-hour pool that is normally almost empty.

  Upstream context: [cli/cli#13433](https://github.com/cli/cli/issues/13433) asks GitHub to raise the `gh` OAuth app's GraphQL limit to 12,500 points/hour, on the argument that 5,000 is undersized for one developer's ordinary day before any automation.

- **Checking the budget before a large pass.** Do not use `gh api rate_limit` — seconds after the hourly rollover it reports 0/5000 on every bucket and looks healthy whatever happened in the previous window. Ask GraphQL what it thinks instead, which costs one point:

  ```sh
  gh api graphql -f query='{ rateLimit { used remaining resetAt } }' --jq .data.rateLimit
  ```

  `X-Ratelimit-Remaining` on the headers of any response works too.

- **When a pass is refused anyway**, GraphQL writes block while REST keeps working. A `gh api {REST path}` form exists for every issue operation — create, comment, edit, label, close, sub-issue linking — so a rate-limited pass can usually finish rather than stop. What it cannot do is Project field writes, which are GraphQL only; those wait for the reset that `resetAt` names.
- **Projects v2 cannot filter or query on issue dependencies** — no `BLOCKED_BY` field, column, or filter qualifier exists. The ready query below reads the Issues API instead.

### The ready query: open, unblocked, unassigned

Projects v2 has no dependency field, so this reads straight from the Issues API rather than the Project. `issue_dependencies_summary.blocked_by` (open blockers only) is present on the `issues` list endpoint, so no per-issue fetch is needed:

```sh
gh api --paginate "/repos/markgoho/doula-cloud/issues?state=open&per_page=100" \
  --jq '.[] | select(.pull_request == null) | select((.assignees | length) == 0) | select(.issue_dependencies_summary.blocked_by == 0) | .number'
```

Drop the trailing `| .number` and pipe to `jq -s length` (or `wc -l` on the number list) for a count — 133 against the open-issue set as of this ticket. `is:blocked` works in the web Issues list but returns 0 against the REST `search/issues` endpoint and the GraphQL `search` field — don't use either for this query.

## Pull requests as a triage surface

**PRs as a request surface: no.** _(Set to `yes` if this repo treats external PRs as feature requests; `/triage` reads this flag.)_

When set to `yes`, PRs run through the same labels and states as issues, using the `gh pr` equivalents:

- **Read a PR**: `gh pr view <number> --comments` and `gh pr diff <number>` for the diff.
- **List external PRs for triage**: `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments` then keep only `authorAssociation` of `CONTRIBUTOR`, `FIRST_TIME_CONTRIBUTOR`, or `NONE` (drop `OWNER`/`MEMBER`/`COLLABORATOR`).
- **Comment / label / close**: `gh pr comment`, `gh pr edit --add-label`/`--remove-label`, `gh pr close`.

GitHub shares one number space across issues and PRs, so a bare `#42` may be either — resolve with `gh pr view 42` and fall back to `gh issue view 42`.

## When a skill says "publish to the issue tracker"

Create a GitHub issue. If the ticket has a parent/source issue (e.g. tickets broken out from a spec via `/to-tickets`), also link it as a native GitHub sub-issue of that parent — `gh api --method POST repos/<owner>/<repo>/issues/<parent>/sub_issues -F sub_issue_id=<child-db-id>`, where `<child-db-id>` is the child's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, not the `#number`). A text reference ("Part of #N") in the body is not a substitute — it doesn't show up in GitHub's sub-issue progress bar or hierarchy UI.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a single issue with **child** issues as tickets.

- **Map**: a single issue labeled `wayfinder:map`, holding the Notes / Decisions-so-far / Fog body. `gh issue create --label wayfinder:map`.
- **Child ticket**: an issue linked to the map as a GitHub sub-issue (`gh api` on the sub-issues endpoint). Where sub-issues aren't enabled, add the child to a task list in the map body and put `Part of #<map>` at the top of the child body. Labels: `wayfinder:<type>` (`research`/`prototype`/`grilling`/`task`). Once claimed, the ticket is assigned to the driving dev.
- **Blocking**: GitHub's **native issue dependencies** — the canonical, UI-visible representation. Add an edge with `gh api --method POST repos/<owner>/<repo>/issues/<child>/dependencies/blocked_by -F issue_id=<blocker-db-id>`, where `<blocker-db-id>` is the blocker's numeric **database id** (`gh api repos/<owner>/<repo>/issues/<n> --jq .id`, _not_ the `#number` or `node_id`). GitHub reports `issue_dependencies_summary.blocked_by` (open blockers only — the live gate). Where dependencies aren't available, fall back to a `Blocked by: #<n>, #<n>` line at the top of the child body. A ticket is unblocked when every blocker is closed.
- **Frontier query**: list the map's open children (`gh issue list --state open`, scoped to the map's sub-issues / task list), drop any with an open blocker (`issue_dependencies_summary.blocked_by > 0`, or an open issue in the `Blocked by` line) or an assignee; first in map order wins.
- **Claim**: `gh issue edit <n> --add-assignee @me` — the session's first write.
- **Resolve**: `gh issue comment <n> --body "<answer>"`, then `gh issue close <n>`, then append a context pointer (gist + link) to the map's Decisions-so-far.
