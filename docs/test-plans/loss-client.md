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
| 1.2-a | Find anything on screen saying this care is four months old | `Status: intake` and a **Created** date. No due date, no gestation, no Client detail of any kind | `missing-feature (MO-G3)` |

### Stage 2 — The loss; the Practice tries to mark it

Walked as Maya. Nadia is not in the app.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Record **what happened** against the Engagement | `engagements` is `(id, client_id, practice_id, status, created_at)`. There is nowhere an outcome can live except a message | `missing-feature (NH-G3)` |
| 2.1-a | Choose a status that is true after a loss | Four values. `active` says the pregnancy continues, `completed` says the work finished, `postpartum` describes a birth with a baby at the end of it, `intake` is where it already sits | `missing-feature (NH-G1)` |
| 2.1-b | Set the status to any value at all | No handler writes `UPDATE engagements`. Even a true value could not be reached | `missing-feature (MO-G4)` |

### Stage 3 — Three weeks of not opening the portal

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | With her thread open, have Maya send a message | The tab refetches and the message appears. The push itself carries no content (ADR-0002) — asserted at the in-app fetch level, which is where the map draws the transport boundary | `automated (push-notification.e2e.ts)` |
| 3.1-a | Close the tab and repeat | Nothing arrives carrying content. Real device delivery is out of scope for this effort | `manual` |
| 3.2 | Wait, and watch for a reminder, a nudge, or a countdown | **Nothing happens**, and that is the finding. No reminder code exists in `api/`, a Visit carries no date (**MO-G1**), and the only push fires on a Message | `manual` |

### Stage 4 — She opens the portal — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/portal/engagements/[engagementId]` after three weeks away | `GET /api/portal/engagements/{id}` returns `practiceName`, `status`, `createdAt` and the page renders | `automated (client-portal-login.e2e.ts)` |
| 4.2 | Read the `<h1>` | **"Welcome to Rooted Birth Collective"** — a first-visit greeting, rendered unconditionally, naming the Practice rather than her (NH-G4) | `manual` |
| 4.2-a | Confirm the greeting is not gated on a first visit | The spec re-asserts the same `<h1>` on a fresh page in the same session. That proves it is not visit-gated; it does not prove what three weeks look like, which is 4.2's job | `automated (client-portal-login.e2e.ts)` |
| 4.2-b | Read the status line and the date under it | `Status: intake` — the raw enum ([#212](https://github.com/markgoho/doula-cloud/issues/212)), still `intake` because status never moves — and **Created**, the only date the portal holds (**MO-G3**) | `manual` |
| 4.3 | Read what is offered below | Two links, **Birth Plan** first, then **Contract**; then the thread | `manual` |

### Stage 5 — The Birth Plan will not go away

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Look at the portal home without tapping anything | The **Birth Plan** link is rendered unconditionally, second from the top | `manual` |
| 5.1-a | Retire, hide, archive or dismiss it — from her side or Maya's | None of the four exist. A Plan Instance can be overwritten by Staff and nothing else; deletion is correctly ruled out | `missing-feature (NH-G2)` |
| 5.2 | Tap the link | The filled Instance renders read-only | `automated (birth-plan.e2e.ts)` |
| 5.2-a | Press **Print** | The print stylesheet hides the Back link, the Print button and the chrome. The mechanism works | `manual` |

### Stage 6 — Money, and the word "voided"

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Contract link | The full signed prose renders. A `voided` Contract is not withheld; only the **Sign** form is, and only because it keys on `status === 'sent'` | `manual` |
| 6.2 | Read the status | `Status: voided`, then "Voided — this Contract is no longer active." The Client portal reuses the Staff `ContractStatus` component and passes no `onVoid`, so she gets the ledger's word with no Void button and no human context (NH-G5) | `manual` |
| 6.2-a | Keep a copy of what she signed | `GET /api/portal/engagements/{id}/contract/pdf` is routed (`main.go:226`) and linked from nowhere | `missing-feature (HS-G3)` |
| 6.2-b | Find what she still owes, or what was refunded | The portal has no Invoice, balance or payment surface at all. The question she is most likely to have cannot be asked on screen | `missing-feature (NH-G6)` |

### Stage 7 — Postpartum support continues anyway

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | As Maya, log a bereavement Visit | A row of a Staff name and a creation timestamp — no date (**MO-G1**), no type (**PR-G6**), no notes (**MO-G2**). Indistinguishable from the prenatal ones | `manual` |
| 7.1-a | As Nadia, find any trace of that Visit | None. There is no client-facing Visit surface (`CONTEXT.md`), by design. The support is real; the record of it is empty on both sides | `manual` |
| 7.2 | Continue the thread both ways | Unchanged and unchangeable — immutable by design, which is correct here | `manual` |
| 7.2-a | Mark that the thread's subject has changed, or pause push | Neither exists. Push unregistration happens only at sign-out, so her only mute is to leave | `missing-feature (NH-G7)` |

### Stage 8 — The record closes without erasing her

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Close the Engagement truthfully | It sits at `intake` forever. "Done looks like" is unreachable: the record is permanent, which is right, and permanently wrong, which is not | `missing-feature (NH-G1)` |

8.1 is 2.1-a and 2.1-b met a second time, from her side rather than Maya's. Both
stages are kept because the map keeps both, and because the run should record that
the same absence is hit twice by two different people.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 5 |
| `manual` | 13 |
| `missing-feature` | 9 (MO-G3, NH-G3, NH-G1 ×2, MO-G4, NH-G2, HS-G3, NH-G6, NH-G7) |

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

Not yet run. First execution is
[#209](https://github.com/markgoho/doula-cloud/issues/209).
