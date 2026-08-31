This is a project called Doula Cloud, it includes a Svelte application and a Hugo website (for marketing)

## Status: pre-launch

**Doula Cloud has not launched. It has no users, and no production data.** The
target launch is **January 2027**.

This changes how findings are handled:

- **Everything found will be fixed before launch.** A missing capability is work
  not yet done, not a defect to triage against a live system.
- **Do not rank work by user impact or severity.** There are no users to impact.
  Where order matters at all, it is because one piece of work depends on another,
  not because one gap hurts more. Do not spend a session producing a priority
  ranking unless asked for one directly.
- **There is no backwards compatibility to preserve** and no migration of live
  data to plan. Schema and API changes are cheap right now, and get expensive in
  January.
- Nothing is in front of a customer, so a broken or absent path is not an
  incident.

## Temporary block: no new components (opened 2026-08-30)

**Do not build a new UI component while this section exists.** How this repo lays out to available space is under active decision in [#518](https://github.com/markgoho/doula-cloud/issues/518), and every component built to the current pattern joins a pile that has to be reworked afterwards. This section is temporary and is deleted when the block lifts — if you are reading it, the block is still on.

**Blocked**: creating a new `.svelte` file under `app/src/lib/components/` (any tier), a new route component that lays out its own contents rather than composing existing ones, and a new Hugo layout or partial under `hugo/layouts/`.

**Exempt**:

- Changing, fixing, or retrofitting a component that already exists — including making one adapt to the space it is given.
- Tickets belonging to [#518](https://github.com/markgoho/doula-cloud/issues/518) itself, which say so in their body.
- A route that composes only existing components and writes no layout CSS of its own.

**What lifts it**: [#518](https://github.com/markgoho/doula-cloud/issues/518) closing. Check with `gh issue view 518 --json state`. If that issue is closed and this section is still here, the deletion was missed — delete it and say so.

**If you are blocked and the work cannot wait**: say so to the person you are working with and let them decide, rather than building the component and noting the exception.

## Cross-cutting expectations

Every feature carries these. They are not separate work items, they are not
ranked against the feature, and no ticket has to ask for them.

- **Audit trail.** A user must be able to answer "how did this thing come to
  be?" — who sent the invoice, when a person accepted an invitation, when an
  employment type changed. Anything that changes state records who did it and
  when. Design each feature so that question has an answer; the shape of the
  record is the feature's own choice.
- **Accessibility.** What is built is usable by everyone who has to use it.
- **Performance.** What is built stays quick under a real Practice's data, not
  only a fixture's.
- **Security.** What is built refuses what it should refuse, at the boundary
  that can actually enforce it.
- **Layout.** What is built adapts intrinsically to the space it is given, never to a device it assumes it is on. Every screen is complete and usable from 320px up, and a component behaves correctly wherever it is placed — full page, narrow column, or embedded in a Practice's own website. **Choosing a component is a layout decision**: whoever picks one owns what it does at 320px with a real Practice's content in it, even on a screen that writes no CSS at all. See `docs/adr/0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md` for the mechanism and `docs/adr/0025-layout-is-verified-across-the-continuum.md` for how it is checked.

## Agent skills

### Git flow: worktree + PR, not a direct push to trunk

Default to one worktree per unit of work, landed via a PR with squash auto-merge — not a
direct push to `trunk`. See `docs/agents/worktree-flow.md` for the exact commands, what gets
provisioned automatically, and the cleanup pruner. Rollout note: the local edit-block hook
and the GitHub ruleset that make this a hard requirement are rolling out in stages — until
both are confirmed active, treat this as the default to follow by choice, not yet as
something that will refuse a direct trunk push.

### Issue tracker

Issues live in GitHub Issues for markgoho/doula-cloud, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Service patterns

The GOV.UK Design System is the default reference for any screen that asks a person for
something or reports a failure — the decision, never the markup or the look. See
`docs/adr/0021-govuk-is-the-reference-for-service-patterns.md` for the rule and
`docs/design/govuk-alignment.md` for the pattern-by-pattern table. Check the table before
building such a screen; a departure needs a recorded reason.

### Testing

100% line coverage gate (with justified inline exceptions), the Podman-based
test infra for `api/` and `app/`, and goose migrations. See `docs/testing.md`.

### Environment variables

Every variable the BFF reads, and what it holds locally, in CI, and on
Cloud Run — including the Stripe test-mode setup and its two webhook
surfaces. See `docs/environment.md`.

### API design

Go BFF HTTP endpoint standards (DTOs, contract stability, idempotency, cursor pagination, rate limits, error structures). See `docs/api-design.md`.

