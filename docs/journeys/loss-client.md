# Nadia Haddad — the Engagement that ends without a living baby

- **Persona**: [loss-client.md](../personas/loss-client.md)
- **Goal**: leave, be supported, and not be spoken to as though the pregnancy is
  still happening
- **Entry point**: already signed in to `/portal`. Her journey turns mid-way, not
  at the start
- **Done looks like**: the record is accurate, Maya can still support her, and no
  screen, label, or notification addresses her as a pregnant person

Written first, before the two other client maps, because the method standard says
to walk Nadia before any journey hers overlaps. Everything below was read from the
code; none of it has been executed.

## Moment of truth

**Stage 4 — the first screen after three weeks away.** Every other stage is
something the Practice does or fails to do. This is the one moment Nadia herself
chooses to come back, and the screen she meets decides whether she ever opens the
portal again. Today that screen greets her by name of Practice, shows her a status
that has not moved since intake, and offers her a Birth Plan.

## The cruelty is static, not automated — today

The product **cannot yet be cruel by automation.** There is no reminder machinery
anywhere in `api/` (no match for "reminder" in any Go file), Visits carry no date
so nothing can be scheduled off one (**MO-G1**), and the only push that exists is
content-free and fires on a new Message (ADR-0002). Nothing will email her "your
baby is due in 3 weeks", because nothing can.

So every wound on this map is a **static** one: a word on a screen, a link that
will not go away, a status that cannot move. That is a finding, not a reprieve:
it means the requirement lands on work not yet done. **Every future gap that adds
scheduling, reminders, due dates, or automated copy inherits this journey as a
constraint.** See Open decisions.

## Words

| Domain term | What Nadia says | Note |
| --- | --- | --- |
| Engagement | "Maya", "the doula thing" | The register says **my care** / "Your care". She is unlikely to name it at all |
| Engagement status | — | She has no word for it, and the portal shows her the raw enum (`intake`) — [#212](https://github.com/markgoho/doula-cloud/issues/212) |
| Birth Plan | "the birth plan" | Unchanged, and that is the problem: the document is named for an event that produced no living baby |
| Visit | "when Maya came" | No client-facing surface (`CONTEXT.md`), so nothing to name |
| Contract | "what I signed" | The portal shows her the word **voided** |
| Invoice | "what I still owe" | No client-facing surface at all — NH-G6 |

## Stages

### Stage 1 — A live Engagement, four months in

**Thinking**: nothing about the software. It works.
**Pain points**: none yet. This stage exists to fix what the record holds before
the turn: a signed Contract, a filled Birth Plan Instance, eleven Visits, and a
message thread.

- **1.1** — Sign in at `/portal/login`, land on
  `/portal/engagements/[engagementId]` (`GET /api/portal/session` →
  `decidePortalLanding`).
- **1.2** — Read and send messages in the one continuous thread.

### Stage 2 — The loss; the Practice tries to mark it

**Thinking**: none — she is not in the app. This stage is Maya's.
**Pain points**: there is nothing to mark and no way to mark it. `engagements` is
`(id, client_id, practice_id, status, created_at)` — the row can hold **what the
Engagement is at**, and nothing about **what happened**. And of the four statuses,
none is true: `active` says the pregnancy continues, `completed` says the work
finished, `postpartum` is the nearest and still describes a birth with a baby at
the end of it. Even had a truthful value existed, no code anywhere runs
`UPDATE engagements` (**MO-G4**), so it could not be set.

- **2.1** — Maya looks for a way to record what happened. **No screen does this.**

### Stage 3 — Three weeks of not opening the portal

**Thinking**: "I can't."
**Pain points**: none the product inflicts today — see the static-cruelty note
above. Staff Messages still reach her as a content-free push that wakes a fetch,
which is the correct transport for this moment: the push itself says nothing.

- **3.1** — Messages from Maya arrive in the thread. Her device is woken by a
  content-free push (ADR-0002); the content is fetched only if she opens it.
- **3.2** — She does not open it. Nothing escalates, because nothing can.

### Stage 4 — She opens the portal — moment of truth

**Thinking**: "Please do not ask me how far along I am."
**Pain points**: three, all on one screen.

1. The `<h1>` is **"Welcome to Rooted Birth Collective"** — an unconditional
   greeting written for a first visit, rendered on every visit forever (NH-G4).
2. **Status: `intake`** — the raw enum ([#212](https://github.com/markgoho/doula-cloud/issues/212)),
   and still `intake` because status never moves (**MO-G4**). The screen tells her
   her care is just getting started.
3. The second link on the page is **Birth Plan** (NH-G2).

- **4.1** — Open `/portal/engagements/[engagementId]`
  (`GET /api/portal/engagements/{id}` → `practiceName`, `status`, `createdAt`).
- **4.2** — Read the heading, the status, and the created date. The created date
  is the only date the portal has — there is no due date anywhere (**MO-G3**), and
  after a loss the date her care began is a strange thing to be shown.
- **4.3** — See the **Birth Plan** and **Contract** links, then the message thread.

### Stage 5 — The Birth Plan will not go away

**Thinking**: "I don't want to see that."
**Pain points**: the link is fixed in the page, second from the top, and nothing
can retire it. The Plan Instance can be overwritten by Staff, never archived or
hidden, and deletion is ruled out — the Engagement is a permanent record. Opening
it renders the full document with a **Print** button.

- **5.1** — The link is present whether or not she taps it.
- **5.2** — If she taps it: the filled Birth Plan, read-only, with **Print**
  (`GET .../birth-plan`). If no Instance exists the page says "No Birth Plan has
  been created for this Engagement yet" — which is the friendlier of the two
  outcomes here, by accident.

### Stage 6 — Money, and the word "voided"

**Thinking**: "Am I still being charged for this?"
**Pain points**: the portal has **no Invoice surface at all** (NH-G6), so the one
question she is most likely to have has no answer on screen. And when Maya voids
the Contract (Dee's stage 7), the portal shows her `Status: voided` plus
"Voided — this Contract is no longer active." — the ledger's word, delivered with
no human context, on the document she signed when she was pregnant (NH-G5).

- **6.1** — Open the Contract page (`GET .../contract`).
- **6.2** — Read the status. A `voided` Contract still renders in full; only the
  **Sign** form is withheld (`status === 'sent'`). The signed PDF endpoint exists
  (`main.go:226`) but is not linked from this page (HS-G3).

### Stage 7 — Postpartum support continues anyway

**Thinking**: "Maya is still coming. That's the only part that's right."
**Pain points**: the thing that works is the thing the model records least. A
bereavement Visit is a row with a Staff name and a creation timestamp — no type
(**PR-G6**), no date (**MO-G1**), no notes (**MO-G2**) — and the Client never sees
Visits at all. The support is real; the record of it is empty.

- **7.1** — Maya logs Visits after the loss. They are indistinguishable from the
  eleven prenatal ones.
- **7.2** — The message thread continues, unchanged and unchangeable — immutable
  by design (`CONTEXT.md`, ADR-0002), which is correct, and with no way for either
  of them to mark that the thread's subject has changed.

### Stage 8 — The record closes without erasing her

**Thinking**: she stops opening the portal.
**Pain points**: nothing closes. The Engagement sits at `intake` forever; there is
no terminal state that is true (NH-G1) and no transition machinery to reach one
(**MO-G4**). "Done looks like" is unreachable today: the record is permanent,
which is right, and permanently wrong, which is not.

- **8.1** — No step. There is nothing to click.

## Gaps found

| ID | Stage | Layer | Gap |
| --- | --- | --- | --- |
| NH-G1 | 2, 8 | Both | `engagement_status` has no value that is true after a loss. ADR-0005 binds one fixed Client label per status for every Client: `active` → Ongoing and `completed` → Care ended are both lies here, and no conditional label is permitted — so by the register's own rule the **status set is missing a value**. Distinct from **MO-G4**, which is the missing transition; this is the missing value. Both must be closed for her journey to work. |
| NH-G2 | 5 | Both | A Birth Plan cannot be retired. The portal link is unconditional and the Instance can only be overwritten by Staff — there is no archive, hide, or client-side dismiss, and deletion is correctly ruled out. |
| NH-G3 | 2 | Both | An Engagement records no **outcome**. `engagements` carries status and nothing else, so even with NH-G1 closed, what happened has nowhere to live except a message. |
| NH-G4 | 4 | Experience | The portal home `<h1>` is an unconditional "Welcome to {practiceName}" — a first-visit greeting shown on every visit for the life of the Engagement. Not a register word, so **not** covered by [#212](https://github.com/markgoho/doula-cloud/issues/212); it must be worked with it. |
| NH-G5 | 6 | Both | A voided Contract reaches the Client as the word `voided` and a bare terminal notice. The Client portal reuses the Staff `ContractStatus` component; nobody chose those words for a Client, let alone this one. |
| NH-G6 | 6 | Interaction | The portal has no Invoice, balance, or payment surface. "What do I still owe?" cannot be answered on screen by any Client, and after a loss it is the question that cannot be asked out loud. |
| NH-G7 | 3, 7 | Experience | The message thread is the only channel and has no controls: no way for her to pause push, and no way for Maya to signal that the thread's subject has changed. Push unregistration happens only at sign-out. |
| NH-G8 | 6 | Interaction | A voided Contract's signed PDF is unreachable, not merely unlinked. `serveSignedPDF` (`api/internal/contracts/signed_pdf.go:66-69`) queries `WHERE engagement_id = $1 AND status = $2::contract_status` bound to `statusSigned`, so once a Contract leaves `signed` — voided included — the query returns no row even though `signed_pdf_object_path` still points at a PDF that still exists in the store. Found walking 6.2-a ([#239](https://github.com/markgoho/doula-cloud/issues/239)): a direct `fetch()` of the routed URL 404s. Distinct root from **HS-G3**, which is the missing link on a Contract that is still `signed` — closing HS-G3 alone would not give Nadia her copy back. The same function backs the Staff-side route (`main.go:208`), so a Practice also loses its own copy of a voided Contract's PDF; that half is a fact for whichever map takes up voiding on the Staff side, not asserted as a finding of this one. |

Also hit here, filed on their owning maps: **MO-G4** (Engagement status never
changes), **MO-G1**, **MO-G2** and **PR-G6** (Visits are dateless, note-less and
typeless, so the bereavement Visit is unrecordable), **MO-G3** (no due date and
no Client detail), and [#212](https://github.com/markgoho/doula-cloud/issues/212)
(raw `engagement_status` and "Engagement" in portal copy).

## Open decisions

Not gaps, and not `journey-gap` issues. Model questions here are out of scope for
this effort and are parked on
[#224](https://github.com/markgoho/doula-cloud/issues/224).

- **Every future scheduling, reminder, or automated-copy feature inherits this
  journey.** The product is safe today only because those features do not exist.
  When a Visit gains a date, when an Invoice gains a reminder, when a due date
  becomes first-class — each one must state what it does on an Engagement in
  Nadia's state, and that requirement should be carried in the gap issue that
  introduces the capability, not discovered afterwards.
- **What the terminal state is called, and who may set it.** NH-G1 says a value is
  missing; it does not say what the value is, whether it is one value or a
  status-plus-outcome pair (NH-G3), or whether the Client may ever set it herself.
  Wants its own decision ticket.
