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

## Agent skills

### Issue tracker

Issues live in GitHub Issues for markgoho/doula-cloud, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical labels (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — `CONTEXT.md` + `docs/adr/` at the repo root. See `docs/agents/domain.md`.

### Testing

100% line coverage gate (with justified inline exceptions), the Podman-based
test infra for `api/` and `app/`, and goose migrations. See `docs/testing.md`.

### API design

Go BFF HTTP endpoint standards (DTOs, contract stability, idempotency, cursor pagination, rate limits, error structures). See `docs/api-design.md`.

