# Tasha Bell — test plan

- **Journey**: [evaluator-doula.md](../journeys/evaluator-doula.md)
- **Persona**: [evaluator-doula.md](../personas/evaluator-doula.md)
- **A pass means**: a Practice with one test Client in it and an intention to come
  back — **or** a clear, recorded reason she left. Both close the journey.

Tasha is the only Persona who may legitimately abandon, so every stage carries an
**abandon check**: the tester records whether a real evaluator would close the tab
there. That check is the point of the plan, and it is a judgement, not an
assertion — record what was seen, in her words, not a pass or a fail.

## Preconditions

None, and that is the test. Tasha arrives from outside with no account and no
fixture. Stages 1 and 2 land on a marketing site that has not been built, so this
plan is the only practice-side one whose first steps have no product to run
against — they are `missing-feature`, not work to be done here (building the
marketing site is out of scope for this map).

## Steps

### Stage 1 — Find it and judge it in thirty seconds

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 1.1 | Follow a link from a search result or a Facebook group | There is nowhere to arrive. No marketing site exists | `missing-feature (TB-G1)` |
| 1.2 | Read what the product is for, looking for "doula" and "birth plan" before giving anyone an email address | No such page exists in or out of the product | `missing-feature (TB-G1)` |

**Abandon check**: she never arrives at all.

### Stage 2 — Find the price

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Look for a price, per month, for two doulas | No price is published anywhere, inside the product or outside it | `missing-feature (TB-G2)` |
| 2.1-a | Open the only money surface that does exist, `/practices/[practiceId]/billing`, once signed up | It sells "credits" and nothing says what a credit buys | `manual` |

**Abandon check**: an unanswered price reads as expensive.

### Stage 3 — Sign up

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/signup` | One screen, four fields, no email confirmation step. This is the strongest leg of her journey | `manual` |
| 3.2 | Fill Practice name, Your name, Email, Password | "Practice name" asks her to name a business she may not think of as one | `manual` |
| 3.3 | Press **Create Practice** | `POST /api/staff/signup` creates the Practice, the Staff row, and a membership holding Owner + Admin + Doula in one statement | `manual` |
| 3.3-a | Look for anything telling her roles exist, or that she now holds three | Nothing does. She cannot evaluate the product for her second doula because she never learns the roster model is there | `missing-feature (TB-G7)` |

**Abandon check**: low risk. Time the whole stage — under a minute is the claim.

### Stage 4 — The first screen (moment of truth)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 4.1 | Land on `/practices/[practiceId]` | `Welcome to {practice name}` and seven links: Clients, Billing, Invite a Staff member, Staff, Plan Templates, Contract Template, Payments | `manual` |
| 4.1-a | Search the screen for the words "birth plan" and "visit" | Neither appears. Six of the seven links are administration; nothing here is about supporting a birth | `manual` |
| 4.2 | Choose where to go first | No guidance on which link comes first | `manual` |

**Abandon check**: this screen. It is an empty filing cabinet, not proof (TB-G4).

### Stage 5 — Kick the tyres

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 5.1 | Open `/practices/[practiceId]/clients/new` and enter a made-up name and email | Two fields; she must invent a Client to see any real screen | `manual` |
| 5.2 | Press **Add Client** | A Client **and** an Engagement at `intake` are created — there is no way to make one without the other — and one of her three credits is silently spent | `manual` |
| 5.2-a | Look for any warning that the trial has a size | None. The wall arrives at her fourth Client with a `402` (MO-G9) | `manual` |
| 5.3 | Open the Engagement from the Clients list | The Engagement page renders | `automated (birth-plan.e2e.ts)` |
| 5.4 | Read the page | Visits, Care Plan, Birth Plan, Contract, Invoices and Messages. **The first moment the product looks like doula work** — four clicks past where she was deciding whether to leave | `manual` |

### Stage 6 — Judge the exit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Export her data — any format — or delete the account | No export of any kind and no account deletion. "Can I get out again?" is unanswerable | `missing-feature (TB-G5)` |

**Abandon check**: half her stated question has no answer.

### Stage 7 — Judge the way in (migrating owner)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Import two years of Clients from a spreadsheet | No import exists | `missing-feature (TB-G6)` |
| 7.1-a | Reproduce one spreadsheet row by hand instead | Impossible: only name and email can be typed, so the row cannot be carried across even manually (MO-G3) | `manual` |

**Abandon check**: her existing data is the reason switching is expensive, and it
cannot come with her.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 1 |
| `manual` | 12 |
| `missing-feature` | 6 steps over 5 gaps (TB-G1, TB-G2, TB-G5, TB-G6, TB-G7) |

TB-G3 (unexplained credits) and TB-G4 (the admin-menu first screen) are observed at
2.1-a and 4.1-a: both screens open, and what they show is the finding. TB-G1 backs
two steps.

Her plan is the least automatable of the six, and for a reason worth keeping: two
of her seven stages happen before the product exists, and a Playwright spec cannot
run against a marketing site that has not been written.

## Run log

Not yet run. First execution is
[#209](https://github.com/markgoho/doula-cloud/issues/209).
