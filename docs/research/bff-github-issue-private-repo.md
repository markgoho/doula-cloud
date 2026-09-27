# How the BFF opens an issue in a private GitHub repo

Research for [#1500](https://github.com/markgoho/doula-cloud/issues/1500), a child of [#1497](https://github.com/markgoho/doula-cloud/issues/1497) ("Wayfinder: a feedback mechanism for the pilot"). #1498's resolution settled the destination: the full feedback item stays in Doula Cloud's own database under the Google Cloud BAA, and the BFF opens an issue in a **private** GitHub repo carrying only the facts that are not PHI, plus a link back to the internal page only the founder can open. This note answers the mechanics: which credential opens that issue, where it lives, and how issue creation rides the outbox.

## Recommendation

A second fine-grained personal access token, scoped only to the private feedback repo with **Issues: write**, stored as its own Secret Manager secret and read by a hand-written outbox worker (the `ErasureWorker` shape, not `MailWorker`) registered under `/api/internal/feedback/process-issue-outbox`. Not a GitHub App, and not a widened `GITHUB_DISPATCH_TOKEN`. Reasons are in [Comparison](#comparison) and [Recommendation in full](#recommendation-in-full).

## Comparison

Both options install on the private feedback repo only, not on `markgoho/doula-cloud` or on the account as a whole.

### Permissions

A fine-grained personal access token (PAT) and a GitHub App installation grant the same permission shape for this job: **Issues: write** on the one repo, plus **Metadata: read**, which GitHub requires automatically as a dependency and does not show as a separate toggle ([permissions required for fine-grained PATs](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens), Issues category). Both are equally fine-grained here; the "GitHub Apps have finer permissions" line in GitHub's own [Apps vs. OAuth Apps comparison](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/differences-between-github-apps-and-oauth-apps) is a comparison against a *classic* PAT or OAuth app (`repo` scope, account-wide), not against a fine-grained one. `docs/environment.md` already draws this same conclusion for a near-identical case: `GITHUB_DISPATCH_TOKEN` is "the narrowest permission GitHub's dispatch endpoint accepts, and it is the same level a GitHub App would need, which is why #443 chose the simpler thing" (Contents: write, for `repository_dispatch`).

One label wrinkle. The REST "create an issue" page states plainly: "Only users with push access can set labels for new issues. Labels are silently dropped otherwise" ([create an issue](https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#create-an-issue)). #1498 Q5 makes the feedback kind a label on the issue, so a silently dropped label breaks the feature. The permissions page separately lists `POST /repos/{owner}/{repo}/issues/{issue_number}/labels` under Issues: write, with no extra requirement ([permissions required for fine-grained PATs](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)). The fix is mechanical either way: create the issue with no `labels` field, then call the labels endpoint as a second request. This removes the "push access" ambiguity for a PAT and costs one extra call.

### Lifetime and rotation

| | Fine-grained PAT | GitHub App installation token |
| --- | --- | --- |
| Credential a human creates | The token itself, at github.com/settings/personal-access-tokens | The App's private key (once), plus installing the App on the repo (once) |
| Token the BFF holds | The PAT itself, used directly as the bearer credential | An installation access token, minted by the BFF from the App's private key |
| Lifetime | Chosen at creation, up to 1 year; defaults to 30 days or less under an org token-lifetime policy ([managing your PATs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)) | Exactly 1 hour, always ([authenticating as a GitHub App installation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)) |
| Who rotates it, and how | A human, by hand: generate a new token at github.com, put it in Secret Manager as a new version, deploy | The BFF itself, automatically: sign a fresh JWT with the stored private key, `POST /app/installations/{id}/access_tokens`, cache the 1-hour token, repeat before it lapses |
| What "expired" looks like | Every request answers `401` until a human rotates it | Never expires on its own; only the underlying private key can be revoked or leaked |

Given the same task, the App's token never needs a human to remember an expiry date; the PAT does. That is a real advantage, but the App does not remove human involvement — it moves it from "renew a token yearly" to "install the App once and keep its private key safe," and it adds code this repo does not have yet: JWT signing and installation-token caching. No package under `api/internal` does this today; `sitebuild.Dispatcher` and `GITHUB_DISPATCH_TOKEN` both use a plain bearer PAT, not App auth.

### Rate limits

Both sit at 5,000 requests/hour on GitHub's primary limit; a GitHub App's ceiling only grows past that with more installed repositories or org users, which does not apply to an App installed on one repo ([rate limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)). Either way, `outbox.Worker.HandlePending` claims and sends rows one at a time inside a single request, so nothing in this feature drives concurrent volume anywhere near that ceiling at pilot scale. Creating issues in a tight loop can also hit a *secondary* rate limit the primary-limit page does not size, which the create-an-issue page notes without a number; the outbox's own backoff schedule already spaces retries far apart, so this is not a design concern here.

### What happens when the credential expires

For either credential, a call answers `401` the moment it is invalid. Inside the outbox, that is not automatically visible: `Worker.MarkFailed` returns `nil` on a `401` the same as on any other send error, so `DrainHandler` still answers `200` and the Cloud Scheduler job stays green while the row retries five times over `outbox.BackoffSchedule` (about a day) and then dead-letters silently. `docs/environment.md`'s own precedent (#443's `GITHUB_DISPATCH_TOKEN`) is explicit that its expiry must not be silent: "a lapsed token is not silent here — the dispatch fails, the page never leaves `pending`, and the Practice's website settings screen says her page is not confirmed." Feedback has no equivalent surface today. Recommended fix, not yet built: the internal page #1498 already plans (where the founder reads the full item and PHI) should show an item's outbox status, so a dead-lettered row ("not yet on GitHub") is visible without adding a new alerting channel.

## Secret placement

Following `docs/environment.md`'s existing table and the `GITHUB_DISPATCH_TOKEN` row in it exactly:

| Place | Mechanism |
| --- | --- |
| Local (`bun run dev:full`) | Unset. `main.go` wires a fake `IssueCreator` (the `stripe_fake.go`/e2e-mailbox shape: an in-memory stub the test stack asserts against, never a real GitHub call) |
| CI (`bun run test:e2e`) | Same fake as local — no GitHub credential in CI, matching "CI stays on the fakes" for Stripe |
| Deployed (Cloud Run `doula-api`) | Secret Manager, a **new** secret (e.g. `doula-cloud-github-feedback-token`), granted `roles/secretmanager.secretAccessor` to `doula-api-runtime@doula-cloud.iam.gserviceaccount.com` in `terraform/secrets.tf`, attached to the service with `--update-secrets` out of band the same way `GITHUB_DISPATCH_TOKEN` was — because, per `docs/environment.md`, provisioning it can only happen once a human has created the token, so it cannot be a hard dependency of a green `ci.yml` deploy |

A **new** secret, not a wider `GITHUB_DISPATCH_TOKEN`. That token is scoped to `markgoho/doula-cloud` with Contents: write for `repository_dispatch`; the feedback token needs Issues: write on a *different*, private repo. Widening one token to cover two repos and two permission categories fails least privilege for no benefit — the two credentials are created, rotated, and can be revoked independently, and a compromise of one does not reach the other repo.

## Fit with `api/internal/outbox`

Yes: a new outbox, one row per feedback item, following the shape `api/internal/client/erasure_outbox.go` already uses for a non-mail act (`ErasureWorker`, riding `outbox.Worker.HandlePending` directly, not `MailWorker`, which is mail-specific). Concretely:

- **A new table**, e.g. `feedback_issue_outbox` — `id`, the feedback item it is for, `status`, `attempt_count`, `next_attempt_at`, `last_error`, and a new `github_issue_number` column (nullable), written once creation succeeds.
- **A hand-written worker**, `feedback.IssueWorker`, with an `IssueCreator` interface (`CreateIssue`, `AddLabels`) taking the same shape as `client.StripeEraser` — a narrow interface this package declares, satisfied by a real GitHub client in `main.go` and a fake in tests.
- **A `Registration`** at `/api/internal/feedback/process-issue-outbox` (under `/feedback`, not `/notifications`, for the same reason erasure sits under `/clients` — it does not mail anyone), added to the list `outbox.Register` mounts and the one the Cloud Scheduler drain already covers with no console change (#481).
- **A `tasknudge.OutboxType`** ("feedback-issue"), nudged immediately after the write commits, the same as every other write-triggered outbox — unlike `sitebuild`'s 90-second coalescing nudge, there is nothing to batch here: one feedback item is one issue, not one of several rows that should collapse into a single GitHub call.
- **`Door`**: empty, unless the feedback item's own table sits under RLS and the worker needs a session variable to read it outside a Staff/Client request context — the same question `ErasureWorker`'s door (empty; it reads `practices` directly) already answers for a comparable case.

### Retry without a duplicate issue

GitHub's create-an-issue endpoint takes no idempotency key and the docs page for it says nothing about one ([create an issue](https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#create-an-issue)) — unlike the Stripe calls this codebase already makes with `params.SetIdempotencyKey` (`api/internal/billing/stripe_api_client.go:170`). So the safety net every other outbox worker leans on does not exist here: `ErasureWorker`'s own comment states its acts are "idempotent on their own side — a deleted Customer deletes again as a no-op", which does not hold for *creating* something.

The concrete risk: `outbox.HandlePending`/`runOutbox` claim a batch, call out, and mark rows sent or failed **inside one open transaction**, committing once at the end (`drain.go`'s `runOutbox`). If the BFF process dies, or the transaction fails to commit, *after* GitHub answers `201 Created` but *before* that commit, the whole transaction — including any `MarkSent` — rolls back. The row is still `pending`, with `attempt_count` unchanged, and looks exactly like a row that was never attempted. The next tick creates a second issue for the same feedback item. This is not a hypothetical unique to GitHub: the same window exists today for every mail kind (a send that succeeds just before a crash can, in principle, re-send), and the project has evidently accepted that risk for a channel where a duplicate is a spare email, not a spare public-facing side effect on a resource with no delete-is-a-no-op property.

The fix has to run on **every** attempt, not just a retry (`attempt_count > 0`), because the dangerous case is the very first attempt whose success was never durably recorded:

1. Put the feedback item's row id in the issue body as a hidden marker (e.g. an HTML comment).
2. Before calling create, check for an existing issue carrying that marker with `GET /repos/{owner}/{repo}/issues?state=all&since=<row's created_at>` ([list repository issues](https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#list-repository-issues)) — the plain REST list, not `/search/issues`, which carries its own tighter rate limit and an indexing lag the REST list does not have.
3. If found, adopt its number and mark the row sent with no second create. If not found, create it (with no `labels` field), then add labels via the separate labels-endpoint call, then write the returned issue number onto the row and mark it sent — in the same transaction, so the row's own audit trail always answers "which issue did this become."

At pilot volume (occasional feedback items from a 14-doula agency) the extra list call before every create is inexpensive and does not need caching or narrowing.

## Recommendation in full

A fine-grained personal access token, not a GitHub App, for the same reason `docs/environment.md` gives for #443's `GITHUB_DISPATCH_TOKEN`: the permission a GitHub App would need is no narrower than what a fine-grained PAT already grants for this one repo, and the App additionally requires code this repo does not have — JWT signing and installation-token caching — for one occasional outbound call. The App's real advantage, a token that never needs a human to remember an expiry date, is a genuine cost saved over the token's life, but it is a cost this project already chose to carry once (`GITHUB_DISPATCH_TOKEN`, 366-day expiry, renewed by hand at github.com/settings/personal-access-tokens) and can carry again with no new mechanism. Consistency with an already-working, already-documented pattern outweighs the App's rotation convenience at this scale and this call frequency.

Scoped as its own secret, not a widened `GITHUB_DISPATCH_TOKEN`, because the two tokens protect different repos and different permission categories, and least privilege means a leak of one must not reach the other.

Wired into `api/internal/outbox` as a new, hand-written, non-mail worker in the `ErasureWorker` shape, registered like every other outbox, nudged immediately (not coalesced) after the write commits, and drained by the existing Cloud Scheduler job with no console change.

Deduplicated against a retry by a marker embedded in the issue body and a plain REST list-and-check before every create — not by GitHub's own idempotency, because the create-an-issue endpoint has none — with the resulting issue number written back onto the outbox row in the same transaction that marks it sent.

Not yet built, and worth deciding on the build ticket: what a human sees when the token lapses and the row dead-letters silently under today's `DrainHandler`/`MarkFailed` behavior. `docs/environment.md`'s #443 precedent treats this as mandatory ("a lapsed token is not silent here"); feedback has no equivalent surface yet, and the internal page #1498 already plans for reading the full item is the natural place to add one.

## Sources

- [Differences between GitHub Apps and OAuth apps](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/differences-between-github-apps-and-oauth-apps)
- [Permissions required for fine-grained personal access tokens](https://docs.github.com/en/rest/authentication/permissions-required-for-fine-grained-personal-access-tokens)
- [Managing your personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)
- [Authenticating as a GitHub App installation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation)
- [Installing your own GitHub App](https://docs.github.com/en/apps/using-github-apps/installing-your-own-github-app)
- [REST API: Create an issue](https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#create-an-issue)
- [REST API: List repository issues](https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#list-repository-issues)
- [Rate limits for the REST API](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)
- `docs/environment.md` (`GITHUB_DISPATCH_TOKEN` row, and the whole `/api/internal/site` section)
- `docs/api-design.md` (section 3, `Idempotency-Key`)
- `api/internal/outbox/outbox.go`, `registry.go`, `drain.go`, `mailworker.go`
- `api/internal/client/erasure_outbox.go`
- `api/internal/idempotency/idempotency.go`
- `api/internal/billing/stripe_api_client.go:170` (`SetIdempotencyKey`)
- `api/internal/sitebuild/sitebuild.go`
- `api/internal/tasknudge/tasknudge.go`, `registry.go`
- ADR-0010 (notification email delivery is an outbox), ADR-0013 (the Cloud Tasks nudge), ADR-0037 (the internal boundary is a caller identity)

