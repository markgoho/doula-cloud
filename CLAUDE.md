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

## Agent skills

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

