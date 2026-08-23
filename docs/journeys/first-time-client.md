# Hannah Sorensen — from a link in a text message to a Birth Plan in her hand

- **Persona**: [first-time-client.md](../personas/first-time-client.md)
- **Goal**: one thread with her doula, and her preferences written down somewhere
  she can hand to a hospital
- **Entry point**: a portal invitation link, accepted at `/portal/accept-invite?token=…`
- **Done looks like**: a portal account, a signed Contract, a Birth Plan she has
  read and printed, a thread with traffic both ways, and an Engagement that
  reflects where she actually is

She is the full-arc Client: the only Persona who walks every client-facing screen
the product has. Where her path crosses Nadia Haddad's, Nadia's map was walked
first ([loss-client.md](loss-client.md)).

## Moment of truth

**Stage 6 — printing the Birth Plan and handing it to a stranger in scrubs.** This
is the only moment the product leaves the screen and enters the room where the
birth happens. It is also the moment the document's authorship shows: she is
handing over a page written *about* her by someone else, which she could read but
never correct (HS-G2).

## Words

| Domain term | What Hannah says | Note |
| --- | --- | --- |
| Engagement | "Maya", "my doula" | The register says **my care** / "Your care". The portal says "Engagement" — [#212](https://github.com/markgoho/doula-cloud/issues/212) |
| Engagement status | "am I still…?" | She is shown the raw enum (`intake`) — [#212](https://github.com/markgoho/doula-cloud/issues/212) |
| Birth Plan | "my birth plan" | Matches — and it is the one thing she thinks of as **hers**, which the product does not agree with |
| Contract | "the paperwork" | |
| Visit | "when she comes over" | No client-facing surface at all (`CONTEXT.md`) |
| Client portal | "the doula site" | She has one prior reference point, her dentist's portal, and she disliked it |
| Due date | "I'm 18 weeks" | The product holds no due date and shows her the date her care was **created** instead (**MO-G3**) |

## Stages

### Stage 1 — Getting invited

**Thinking**: "Is this real?"
**Pain points**: there is no email infrastructure at all
(`portalinvite/invite.go`, InviteResponse doc comment), so the invite endpoint
hands Maya a raw one-time token and Maya pastes a URL into a text message. Hannah,
who is pregnant and reading everything, receives an unbranded link with a token in
it from a small business, and has no way to verify it. This is **RA-G1**'s root and
**PR-G7**'s shape, in the population where a phishing-shaped link lands hardest —
cited, not re-filed.

- **1.1** — Maya calls `POST /api/practices/{id}/engagements/{id}/portal-invite`
  from the Engagement page and gets back `{clientPortalUserId, inviteToken}`.
- **1.2** — Maya sends `…/portal/accept-invite?token=<token>` by hand.

### Stage 2 — Making an account

**Thinking**: "Another password."
**Pain points**: the form asks her to choose between "I'm new here — create an
account" and "I already have an account — log in" before she knows which she is;
for a first-time Client the second option can only fail. Password minimum is six
characters and nothing else is asked — no name, because the Practice already
holds it.

- **2.1** — Open `/portal/accept-invite?token=…`. With no token the page shows
  "Missing invite token" and nothing else.
- **2.2** — Enter email and password, leave the mode radio on **signup**, press
  **Accept invite** (`POST /api/portal/accept-invite`). The session cookie is set
  on that response (#145).
- **2.3** — `GET /api/portal/session` → one Engagement → redirect straight to it.

### Stage 3 — The first screen

**Thinking**: "OK. So what is here?"
**Pain points**: the heading is **"Welcome to Rooted Birth Collective"** — the
Practice's name, not hers, and unconditional forever (NH-G4). Under it: **Status:
`intake`** (raw enum, [#212](https://github.com/markgoho/doula-cloud/issues/212))
and **Created**, a date that means nothing to her — the product has no due date to
show instead (**MO-G3**). Then two links and the message thread. There is no
explanation of what any of it is for.

- **3.1** — `GET /api/portal/engagements/{id}` → `practiceName`, `status`,
  `createdAt`.
- **3.2** — See the **Birth Plan** and **Contract** links.
- **3.3** — The page registers a push subscription once per device, fire-and-forget
  (#61) — she is never told this, and there is no notification setting anywhere.

### Stage 4 — Signing the Contract

**Thinking**: "This is the bit that makes it official."
**Pain points**: none in the flow itself — this is the strongest screen in the
portal. The ordering around it is implicit: Maya cannot get a signature before
sending the invite, and nothing on the Staff side says so (**MO-G6**). A Draft
Contract is invisible here — RLS gives her no row and the page says "No Contract
has been sent for this Engagement yet", which is the right message.

- **4.1** — Open the Contract link (`GET .../contract`) → prose with merge-field
  values and `Status: sent`.
- **4.2** — Type her full legal name, tick the attestation, submit
  (`POST .../contract/sign`). The page re-renders at `signed`.
- **4.3** — Try to keep a copy. **She cannot from this page**: the signed-PDF
  endpoint is routed (`main.go:226`) and never linked (HS-G3).

### Stage 5 — Reading the Birth Plan

**Thinking**: "That's not quite what I said about the epidural."
**Pain points**: the Birth Plan is **staff-drafted and read-only to her**
(`CONTEXT.md`). Her only route to a correction is to type it into the message
thread and hope it is transcribed. The document she will hand to a hospital is one
she cannot author, cannot annotate, and cannot confirm she has read (HS-G2). Before
Maya fills it in, the page says "No Birth Plan has been created for this Engagement
yet" — with no indication of whether one is coming, or when.

- **5.1** — Open the Birth Plan link (`GET .../birth-plan`) → the filled Instance,
  read-only.
- **5.2** — Message Maya about the two things she wants changed.

### Stage 6 — Print it and hand it over — moment of truth

**Thinking**: "Please read this."
**Pain points**: the mechanism works — a real print stylesheet hides the Back link,
the Print button, and the chrome
(`birth-plan/+page.svelte`, `@media print`). What is missing is everything around
it: no PDF, no share link, no way for the hospital to receive it except paper she
remembered to print in advance, and no way for Maya to hand it over on her behalf
(**PR-G5**, from the Doula's side of the same moment).

- **6.1** — Open the Birth Plan on her phone or laptop and press **Print**.
- **6.2** — Hand the paper over.

### Stage 7 — Living in the thread

**Thinking**: "Is it normal that…?"
**Pain points**: none identified — this is the part of the product built for her.
Messages are immutable and kept as a permanent record (`CONTEXT.md`, ADR-0002),
attachments are supported both ways with inline image previews, older messages
page in on demand, and a content-free push wakes a fetch so a new Message appears
without a reload. What she is never told is that the push is not an alarm: ADR-0002
is explicit that it is not a substitute for a phone call in a time-critical moment,
and nothing on screen says so (HS-G4).

- **7.1** — Send a message at 11pm from her phone, optionally with a photo
  (`POST .../messages`, `multipart/form-data`).
- **7.2** — Receive Maya's reply in the same thread; the open tab refetches on the
  push message (#61).
- **7.3** — Scroll back through the whole Engagement's history ("Load older").

### Stage 8 — Birth, and afterwards

**Thinking**: "Everything's different now."
**Pain points**: nothing in the portal changes, ever. Her Engagement is still
`intake` (**MO-G4**) through pregnancy, birth, and postpartum, so the status line
on her home screen has been wrong since the day she signed. Her Visits — including
the birth — are invisible to her (`CONTEXT.md`: no client-facing Visit surface),
and the Birth Plan link stays exactly where it was.

- **8.1** — Open the portal. It looks the same as it did at 18 weeks.

### Stage 9 — Her partner asks for the login

**Thinking**: "Just use mine, I suppose."
**Pain points**: `client_portal_users` holds one `identity_uid` per Client row, and
`CONTEXT.md` names portal access for a second person a **future extension of
Client**, not a new entity. So there is no second account, no invite, and no
read-only guest — the supported path is sharing her password, which is the one path
nobody designed (HS-G5).

- **9.1** — No step. There is nothing to click.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| HS-G1 | 2 | Experience | The accept-invite form makes her choose "new here" or "I already have an account" before she can know which she is, and the wrong choice fails with a generic "Accept invite failed". |
| HS-G2 | 5, 6 | Both | The Birth Plan is hers in every sense except authorship: staff-drafted, read-only to the Client, with no comment, no suggested edit, and no acknowledgement that she has read it. The correction route is the message thread and a re-type by Staff. |
| HS-G3 | 4 | Interaction | The Client cannot get a copy of what she signed. `GET /api/portal/engagements/{id}/contract/pdf` is routed (`main.go:226`) but nothing in `portal/…/contract/+page.svelte` links it. |
| HS-G4 | 3, 7 | Experience | Push is registered silently on first landing and can never be reviewed, muted, or explained. There is no notification setting, and nothing tells her that a push is not an alarm (ADR-0002). Nadia's NH-G7 is the same absence at its cruellest. |
| HS-G5 | 9 | Both | A second person cannot be given portal access. One `identity_uid` per Client, no guest role, no read-only share of the Birth Plan — so partners are onboarded by password-sharing. `CONTEXT.md` marks this a future extension of Client. |
| HS-G6 | 4, 7 | Interaction | Retrieving a stored object 500s outright — the write succeeds and the object is present in the store, but nothing can read it back. Confirmed on both the signed Contract PDF (`GET .../contract/pdf`, sharpening HS-G3: fixing the missing link would not fix this) and a message attachment, in both directions. `objectstore.GCSStore.Get` (`api/internal/objectstore/gcs.go`) is backed by a `storage.Client` built with no options (`main.go:263`); whether this also breaks against real GCS or is local-emulator-only is unverified — walked at [#240](https://github.com/markgoho/doula-cloud/issues/240). |
| HS-G7 | 6 | Interaction | The Birth Plan has no export mechanism of its own beyond the browser's **Print** — no PDF, no share link, from the Client's own side. Distinct from PR-G5, which is the same absence from the Doula's side of the same moment. Walked at [#240](https://github.com/markgoho/doula-cloud/issues/240). |

Also hit here, filed on their owning maps: **RA-G1** and **PR-G7** (no email
infrastructure, so her invite arrives as an unverifiable link in a text message),
**MO-G3** (no due date and no Client detail — the portal shows her the created
date instead), **MO-G4** (her Engagement is `intake` for its whole life),
**MO-G6** (invite-before-signature ordering is implicit), **PR-G5** (no phone-first
handoff of the Birth Plan from the Doula's side), **NH-G4** (the unconditional
"Welcome to {practiceName}" heading), **NH-G6** (no Invoice surface in the portal —
she never sees what she owes or what she has paid), and
[#212](https://github.com/markgoho/doula-cloud/issues/212) (raw `engagement_status`
and "Engagement" in portal copy).

## Open decisions

Not gaps, and not `journey-gap` issues. Model questions here are out of scope for
this effort and are parked on
[#224](https://github.com/markgoho/doula-cloud/issues/224).

- **Who authors a Birth Plan?** HS-G2 says she cannot. It does not say whether the
  fix is client-editable fields, a suggest-and-approve loop, or an
  acknowledgement-only signal. This is a product decision, not a missing button,
  and it interacts with the Plan Instance snapshot rule (ADR-0001).
