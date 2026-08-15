This is a project called Doula Cloud, it includes a Svelte application and a Hugo website (for marketing)

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
