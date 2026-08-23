# Lena Vasquez — test plan

- **Journey**: [contractor-doula.md](../journeys/contractor-doula.md)
- **Persona**: [contractor-doula.md](../personas/contractor-doula.md)
- **A pass means**: the Engagement she took is finished, she can point to what she
  agreed and what she was paid, and she never saw a Client who was not hers.

This plan is expected to fail at step 1.2 and never reach stage 3 unmodified. It is
written to be run anyway: the failures are the evidence, and the fixture bypass
below is what lets the later stages be walked at all.

## Preconditions

- **Two** Practices: the other agency where Lena already works, and Rooted Birth
  Collective. She holds a Staff account at the first one already — that is the
  point of her.
- Rooted Birth Collective with an Owner and **at least two** Clients, only one of
  which is meant to be hers.
- **Fixture bypass for stages 2–8.** Stage 1 cannot produce her second membership
  (LV-G2), so insert a `practice_memberships` row for her **existing** `staff` row
  against the second Practice, directly in Postgres. Whether the schema accepts
  that is itself unproven — `00002` promises multi-Practice membership in its own
  comment — so the run's first job is to find out, and to record the answer.
- No fixture can create an **Offer** or an employment type. Stage 3 has nothing to
  provision.

## Steps

### Stage 1 — Join the agency, on an account she already has

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Open the invite link at `/accept-invite` | The accept form renders | `manual` |
| 1.2 | Sign in as herself and submit | **Expected failure**: `InviteHandler` inserted a fresh `staff` row and acceptance writes her identity onto it, but `staff.identity_uid` is `UNIQUE` and hers is on the other agency's row. A unique-constraint violation, caught at `accept.go:104` and surfacing as a **`409`**, "a staff account already exists for this identity" | `manual` |
| 1.2-a | Check for a second membership | **None she can reach.** One *does* get written — `invite.go:56` and `:66` insert the pending `staff` row and its `roles = '{}'` membership at invite time, before anyone accepts — but it hangs off a second `staff` row her identity can never claim, so the Practice gains a member who cannot sign in and whom nothing removes (LV-G8) | `missing-feature (LV-G2)` |
| 1.3 | Record that she is a contractor, not an employee | `practice_memberships` is `(practice_id, staff_id, roles, created_at)` — no column holds it | `missing-feature (LV-G1)` |

### Stage 2 — Sign in and choose the agency

Reachable only after the fixture bypass.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Sign in at `/login` | `POST /api/session` succeeds | `automated (staff-login.e2e.ts)` |
| 2.2 | Choose Rooted Birth Collective from **two** memberships | The picker lists both. She is the only Persona for whom this is a real decision, and she meets it every time | `manual` |
| 2.2-a | Read what she is at each Practice from the picker | Nothing on it distinguishes the agency she contracts to from the one she works at | `missing-feature (LV-G1)` |
| 2.3 | Read the tiles | Owner tiles hidden; **Clients, Billing and Payments remain** — three, not two, because Payments sits outside the owner block (RA-G9). Billing is the agency's own credit spending and Payments its Stripe state — neither is her business at all, and she is not even the employee they were already wrong for (DW-G4) | `manual` |
| 2.3-a | Open **Billing** | The agency's whole credit ledger — balance, every purchase and every consumption with its date. **Buy credits** renders `disabled` for a non-owner (DW-G4) | `manual` |
| 2.3-b | Open **Payments** | The agency's Stripe Connect status, with `Ask a Practice Owner to connect Stripe.` in place of the button (RA-G9) | `manual` |

### Stage 3 — The offer (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Receive the offer of the February Engagement | Nothing offers an Engagement to anybody. There is no Offer, and no attachment for one to lead to | `missing-feature (LV-G6)` |
| 3.2 | Read enough to take or refuse it — Client, dates, on-call terms, fee — while still an outsider | No read rule covers *offered, not yet accepted*; ADR-0006's table has four columns and none is hers | `missing-feature (LV-G7)` |
| 3.3 | Decline, and have Renata see the refusal | A decline is not recorded anywhere, so silence and "no" are the same thing to the Practice | `missing-feature (LV-G6)` |

Every step of the stage her whole relationship with the product is built on is a
hole. Nothing here degrades to a manual walk-through: there is no screen to open.

### Stage 4 — Find the one job she took

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Open `/practices/[practiceId]/clients` | The list renders for a non-owner | `manual` |
| 4.2 | Count the rows | **Every Client at Rooted Birth Collective.** For Priya this is a scope failure inside one team; here it is one business reading another's book (LV-G5, same root as PR-G1) | `manual` |
| 4.2-a | Pick hers out | Only by remembering the name from the offer — no column marks it (RA-G4) | `manual` |
| 4.1-a | Add a Client to the agency's book | **Nothing refuses her.** `engagement.CreateHandler` sits behind `staffauth.Middleware` with no role check (`api/main.go:169`) and consumes a Practice credit on the way through (`engagement/create.go:97`), so an outside contractor spends the agency's money (LV-G9) | `manual` |

### Stage 5 — Check the terms and the fee

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open her Engagement | The single-page view renders | `manual` |
| 5.2 | Read the Contract section | Prose, merge fields and values in one object with no role check — **she gets the money**, on every Engagement in the Practice, not only hers. ADR-0006 says she should read it on her own work and Priya should not, which needs a split the read cannot make (PR-G2) | `manual` |
| 5.2-a | Price and send a Contract on a Client who is not hers | **Nothing refuses her.** The write side has no role check either, so an outsider can put a priced agreement in front of another business's Client (PR-G8) | `manual` |
| 5.3 | Find what *she* is owed | Nothing holds it. The Contract prices the Client's care, and her rate lives in the phone call | `missing-feature (LV-G3)` |

### Stage 6 — Do the work

Identical to Priya's stages 6–8 and marked there; walk
[employed-doula.md](employed-doula.md) steps 6.1 to 8.2 as Lena and record only
where a contractor differs. Nothing is expected to — which is the finding: the care
half is the half the product already treats correctly.

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Read the Birth Plan | As Priya 6.1–6.2, including no deep link and no handoff from her side (PR-G5) | `manual` |
| 6.2 | Log a Visit | As Priya 7.1: no date, no type, no note (MO-G1, MO-G2, PR-G6) | `manual` |
| 6.3 | Message the Client | As Priya 8.1–8.2 | `manual` |

### Stage 7 — Get paid

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Point to what she was paid for the February birth | No product step exists. `invoices` rows are `(practice_id, contract_id, …)` — the Practice billing the Client. Nothing records a Practice owing a doula, so the two sides keep separate books | `missing-feature (LV-G3)` |

### Stage 8 — The job ends

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 8.1 | End the attachment when the job is finished | Nothing expresses it, and **removing her membership is not a lever the product has** — `api/main.go` mounts no route that deletes a membership or a `staff` row. The only control on the Staff screen is **End sessions everywhere**, which ends a sign-in and not a read. Her read never lapses | `missing-feature (LV-G4)` |
| 8.1-a | Re-run step 4.1 afterwards | She still reads the agency's Clients | `manual` |

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 1 |
| `manual` | 17 (13 on the plan as drafted, plus 2.3-a, 2.3-b, 4.1-a and 5.2-a appended by the walk) |
| `missing-feature` | 9 steps over 6 gaps (LV-G1, LV-G2, LV-G3, LV-G4, LV-G6, LV-G7) |

LV-G5 is observed at 4.2 rather than given a step of its own: the list opens, and
what it shows is the finding.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 2.1 | `staff-login.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-08-23 — manual walk ([#238](https://github.com/markgoho/doula-cloud/issues/238))

`bun run dev:full` in `app/`, walked as Lena Vasquez at 1280x900. Her journey names
no device, so the desktop default stands. This plan's one `automated` step was **not**
re-run; `/login` was driven anyway, because 2.2 sits behind it.

Preconditions built as the plan requires, and in the order the plan requires — stage 1
is only a test while the bypass does not yet exist. **Willow Bend Doulas** (Beth
Alvarado, Owner) invited Lena and she accepted, which is what put her `identity_uid`
on a `staff` row and made her the person this plan is about; Beth then gave her
`doula`. **Rooted Birth Collective** (Renata Alvarez, Owner) was signed up separately
with two Clients — **Adaeze Nwosu**, the February birth offered to Lena, and
**Tabitha Nunes**, who is not hers — a filled Birth Plan and Care Plan on Adaeze's
Engagement, a Contract priced `$2,400` and `sent`, and Adaeze's portal invite accepted.
Renata then invited Lena to Rooted, and that invitation **is** steps 1.1 and 1.2, so it
was not provisioned away.

**The fixture bypass, and the answer the plan asked it for.** Postgres took the insert:
`INSERT INTO practice_memberships (practice_id, staff_id, roles) VALUES (<rooted>,
<lena's existing staff id>, '{doula}')` returned one row. `00002`'s own comment is
**true of the schema and false of the API** — the table's only relevant constraint is
`UNIQUE (practice_id, staff_id)`, which a second Practice does not touch. `roles` was
the session's call, not the plan's: `{doula}` is what her Persona says she is, 2.3
expects a non-owner's tiles, and `visit/roles.go:40` would 403 her out of 6.2 without
it. Recorded here rather than asked.

**Four steps were appended by the walk** — 2.3-a and 2.3-b (the cell says Billing is
not her business; whether she can actually read it is the finding), 4.1-a and 5.2-a
(both stages test her *reads* and neither tested what she can write). Dee's 7.2-a and
Priya's 5.2-a are the precedent.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `manual` | as expected | `/accept-invite?token=…` renders `Accept your Staff invite`, an Email box, a Password box, an **Account mode** radio pair and one **Accept invite** button. `scrollWidth` equals `clientWidth`. The tab title is empty (DW-G8) |
| 1.2 | `manual` | as expected | **`409 POST /api/staff/accept-invite`**, printed on the screen as `a staff account already exists for this identity`. Exactly the refusal `accept.go:104` is written to give, from the unique violation on `accept.go:99`'s `UPDATE staff SET identity_uid` |
| 1.2-a | `missing-feature (LV-G2)` | **falsified, and it mints a gap** | A second membership **does** exist — `practice_memberships` carries a Rooted Birth Collective row for Lena with `roles = '{}'`, because `invite.go:56` and `:66` write the pending `staff` row and its membership at **invite** time, in a different request from the one that fails. The rollback of the failed accept cannot reach them. So the 409 is clean and the data is not: Renata's Staff screen prints **two `Lena Vasquez` rows on the same email**, one `no roles yet` and one `doula`, and the only control under **Actions** on either is **End sessions everywhere**. The phantom is indistinguishable from a real invitee who has not been given roles yet (Priya's acceptance leaves exactly that state), and no route in `api/main.go` deletes a membership or a `staff` row. New gap **LV-G8**. LV-G2 stands: she still has no membership she can reach |
| 1.3 | `missing-feature (LV-G1)` | as expected | Confirmed unwalkable against the live table: `practice_memberships` is `id, practice_id, staff_id, roles, created_at` and nothing else. No column holds an employment type |
| 2.1 | `automated (staff-login.e2e.ts)` | not re-run | Already green in the 2026-08-22 suite run |
| 2.2 | `manual` | as expected — **and it is the first time this screen has ever rendered** | Signing in at `/login` lists **`Rooted Birth Collective`** and **`Willow Bend Doulas`** under `Choose a Practice`, each linking to its own `/practices/{id}`. Renata's 1.2, Dee's 1.3 and Priya's 3.2 each recorded "there is nothing to choose"; the picker was never missing, it needed two memberships, and only the bypass can produce them. One detail no plan names: it renders **on `/login` itself**, below the still-visible Email, Password and **Log in** controls, rather than on a page of its own |
| 2.2-a | `missing-feature (LV-G1)` | as expected | Confirmed unwalkable. Two bare links, Practice name only. Nothing says what she is at either — not a role, and not the employment type that does not exist |
| 2.3 | `manual` | **falsified** | **Three** tiles, not two: `Clients`, `Billing`, **`Payments`**. Invite, Staff, Plan Templates and Contract Template are gone. Payments is outside the owner block — RA-G9, now seen from a fifth membership and the first one belonging to a person outside the business |
| 2.3-a | `manual` | **new** | `200 GET .../billing`. She reads `Credit balance: 1` and the whole ledger — `signup_bonus +3` and two `consumption -1` rows with timestamps. An outside contractor reads what the agency has bought and every time it has spent. **Buy credits** renders `disabled` (DW-G4) |
| 2.3-b | `manual` | **new** | `200 GET .../payments/connect`. `Stripe Connect status: Not connected`, then `Ask a Practice Owner to connect Stripe.` and no button. The refusal is a hidden control, as Priya's PR-B5 found — but the *status* of another business's payment processing is handed to her before it (RA-G9) |
| 3.1 | `missing-feature (LV-G6)` | as expected | Confirmed unwalkable, and confirmed absent rather than hidden: `offer` and `decline` appear nowhere in `api/db/migrations`, `api/internal` or `app/src` except as English in comments and test names. There is no screen to open |
| 3.2 | `missing-feature (LV-G7)` | as expected | Confirmed unwalkable. `engagements` is `id, client_id, practice_id, status, created_at` — no staff column at all, so there is no *offered* state for a read rule to describe |
| 3.3 | `missing-feature (LV-G6)` | as expected | Confirmed unwalkable. Nothing records a decline, so silence and "no" remain the same thing to the Practice |
| 4.1 | `manual` | as expected | `200 GET .../clients`. The list renders for a non-owner |
| 4.2 | `manual` | as expected | **Two rows, both of them Rooted Birth Collective's: `Adaeze Nwosu` and `Tabitha Nunes`.** One is hers. LV-G5 confirmed against a Practice built to test it: one business reading another's book |
| 4.2-a | `manual` | as expected | The table is two columns, `Name` and `Status`. Nothing marks which is hers; picking it out means remembering the name (RA-G4) |
| 4.1-a | `manual` | **new — and the worst result on this plan** | Lena, holding `doula` at a Practice she does not belong to, opened **Add a Client**, entered `Wanda Peel`, pressed **Add Client** and got `201`, landing on a new Engagement. **The agency's credit balance went 1 -> 0** and a `consumption -1` row appeared in Renata's ledger. `POST .../clients` is mounted behind `staffauth.Middleware` alone (`api/main.go:169`) and `engagement/create.go:97` consumes the credit inside the same transaction, so an outsider spends the Practice's money and no role check stands anywhere between the two. New gap **LV-G9** |
| 5.1 | `manual` | as expected | Adaeze's Engagement opened on the first try. Seven `GET`s, all `200`: the Engagement, Visits, Messages, both plans, the Contract and the Invoices. No read path is role-checked |
| 5.2 | `manual` | as expected | She gets the money: `Price` reads `$2,400`, and scope, both dates and both names are filled. PR-G2 confirmed from its other half — ADR-0006 says Lena *should* read this on her own work and Priya should not, and the read cannot tell them apart |
| 5.2-a | `manual` | **new** | On **Tabitha Nunes** — a Client of the agency who is not hers — Lena pressed **Create Draft Contract** (`201`), set `scope_of_service` to `Lena wrote this` and `price` to `$99` (`200 PUT`), and **sent it to the Client** (`200 POST .../contract/send`). PR-G8 confirmed and escalated: Priya's version is an employee writing money inside her own Practice; this is an outside business putting a priced agreement in front of another business's Client |
| 5.3 | `missing-feature (LV-G3)` | as expected | Confirmed unwalkable. Nothing on the page holds her rate. The `Price` field is the Client's fee, and the Invoices section bills the Client |
| 6.1 | `manual` | as expected, and it corroborates two of Priya's | `200 GET .../plans/birth_plan` renders the snapshot — `Birth center`, `Partner`/`Doula`/`Midwife` ticked, `Low light, my sister on the phone. Ask before any exam.`, photos consented. The **Birth Plan** heading sits at y=567 of a 1445px page at 1280x900, and the page renders **0** `<a>` elements, so there is no deep link and no way out (PR-G5, PR-G9) |
| 6.2 | `manual` | as expected | **Add a Visit** -> `201`, and the Visits table gained `Lena Vasquez`, `8/23/2026`. The `Date` column is `created_at` (MO-G1); no type and no note anywhere (MO-G2, PR-G6). The only other control on the row is `Reassign to Staff id` (RA-G10) |
| 6.3 | `manual` | as expected | `201 POST .../messages`, appended to the one Engagement thread as `Lena Vasquez (staff) — 8/23/2026, 9:12:13 AM`. Nothing about it differs for a contractor — which is the finding stage 6 was written to get |
| 7.1 | `missing-feature (LV-G3)` | as expected | Confirmed unwalkable against the live schema. `invoices` is `(practice_id, contract_id, stripe_invoice_id, status, amount_cents, currency, …)` — a Practice billing a Client. None of the database's twenty tables records a Practice owing a doula |
| 8.1 | `missing-feature (LV-G4)` | as expected, and the cell understated it | Confirmed unwalkable, and the workaround the cell names does not exist either: `api/main.go` mounts no route that removes a membership or a `staff` row. The one lever an Owner has on the Staff screen is **End sessions everywhere** (`DELETE .../staff/{staffId}/sessions`), which ends a sign-in, not a read. Renata pulled it: `204` |
| 8.1-a | `manual` | as expected | Lena signed in again immediately afterwards and read `Adaeze Nwosu` and `Tabitha Nunes`, `200`. After the job is done, and after the only lever the product gives an Owner, an outside contractor still reads the agency's whole book |

**17 `manual` steps walked (2.3-a, 2.3-b, 4.1-a and 5.2-a added by the walk); 9
`missing-feature` steps confirmed unwalkable; no `blocked` step on this plan.** Three
expected results were falsified — 1.2-a, 2.3 and 8.1 — and none was re-marked: 2.3 is a
performable step whose claim was wrong, and 1.2-a and 8.1 are still unwalkable, only
for a sharper reason than the cell gave ([#235](https://github.com/markgoho/doula-cloud/issues/235)'s
precedent both times). Two gaps minted on the journey map that owns the stages,
**LV-G8** and **LV-G9**. No `journey-gap` issue was filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

**Verdict against "a pass means": it does not pass, and it fails on all three
clauses.** The Engagement she took cannot be finished — no status moves and nothing
ends an attachment. She can point to what she *agreed* only by reading the Client's
price, and to what she was *paid* not at all. And she saw every Client the agency has,
before and after the job, plus its credit ledger and its Stripe status. Stage 3, her
moment of truth, has no screen in it at all: she never weighed the job, because the
product cannot offer one. What the walk adds to the map's argument is the direction of
the leak — the map wrote her as someone the product tells too much, and 4.1-a and 5.2-a
show it also lets her *do* too much, spending the agency's credits and pricing its
Clients' care.

