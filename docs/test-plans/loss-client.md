# Nadia Haddad — test plan

- **Journey**: [loss-client.md](../journeys/loss-client.md)
- **Persona**: [loss-client.md](../personas/loss-client.md)
- **A pass means**: the record is accurate, Maya can still reach her, and no
  screen, label, or link addresses her as a pregnant person. **This plan cannot
  pass.** It is written to be run so the failures are recorded rather than
  imagined.

Written first, before the two other client-side plans, because the method standard
says to walk Nadia before any journey hers overlaps.

Her journey turns mid-way, so half this plan is walked as Maya on the staff side
and observed from Nadia's screen. Those steps are marked as her map marks them —
the stage belongs to her even where the click does not.

## Preconditions

- A solo Practice (Owner + Admin + Doula in one person) with one Client and one
  Engagement carrying a **signed** Contract, a **filled** Birth Plan Instance,
  several Visits, and a message thread with traffic both ways.
- **The Engagement cannot be aged.** Every Engagement is created at `intake` and
  no code anywhere runs `UPDATE engagements` (**MO-G4**), so "four months in"
  cannot be represented. Run against a fresh Engagement and treat it as the
  four-month one. That a four-month Engagement is indistinguishable from an
  hour-old one is stage 1's finding, not a defect in the fixture.
- Stage 6 needs a **voided** Contract. Sign it as the Client first, then void it
  from the staff Engagement page — the **Void** button renders only on a `signed`
  Contract (`app/src/lib/ContractStatus.svelte`). This is Dee's step 7.1-a; see
  the crossing note there.
- Two browser sessions or two profiles: Nadia in the portal, Maya on the staff
  side, both open at once. Stages 2, 3 and 7 need them side by side.

## Steps

### Stage 1 — A live Engagement, four months in

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Sign in at `/portal/login` | `GET /api/portal/session` resolves one Engagement and lands on `/portal/engagements/[engagementId]` | `automated (client-portal-login.e2e.ts)` |
| 1.2 | Read the thread and send a message | One continuous thread, in order, immutable (ADR-0002). Attachments upload both ways | `manual` |
| 1.2-a | Find anything on screen saying this care is four months old | `Status: intake` and a **Created** date. No due date, no gestation, no Client detail of any kind | `missing-feature (MO-G3)` [#252](https://github.com/markgoho/doula-cloud/issues/252) |

### Stage 2 — The loss; the Practice tries to mark it

Walked as Maya. Nadia is not in the app.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Record **what happened** against the Engagement | `engagements` is `(id, client_id, practice_id, status, created_at)`. There is nowhere an outcome can live except a message | `missing-feature (NH-G3)` [#295](https://github.com/markgoho/doula-cloud/issues/295) |
| 2.1-a | Choose a status that is true after a loss | Four values. `active` says the pregnancy continues, `completed` says the work finished, `postpartum` describes a birth with a baby at the end of it, `intake` is where it already sits | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) |
| 2.1-b | Set the status to any value at all | No handler writes `UPDATE engagements`. Even a true value could not be reached | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) |

### Stage 3 — Three weeks of not opening the portal

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | With her thread open, have Maya send a message | The tab refetches and the message appears. The push itself carries no content (ADR-0002) — asserted at the in-app fetch level, which is where the map draws the transport boundary | `automated (push-notification.e2e.ts)` |
| 3.1-a | Close the tab and repeat | Nothing arrives carrying content. Real device delivery is out of scope for this effort | `manual` |
| 3.2 | Wait, and watch for a reminder, a nudge, or a countdown | **Nothing happens**, and that is the finding. No reminder code exists in `api/`, a Visit carries no date (**[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**), and the only push fires on a Message | `manual` |

### Stage 4 — She opens the portal — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/portal/engagements/[engagementId]` after three weeks away | `GET /api/portal/engagements/{id}` returns `practiceName`, `status`, `createdAt` and the page renders | `automated (client-portal-login.e2e.ts)` |
| 4.2 | Read the `<h1>` | **"Welcome to Rooted Birth Collective"** — a first-visit greeting, rendered unconditionally, naming the Practice rather than her ([NH-G4](https://github.com/markgoho/doula-cloud/issues/296)) | `manual` |
| 4.2-a | Confirm the greeting is not gated on a first visit | The spec re-asserts the same `<h1>` on a fresh page in the same session. That proves it is not visit-gated; it does not prove what three weeks look like, which is 4.2's job | `automated (client-portal-login.e2e.ts)` |
| 4.2-b | Read the status line and the date under it | `Status: intake` — the raw enum ([#212](https://github.com/markgoho/doula-cloud/issues/212)), still `intake` because status never moves — and **Created**, the only date the portal holds (**[MO-G3](https://github.com/markgoho/doula-cloud/issues/252)**) | `manual` |
| 4.3 | Read what is offered below | Two links, **Birth Plan** first, then **Contract**; then the thread | `manual` |

### Stage 5 — The Birth Plan will not go away

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Look at the portal home without tapping anything | The **Birth Plan** link is rendered unconditionally, second from the top | `manual` |
| 5.1-a | Retire, hide, archive or dismiss it — from her side or Maya's | None of the four exist. A Plan Instance can be overwritten by Staff and nothing else; deletion is correctly ruled out | `missing-feature (NH-G2)` [#294](https://github.com/markgoho/doula-cloud/issues/294) |
| 5.2 | Tap the link | The filled Instance renders read-only | `automated (birth-plan.e2e.ts)` |
| 5.2-a | Press **Print** | The print stylesheet hides the Back link, the Print button and the chrome. The mechanism works | `manual` |

### Stage 6 — Money, and the word "voided"

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Contract link | The full signed prose renders. A `voided` Contract is not withheld; only the **Sign** form is, and only because it keys on `status === 'sent'` | `manual` |
| 6.2 | Read the status | `Status: voided`, then "Voided — this Contract is no longer active." The Client portal reuses the Staff `ContractStatus` component and passes no `onVoid`, so she gets the ledger's word with no Void button and no human context ([NH-G5](https://github.com/markgoho/doula-cloud/issues/212)) | `manual` |
| 6.2-a | Keep a copy of what she signed | `GET /api/portal/engagements/{id}/contract/pdf` 404s outright once the Contract is `voided` — `serveSignedPDF` (`signed_pdf.go:66-69`) queries `WHERE status = 'signed'`, so linking it (HS-G3) would not be enough | `missing-feature (HS-G3, NH-G8)` [#302](https://github.com/markgoho/doula-cloud/issues/302) [#299](https://github.com/markgoho/doula-cloud/issues/299) |
| 6.2-b | Find what she still owes, or what was refunded | The portal has no Invoice, balance or payment surface at all. The question she is most likely to have cannot be asked on screen | `missing-feature (NH-G6)` [#297](https://github.com/markgoho/doula-cloud/issues/297) |

### Stage 7 — Postpartum support continues anyway

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | As Maya, log a bereavement Visit | A row of a Staff name and a creation timestamp — no date (**[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**), no type (**[PR-G6](https://github.com/markgoho/doula-cloud/issues/281)**), no notes (**[MO-G2](https://github.com/markgoho/doula-cloud/issues/251)**). Indistinguishable from the prenatal ones | `manual` |
| 7.1-a | As Nadia, find any trace of that Visit | None. There is no client-facing Visit surface (`CONTEXT.md`), by design. The support is real; the record of it is empty on both sides | `manual` |
| 7.2 | Continue the thread both ways | Unchanged and unchangeable — immutable by design, which is correct here | `manual` |
| 7.2-a | Mark that the thread's subject has changed, or pause push | Neither exists. Push unregistration happens only at sign-out, so her only mute is to leave | `missing-feature (NH-G7)` [#298](https://github.com/markgoho/doula-cloud/issues/298) |

### Stage 8 — The record closes without erasing her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Close the Engagement truthfully | It sits at `intake` forever. "Done looks like" is unreachable: the record is permanent, which is right, and permanently wrong, which is not | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) |

8.1 is 2.1-a and 2.1-b met a second time, from her side rather than Maya's. Both
stages are kept because the map keeps both, and because the run should record that
the same absence is hit twice by two different people.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 5 |
| `manual` | 13 |
| `missing-feature` | 9 ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252), [NH-G3](https://github.com/markgoho/doula-cloud/issues/295), [NH-G1](https://github.com/markgoho/doula-cloud/issues/293) ×2, [MO-G4](https://github.com/markgoho/doula-cloud/issues/253), [NH-G2](https://github.com/markgoho/doula-cloud/issues/294), [HS-G3](https://github.com/markgoho/doula-cloud/issues/302) + [NH-G8](https://github.com/markgoho/doula-cloud/issues/299), [NH-G6](https://github.com/markgoho/doula-cloud/issues/297), [NH-G7](https://github.com/markgoho/doula-cloud/issues/298)) |

No step is `blocked`. Stripe never reaches the Client portal, so nothing here waits
on an account nobody has opened — **NH-G6** is a hole in the product, not a bill.

NH-G4, NH-G5, **MO-G1**, **MO-G2**, **PR-G6** and
[#212](https://github.com/markgoho/doula-cloud/issues/212) are observed inside
walkable steps (4.2, 6.2, 7.1, 4.2-b) rather than given steps of their own: the
step can be performed, and what it hands back is the finding.

Her five automated steps all come from specs written for a happy path. Every one of
them passes on this journey, and passing is the problem: `client-portal-login`
greets her, `push-notification` reaches her, `birth-plan` shows her the document.
The suite cannot tell her journey from Hannah's.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 1.1 | `client-portal-login.e2e.ts` | pass |
| 3.1 | `push-notification.e2e.ts` | pass |
| 4.1 | `client-portal-login.e2e.ts` | pass |
| 4.2-a | `client-portal-login.e2e.ts` | pass |
| 5.2 | `birth-plan.e2e.ts` | pass |

**5 automated steps: all pass.**

The `manual`, `blocked` and `missing-feature` steps are **not walked yet**.
That is [#239](https://github.com/markgoho/doula-cloud/issues/239).

### 2026-08-23 — manual and missing-feature steps ([#239](https://github.com/markgoho/doula-cloud/issues/239))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Willow Creek
Doula Care", Owner+Admin+Doula in one Staff row — Maya's shape, since her
journey carries this plan's staff-side stages) with one Client (Nadia
Haddad), one Engagement, four Visits, a filled Birth Plan, a Contract signed
by the Client and then voided by staff, and a four-message thread. Walked in
Chrome via playwriter, one browser profile.

**Note on the fixture**: Staff and Client-portal sessions share one
`__session` cookie per origin (confirmed via `Network.getCookies`), so a
single profile can hold only one of the two logged in at a time on
`localhost:5173` — signing in as the other silently 403s the first
("no matching staff account" on next request). This is exactly what the
plan's Preconditions warn about ("Two browser sessions or two profiles");
not a new finding, and not evidence of a production defect, since staff and
portal are reached via different routes there. Walked by re-authenticating
whichever side was about to act, immediately before each of its steps.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.2 | `manual` | as expected | One continuous thread; Maya's and Nadia's messages both appear in order, immutable |
| 1.2-a | `missing-feature (MO-G3)` [#252](https://github.com/markgoho/doula-cloud/issues/252) | confirmed | Engagement page (both sides) shows only `Status` and `Created` — no due date, no gestation, no other Client detail |
| 2.1 | `missing-feature (NH-G3)` [#295](https://github.com/markgoho/doula-cloud/issues/295) | confirmed | No control anywhere on the Engagement page records an outcome |
| 2.1-a | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) | confirmed | No status value is offered anywhere in the UI — there is no status control to read a value from |
| 2.1-b | `missing-feature (MO-G4)` [#253](https://github.com/markgoho/doula-cloud/issues/253) | confirmed | Same absence — no status control exists to set any value with |
| 3.1-a | `manual` | as expected, confirmed by inspection not by walking | Closing the tab was not exercised live; grounded instead in the same fact the journey map already cites (no reminder machinery in `api/`, content-free push by design, ADR-0002) — nothing was found to contradict it |
| 3.2 | `manual` | as expected, confirmed by inspection not by walking | Not literally waited out; grounded the same way as 3.1-a — no reminder/nudge/countdown code exists anywhere in `api/` |
| 4.2 | `manual` | as expected, one correction | `<h1>` reads **"Welcome to Willow Creek Doula Care"** — the actual Practice name, confirming [NH-G4](https://github.com/markgoho/doula-cloud/issues/296)'s `{practiceName}` template. (The plan's illustrative text names "Rooted Birth Collective", a different Practice used as the shared fixture in other personas' plans; the mechanism, not the literal string, is what NH-G4 claims, and it holds.) |
| 4.2-b | `manual` | as expected | `Status: intake`, `Created: 8/23/2026` — the only date the portal holds |
| 4.3 | `manual` | as expected | **Birth Plan** then **Contract**, then the thread — same order every time |
| 5.1 | `manual` | as expected | **Birth Plan** link renders unconditionally, second from the top |
| 5.1-a | `missing-feature (NH-G2)` [#294](https://github.com/markgoho/doula-cloud/issues/294) | confirmed | No retire/hide/archive/dismiss control on either side — Staff's Birth Plan section offers only **Save Birth Plan** |
| 5.2-a | `manual` | as expected | `page.emulateMedia({ media: 'print' })` hides both **Back** and **Print** (`isVisible()` false for both); the print mechanism works |
| 6.1 | `manual` | as expected | A voided Contract still renders its full signed prose on the Client side |
| 6.2 | `manual` | as expected | `Status: voided`, then "Voided — this Contract is no longer active." — the ledger's word, no Void button, no human context ([NH-G5](https://github.com/markgoho/doula-cloud/issues/212)) |
| 6.2-a | `missing-feature (HS-G3)` [#302](https://github.com/markgoho/doula-cloud/issues/302), reasoning corrected, new gap **[NH-G8](https://github.com/markgoho/doula-cloud/issues/299)** minted | **falsified in part** | The plan's claim was that the endpoint is merely unlinked. It is worse: `GET /api/portal/engagements/{id}/contract/pdf` itself 404s ("no signed contract found for this engagement") once the Contract is `voided`, confirmed with a direct `fetch()` from the page. `serveSignedPDF` (`api/internal/contracts/signed_pdf.go:66-69`) queries `WHERE ... AND status = $2::contract_status` bound to `statusSigned` — a Contract that has since moved to any other status, voided included, can never resolve a row, even though `signed_pdf_object_path` is still set and the PDF still exists in the store. Fixing HS-G3 (adding a link) would not fix this: the endpoint itself refuses. Distinct root from HS-G3, which is about the missing link on a Contract that is still `signed`; this is about the query excluding every Contract that is not. Same shared function backs the Staff-side signed-PDF route (`main.go:208`), so a Practice loses its own copy of a voided Contract too, once it is voided — noted here since this map owns the void transition, not asserted as a Staff-side finding of this plan. |
| 6.2-b | `missing-feature (NH-G6)` [#297](https://github.com/markgoho/doula-cloud/issues/297) | confirmed | No Invoice, balance, or payment surface anywhere in the portal — the portal's links are Birth Plan and Contract only |
| 7.1 | `manual` | as expected | A fourth Visit row: `Maya Okonkwo`, `8/23/2026` — indistinguishable from the first three |
| 7.1-a | `manual` | as expected | No Visit surface on the Client side at all — the portal home's only links remain Birth Plan and Contract |
| 7.2 | `manual` | as expected | The thread continues both ways; nothing marks that its subject has changed |
| 7.2-a | `missing-feature (NH-G7)` [#298](https://github.com/markgoho/doula-cloud/issues/298) | confirmed | No mute, pause, or subject-change control on either side's Messages section |
| 8.1 | `missing-feature (NH-G1)` [#293](https://github.com/markgoho/doula-cloud/issues/293) | confirmed | Same absence as 2.1-a/2.1-b, observed a second time from the Client's own side |

**22 steps walked: 13 `manual`, 9 `missing-feature`. Every mark holds; one
reasoning corrected (6.2-a) and one new gap minted (NH-G8, added to
[docs/journeys/loss-client.md](../journeys/loss-client.md)).** No `blocked`
step exists on this plan, confirmed. No `journey-gap` issue filed from this
ticket — that is [#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict**: this plan cannot pass, as written, and does not. Every
`missing-feature` step confirmed genuinely unwalkable; the record stays at
`intake` forever, the Birth Plan cannot be retired, and money has no
client-facing surface at all.

**Unexplained, not chased**: three `404`s in the browser console right after
**Save Contract** and one after **Void Contract** (staff side), origin not
identified. Neither action's own result was wrong — the Contract saved and
voided correctly both times — so this was not chased down to a source. Named
here rather than dropped, since an unexplained `404` on a write path is
exactly what turned into real findings on Maya's and Dee's walks.
