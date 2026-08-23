# Hannah Sorensen — test plan

- **Journey**: [first-time-client.md](../journeys/first-time-client.md)
- **Persona**: [first-time-client.md](../personas/first-time-client.md)
- **A pass means**: a portal account, a signed Contract, a Birth Plan she has read
  and printed, a thread with traffic both ways, and an Engagement that reflects
  where she actually is. The last of the five is unreachable.

She is the full-arc Client and the only Persona who walks every client-facing
screen the product has, so it carries the most `automated` steps of any client-side
plan — every portal spec was written along her path. Only Maya's, at nine, carries
more, and hers are the Staff side of the same walk. Where her path crosses
Nadia Haddad's, Nadia's plan was written first
([loss-client.md](loss-client.md)).

## Preconditions

- A Practice with an Owner and one Client created for Hannah — name and email
  only, which is all `POST .../clients` takes (**MO-G3**).
- A Contract Template with merge fields, so stage 4 has prose to render.
- Nothing else. She is the one Persona the product can provision end to end.
- A phone, or a phone-sized viewport, for stages 6 and 7. Stage 6 is the moment of
  truth and it is device-specific: she prints from whatever she has to hand.

## Steps

### Stage 1 — Getting invited

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Press **Send portal invite** on the Engagement page | `POST .../portal-invite` returns `{clientPortalUserId, inviteToken}` and the page prints `…/portal/accept-invite?token=…` as a `<code>` block to copy. `portal-invite-accept.e2e.ts` calls this endpoint as fixture setup, so **the button itself is unautomated** | `manual` |
| 1.2 | Send the link by hand | There is no email infrastructure at all (`portalinvite/invite.go`). She receives an unbranded URL with a token in it, by text, from a small business, with no way to verify it — **RA-G1**'s root and **PR-G7**'s shape, in the population where a phishing-shaped link lands hardest | `manual` |

### Stage 2 — Making an account

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Open `/portal/accept-invite` with no token | "Missing invite token" and nothing else — no form | `manual` |
| 2.1-a | Open it with the token | Email, Password, an **Account mode** radio defaulting to "I'm new here — create an account", and **Accept invite** | `automated (portal-invite-accept.e2e.ts)` |
| 2.2 | Fill email and password, leave the mode alone, press **Accept invite** | `POST /api/portal/accept-invite` claims the pending row and sets the session cookie on its own response (#145). No Identity Platform credential is left in IndexedDB (#150). Password minimum is six characters; nothing else is asked, because the Practice already holds her name | `automated (portal-invite-accept.e2e.ts)` |
| 2.2-a | Repeat with "I already have an account — log in" chosen | **Fails**, with a generic "Accept invite failed" covering both the sign-up and the sign-in path. For a first-time Client this option can only fail, and the form asked her to choose before she could know which she was (HS-G1) | `manual` |
| 2.3 | Watch where she lands | `GET /api/portal/session` resolves one Engagement, so she is redirected straight to it rather than shown a chooser | `automated (portal-invite-accept.e2e.ts)` |

### Stage 3 — The first screen

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Read the `<h1>` | **"Welcome to Rooted Birth Collective"** — the Practice's name, not hers, and unconditional for the life of the Engagement (NH-G4) | `automated (portal-invite-accept.e2e.ts)` |
| 3.1-a | Read the two lines under it | `Status: intake` — the raw enum ([#212](https://github.com/markgoho/doula-cloud/issues/212)) — and **Created**, a date that means nothing to her. The product holds no due date to show instead (**MO-G3**) | `manual` |
| 3.2 | Read what is offered below | **Birth Plan** and **Contract** links, then the thread. Nothing explains what either is for, or what she is meant to do first | `manual` |
| 3.3 | Watch for anything asking about notifications | Nothing asks. A push subscription is registered once per device, fire-and-forget (#61), silently | `manual` |
| 3.3-a | Review, mute, or explain that subscription afterwards | There is no notification setting anywhere in the portal | `missing-feature (HS-G4)` |

### Stage 4 — Signing the Contract

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open the Contract link once Maya has sent it | The prose renders with merge-field values filled, and `Status: sent` | `manual` |
| 4.1-a | Open it while the Contract is still `draft` | "No Contract has been sent for this Engagement yet" — RLS gives her no row, and that is the right message | `manual` |
| 4.1-b | As Maya, take a signature before sending the portal invite | The Contract can be sent, and no Client can reach it. The ordering is real and nothing on the Staff side states it | `missing-feature (MO-G6)` |
| 4.2 | Type her full legal name, tick the attestation, submit | `POST .../contract/sign` succeeds and the page re-renders at `signed`. **The strongest screen in the portal** — and no spec covers it | `manual` |
| 4.3 | Keep a copy of what she signed | `GET /api/portal/engagements/{id}/contract/pdf` is routed (`main.go:226`) and nothing in the portal's contract page links it | `missing-feature (HS-G3, HS-G6)` |

### Stage 5 — Reading the Birth Plan

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open the Birth Plan link before Maya has filled one | "No Birth Plan has been created for this Engagement yet", with no indication of whether one is coming, or when | `manual` |
| 5.1-a | Open it once filled | The Plan Instance's snapshot renders read-only | `automated (birth-plan.e2e.ts)` |
| 5.2 | Correct the two things she wants changed | She cannot. It is staff-drafted and read-only to her: no editable field, no comment, no suggested edit, no acknowledgement that she has read it (HS-G2) | `missing-feature (HS-G2)` |
| 5.2-a | Message Maya about them instead | Succeeds. Her only route to a correction is a re-type by Staff, and nothing links the message to the document | `manual` |

### Stage 6 — Print it and hand it over — moment of truth

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Open the Birth Plan on her phone and press **Print** | The mechanism works: a real print stylesheet hides the Back link, the Print button and the chrome (`birth-plan/+page.svelte`, `@media print`). **Print is the only export** — no PDF, no share link | `manual` |
| 6.1-a | Have Maya hand it over on her behalf | No deep link, no print from the Doula's side; the print stylesheet lives only on the Client's portal view. The same moment from the Doula's side is **PR-G5** | `missing-feature (PR-G5)` |
| 6.2 | Hand the paper over | Leaves the product entirely. The last mile is paper she remembered to print in advance, handed to a stranger in scrubs | `manual` |

### Stage 7 — Living in the thread

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Send a message at 11pm from her phone, with a photo | `POST .../messages` as `multipart/form-data`; the image renders inline. Attachments work both ways | `manual` |
| 7.2 | Receive Maya's reply with the tab open | The tab refetches on the push message and the reply appears without a reload. The push itself carries no content (ADR-0002) | `automated (push-notification.e2e.ts)` |
| 7.3 | Press **Load older** and read back through the Engagement | Older messages page in on demand, in order, immutable | `manual` |
| 7.3-a | Find out that a push is **not an alarm** | Nothing on screen says so. ADR-0002 is explicit that it is not a substitute for a phone call in a time-critical moment, and that fact reaches her nowhere | `missing-feature (HS-G4)` |

### Stage 8 — Birth, and afterwards

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | Open the portal after the birth | It looks exactly as it did at 18 weeks. Same heading, same `intake`, same two links | `manual` |
| 8.1-a | Have the status reflect where she actually is | No handler writes `UPDATE engagements`, so the status line on her home screen has been wrong since the day she signed | `missing-feature (MO-G4)` |
| 8.1-b | Find any record of the birth itself | Visits are invisible to the Client (`CONTEXT.md`), the birth included | `manual` |

### Stage 9 — Her partner asks for the login

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 9.1 | Give her partner access | `client_portal_users` holds one `identity_uid` per Client row. No second invite, no guest role, no read-only share of the Birth Plan. The supported path is sharing her password, which is the one path nobody designed | `missing-feature (HS-G5)` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 6 |
| `manual` | 18 |
| `missing-feature` | 8 (HS-G4 ×2, MO-G6, HS-G3+HS-G6, HS-G2, PR-G5, MO-G4, HS-G5) |

No step is `blocked`. She never reaches a Stripe surface — the portal has none —
so the one thing she cannot see about money (**NH-G6**: no Invoice, balance or
payment view anywhere in the portal) is a hole in the product, not a bill.

HS-G1, NH-G4, **RA-G1**, **PR-G7**, **MO-G3** and
[#212](https://github.com/markgoho/doula-cloud/issues/212) are observed inside
walkable steps (2.2-a, 3.1, 1.2, 3.1-a) rather than given steps of their own.

The Birth Plan's absent export is now [her map](../journeys/first-time-client.md)'s
**HS-G7**, and 4.3's absent Contract copy sharpened into **HS-G6** — both minted at
the run ([#240](https://github.com/markgoho/doula-cloud/issues/240)). One fact
still only handed forward, not a gap: **stage 4 is the strongest screen in the
portal with no spec on it** — 4.2 signs a Contract and the suite never does.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 2.1-a | `portal-invite-accept.e2e.ts` | pass |
| 2.2 | `portal-invite-accept.e2e.ts` | pass |
| 2.3 | `portal-invite-accept.e2e.ts` | pass |
| 3.1 | `portal-invite-accept.e2e.ts` | pass |
| 5.1-a | `birth-plan.e2e.ts` | pass |
| 7.2 | `push-notification.e2e.ts` | pass |

**6 automated steps: all pass.**

### 2026-08-23 — manual and missing-feature steps ([#240](https://github.com/markgoho/doula-cloud/issues/240))

`bun run dev:full` in `app/`, against a fresh solo Practice ("Rooted Birth
Collective", Owner+Admin+Doula in one Staff row) with one Client (Hannah
Sorensen), a Contract sent with real merge-field values entered, and a filled
Birth Plan. Walked in Chrome via playwriter, one browser profile, phone
viewport (390×844) for stages 6 and 7.

**Note on the fixture**: confirms the same shared-`__session`-cookie fact
[#239](https://github.com/markgoho/doula-cloud/issues/239) already recorded —
signing in as Staff or as the Client silently invalidates whichever side was
signed in on the other route. Not re-argued here; walked by re-authenticating
whichever side was about to act, immediately before each of its steps.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `POST .../portal-invite` returns the token; the page prints the accept-invite URL as a `<code>` block, no email sent |
| 1.2 | `manual` | as expected | The link was used directly, exactly as printed — no verification mechanism exists |
| 2.1 | `manual` | as expected | "Missing invite token", no form |
| 2.2-a | `manual` | as expected | Chosen on Hannah's real, still-unclaimed token: `400`, generic "Accept invite failed"; the token stayed unconsumed and signup succeeded right after |
| 3.1-a | `manual` | as expected | `Status: intake`, `Created: 8/23/2026` |
| 3.2 | `manual` | as expected | **Birth Plan** and **Contract** links only, nothing explains either |
| 3.3 | `manual` | as expected | Nothing on screen asks about notifications |
| 3.3-a | `missing-feature (HS-G4)` | confirmed | No notification setting anywhere in the portal — the banner holds only **Sign out** |
| 4.1-a | `manual` | as expected | "No Contract has been sent for this Engagement yet." while the Contract was still `draft` |
| 4.1-b | `missing-feature (MO-G6)` | confirmed | On a second, never-invited Client, **Send Contract** succeeded (`Status: sent`) with no prior portal invite at all; nothing on the Staff side warns of the ordering |
| 4.1 | `manual`, **MO-G10 reconfirmed** | as expected, with a correction | `Status: sent` renders, but every merge field is blank — real values (`Rooted Birth Collective`, `Hannah Sorensen`, `$2200`, dates) were typed and **Send Contract** pressed, and neither the staff-side form nor the client-side prose ever showed them on reload. Same defect MO-G10 already names (`contracts/contract.go:124`); not a data-entry mistake, confirmed by re-checking the Staff form after a fresh login |
| 4.2 | `manual` | as expected | Full legal name typed, attestation checked, **Sign** re-renders the page at `Status: signed` |
| 4.3 | `missing-feature (HS-G3)`, sharpened, new gap **HS-G6** minted | **falsified in part** | The plan's claim was that the endpoint is merely unlinked. It is worse: `GET /api/portal/engagements/{id}/contract/pdf` itself answers `500 internal error` when called directly (`fetch()` from the page), even though the signed Contract's `status` is `signed`. Querying `fake-gcs-server`'s own API directly (`GET /storage/v1/b/doula-cloud-e2e-attachments/o`) shows the PDF *is* present at `contracts/{engagementId}/signed.pdf` and downloads fine through the emulator's own `mediaLink` — so the object write and the object itself are both fine. The break is in the BFF's read path: `objectstore.GCSStore.Get` (`api/internal/objectstore/gcs.go`) is backed by a `storage.Client` constructed with no options at all (`storage.NewClient(context.Background())`, `main.go:263`), a known incompatibility class between the Go GCS client's newer default read behavior and `fake-gcs-server`. Whether this also breaks against real GCS in Cloud Run is unverified from this walk — a local-only harness defect and a real product defect look identical from here, and that ambiguity belongs to whoever picks up HS-G6, not this ticket |
| 5.1 | `manual` | as expected | "No Birth Plan has been created for this Engagement yet." |
| 5.2 | `missing-feature (HS-G2)` | confirmed | Fully read-only rendering (`Hospital`, notify list, atmosphere text, photo preference) — no editable field, no comment box, no acknowledgement control anywhere on the page |
| 5.2-a | `manual` | as expected | A message referencing the Birth Plan sends and appears in the thread immediately |
| 6.1 | `manual` | as expected | `page.emulateMedia({ media: 'print' })` removes **Sign out**, **Back** and **Print** from the accessibility tree, leaving only the Plan content — the print stylesheet works |
| 6.1-a | `missing-feature (PR-G5)` | confirmed | Staff's Birth Plan section offers only the edit form and **Save Birth Plan** — no print-for-client, deep link, or hand-over control of any kind |
| 6.2 | `manual` | as expected, not literally walkable | Conceptual — the product has nothing left to observe once the paper is handed over |
| 7.1 | `manual`, new gap **HS-G6** | **as expected on upload, falsified on delivery** | A message with a `pixel.png` attachment posts successfully (`201`) and appears in the thread as a clickable `pixel.png` control — but clicking it, and fetching the attachment URL directly, both answer `500 internal error` and surface a visible **internal error** alert to the Client. Same root as 4.3: the object is written (confirmed present in `fake-gcs-server`) and the BFF's `Get` cannot read it back. "Attachments work both ways" does not hold on this stack — nothing renders inline, on either side of the exchange, once the read path is exercised for real for the first time |
| 7.3 | `manual`, not exercised | inconclusive | Only one message existed in the thread; no **Load older** control rendered. Not evidence of a defect — the fixture never produced enough history to trigger it |
| 7.3-a | `missing-feature (HS-G4)` | confirmed | Nothing near the thread says a push is not an alarm |
| 8.1 | `manual` | as expected | Same heading, same `intake`, same two links, revisited after a reload |
| 8.1-a | `missing-feature (MO-G4)` | confirmed | No status control exists anywhere on either side |
| 8.1-b | `manual` | as expected | No Visits section anywhere on the Client side of the product |
| 9.1 | `missing-feature (HS-G5)` | confirmed | No second-account, guest, or share control anywhere in the portal — the banner holds only **Sign out** |

**26 steps walked: 18 `manual`, 8 `missing-feature`. Every existing mark
holds; 4.3 was falsified in part (sharpened, not deleted) and two new gaps
minted — HS-G6 and HS-G7 — added to
[docs/journeys/first-time-client.md](../journeys/first-time-client.md).** No
`blocked` step exists on this plan, confirmed. No `journey-gap` issue filed
from this ticket — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict**: this plan cannot pass, as written, and does not. Every
`missing-feature` step confirmed genuinely unwalkable; the Contract's own
merge fields never render, and what looked like a merely-unlinked PDF copy
(HS-G3) turned out to be a PDF copy that cannot be fetched at all — the same
defect silently breaks message attachments in both directions.

**Facts handed forward, not gaps**: stage 4 remains the strongest screen in
the portal with no spec on it — 4.2 signs a Contract and the suite never
does. And the "Birth Plan has no export but Print" callout the plan itself
flagged for this run is now minted as HS-G7, so it no longer needs carrying
forward.
