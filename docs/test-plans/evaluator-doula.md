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
| 1.1 | Follow a link from a search result or a Facebook group | There is nowhere to arrive. No marketing site exists | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) |
| 1.2 | Read what the product is for, looking for "doula" and "birth plan" before giving anyone an email address | No such page exists in or out of the product | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) |

**Abandon check**: she never arrives at all.

### Stage 2 — Find the price

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 2.1 | Look for a price, per month, for two doulas | No price is published anywhere, inside the product or outside it | `missing-feature (TB-G2)` [#285](https://github.com/markgoho/doula-cloud/issues/285) |
| 2.1-a | Open the only money surface that does exist, `/practices/[practiceId]/billing`, once signed up | It sells "credits" and nothing says what a credit buys | `manual` |

**Abandon check**: an unanswered price reads as expensive.

### Stage 3 — Sign up

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 3.1 | Open `/signup` | One screen, four fields, no email confirmation step. This is the strongest leg of her journey | `manual` |
| 3.2 | Fill Practice name, Your name, Email, Password | "Practice name" asks her to name a business she may not think of as one | `manual` |
| 3.3 | Press **Create Practice** | `POST /api/staff/signup` creates the Practice, the Staff row, and a membership holding Owner + Admin + Doula in one statement | `manual` |
| 3.3-a | Look for anything telling her roles exist, or that she now holds three | Nothing at signup or on the first screen does. The **Staff** screen does, one unprompted click away, and it reads `owner, office_manager, doula` — the schema's word, not **Admin** | `manual` |

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
| 5.2-a | Look for any warning that the trial has a size | None. The wall arrives at her fourth Client with a `402` ([MO-G9](https://github.com/markgoho/doula-cloud/issues/257)) | `manual` |
| 5.3 | Open the Engagement from the Clients list | The Engagement page renders | `automated (birth-plan.e2e.ts)` |
| 5.4 | Read the page | Visits, Care Plan, Birth Plan, Contract, Invoices and Messages. **The first moment the product looks like doula work** — four clicks past where she was deciding whether to leave | `manual` |

### Stage 6 — Judge the exit

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 6.1 | Export her data — any format — or delete the account | No export of any kind and no account deletion. "Can I get out again?" is unanswerable | `missing-feature (TB-G5)` [#288](https://github.com/markgoho/doula-cloud/issues/288) |

**Abandon check**: half her stated question has no answer.

### Stage 7 — Judge the way in (migrating owner)

| Step | Action | Expected result | Mark |
| --- | --- | --- | --- |
| 7.1 | Import two years of Clients from a spreadsheet | No import exists | `missing-feature (TB-G6)` [#289](https://github.com/markgoho/doula-cloud/issues/289) |
| 7.1-a | Reproduce one spreadsheet row by hand instead | Impossible: only name and email can be typed, so the row cannot be carried across even manually ([MO-G3](https://github.com/markgoho/doula-cloud/issues/252)) | `manual` |

**Abandon check**: her existing data is the reason switching is expensive, and it
cannot come with her.

## Marks

| Mark | Steps |
| --- | --- |
| `automated` | 1 |
| `manual` | 13 |
| `missing-feature` | 5 steps over 4 gaps ([TB-G1](https://github.com/markgoho/doula-cloud/issues/284), [TB-G2](https://github.com/markgoho/doula-cloud/issues/285), [TB-G5](https://github.com/markgoho/doula-cloud/issues/288), [TB-G6](https://github.com/markgoho/doula-cloud/issues/289)) |

TB-G3 (unexplained credits), TB-G4 (the admin-menu first screen) and TB-G7 (the
unsignposted roster model) are observed at 2.1-a, 4.1-a and 3.3-a: all three
screens open, and what they show is the finding. TB-G1 backs two steps.

3.3-a moved from `missing-feature (TB-G7)` to `manual` in the 2026-08-22 walk
below: the step can be performed, and what the Staff screen hands back is the
result.

Her plan is the least automatable of the six, and for a reason worth keeping: two
of her seven stages happen before the product exists, and a Playwright spec cannot
run against a marketing site that has not been written.

## Run log

### 2026-08-22 — automated steps ([#209](https://github.com/markgoho/doula-cloud/issues/209))

`bun run test:e2e` in `app/`, whole suite, one run: **16 passed, 0 failed** (20.5s).
Stack per [docs/testing.md](../testing.md) — Postgres in compose, the goose
migration, the Go BFF and the Firebase Auth emulator, all local.

| Step | Spec | Result |
| --- | --- | --- |
| 5.3 | `birth-plan.e2e.ts` | pass |

**1 automated steps: all pass.**

### 2026-08-22 — manual walk ([#233](https://github.com/markgoho/doula-cloud/issues/233))

`bun run dev:full` in `app/`, walked once end to end in a desktop browser at
1280x900 as Tasha Bell. Preconditions: none, as the plan says — signed up from
`/signup` with no fixture. Practice `Bell & Co Birth Support`, one Client
`Test Client One`. The 1 `automated` step was **not** re-run.

| Step | Mark | Result | What was seen |
| --- | --- | --- | --- |
| 1.1 | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) | as expected | Unwalkable, and the gap needs one word changed: a Hugo **scaffold** exists (`hugo/hugo.toml`, `hugo/layouts/index.html`), it is wired to its own Firebase Hosting target (`firebase.json`, target `hugo`), and what it publishes is `<h1>Hello, World!</h1>` and nothing else (`hugo/public/index.html`). There is no `content/` directory. So the site is not absent — it is empty, which is the same nowhere to arrive at |
| 1.2 | `missing-feature (TB-G1)` [#284](https://github.com/markgoho/doula-cloud/issues/284) | as expected | The words "doula" and "birth plan" appear nowhere in `hugo/` |
| 2.1 | `missing-feature (TB-G2)` [#285](https://github.com/markgoho/doula-cloud/issues/285) | as expected | No price in `hugo/`, and none in the app. **Sharper than the plan claimed**: the one screen that sells credits shows no price either — see 2.1-a |
| 2.1-a | `manual` | as expected | `Billing` — `Credit balance: 3`, one ledger row `8/22/2026, 12:43:58 PM / signup_bonus / +3`, then `Quantity [1]` and a **Buy credits** button. Nothing says what a credit buys, what one costs, or what `signup_bonus` is — the origin is printed as the raw enum. Pressing **Buy credits** used to return `internal error` (HTTP 500) with no Stripe key. Re-run 2026-08-22 against the Sandbox ([#242](https://github.com/markgoho/doula-cloud/issues/242)): it now opens Stripe Checkout and a paid Session credits the ledger from the `checkout.session.completed` webhook. **[TB-G2](https://github.com/markgoho/doula-cloud/issues/285) and [TB-G3](https://github.com/markgoho/doula-cloud/issues/286) stand unchanged** — a credit costs $5.00 and the screen still never says so, nor what a credit buys; the price is visible only once the Practice has already left for Stripe |
| 3.1 | `manual` | as expected | `/signup`: heading `Sign up your Practice`, four fields, one **Create Practice** button. No email confirmation, no terms, no link back to anything |
| 3.2 | `manual` | as expected | The first field is labelled `Practice name`, exactly as the abandon check predicted |
| 3.3 | `manual` | as expected | Landed on `/practices/{id}`. DB after: **one** `practices` row, **one** `staff` row, one `practice_memberships` row with `{owner,office_manager,doula}` — the three roles in one statement, as claimed. **The stage-3 timing claim ("under a minute") was not tested**: a driven browser types faster than a person, so any number here would be fiction |
| 3.3-a | `manual` (was `missing-feature (TB-G7)` [#290](https://github.com/markgoho/doula-cloud/issues/290)) | **falsified** | The **Staff** link on the first screen opens a roster with a `Roles` column reading `owner, office_manager, doula`. She *can* learn that roles exist, in one click. TB-G7 is narrowed, not deleted: nothing signposts it, the only Action offered is `End sessions everywhere` (no role can be changed — [RA-G2](https://github.com/markgoho/doula-cloud/issues/261), owned by Renata's map), and the word on screen is `office_manager`, which is the schema's word and not **Admin** (#204). See the journey map's revised TB-G7 row |
| 4.1 | `manual` | as expected | `Welcome to Bell & Co Birth Support` and the seven links in the order the plan lists them. One control the plan does not name: a `Sign out` button in the banner. That is the whole screen |
| 4.1-a | `manual` | as expected | The complete body text is `Sign out / Welcome to Bell & Co Birth Support / Clients Billing Invite a Staff member Staff Plan Templates Contract Template Payments`. Neither "birth plan" nor "visit" appears |
| 4.2 | `manual` | as expected | Seven links, flat, no descriptions, no empty-state prompt, no ordering cue |
| 5.1 | `manual` | as expected | `Add a Client`: two fields, `Their name` and `Their email`. Nothing on the screen mentions credits |
| 5.2 | `manual` | as expected | One `clients` row, one `engagements` row at `intake`, and a `credit_ledger` row `consumption / -1` at the identical timestamp. Nothing on screen said a credit was spent. **The plan's route is one click long, not two**: **Add Client** lands straight on the Engagement page — she never passes through the Clients list |
| 5.2-a | `manual` | as expected | No warning of any kind, on either screen |
| 5.4 | `manual` | as expected, with one correction | The Engagement page shows `Test Client One`, `Status intake` (raw enum), `Created`, `Send portal invite`, Visits, Care Plan, Birth Plan, Contract, Messages. **Three clicks from the first screen, not four** (Clients → Add a Client → Add Client), because 5.2 lands her here. The Visits table's `Date` column is `visit.createdAt` (`app/src/routes/practices/[practiceId]/engagements/[engagementId]/+page.svelte:525`) — a column labelled Date that holds when the row was typed. Corroborates **[MO-G1](https://github.com/markgoho/doula-cloud/issues/250)**; no new gap |
| 6.1 | `missing-feature (TB-G5)` [#288](https://github.com/markgoho/doula-cloud/issues/288) | as expected | Unwalkable and confirmed at the route table: `api/main.go:137-236` registers no export, no download, and no account deletion. The seven links carry no account or settings screen, and the Clients list offers no export |
| 7.1 | `missing-feature (TB-G6)` [#289](https://github.com/markgoho/doula-cloud/issues/289) | as expected | Same route table: no import endpoint exists |
| 7.1-a | `manual` | as expected | Walked and impossible, as claimed: `Add a Client` accepts a name and an email and nothing else (**[MO-G3](https://github.com/markgoho/doula-cloud/issues/252)**) |

**12 `manual` steps walked, 1 re-marked from `missing-feature`; 5 remaining
`missing-feature` steps confirmed unwalkable; 0 `blocked` steps (this plan has
none).** One gap wording was falsified and rewritten (TB-G7); two were sharpened
in place (TB-G1, TB-G2). No gap ID was minted, and no `journey-gap` issue was
filed — that is
[#209](https://github.com/markgoho/doula-cloud/issues/209).

#### Abandon checks — what a real evaluator would do

Recorded as judgement, in Tasha's words, not as pass or fail.

- **Stage 1** — She never arrives. Nothing to close the tab on.
- **Stage 2** — "What does it cost?" is unanswered before signup *and* after it.
  The Billing screen is worse than silence: it names a currency she has never
  heard of, prints `signup_bonus` at her, and the one button that might explain
  the price returns `internal error`. She would not put a card near this.
- **Stage 3** — She would not leave here. Four fields, one screen, no
  confirmation email. This is the strongest leg, exactly as the map says.
- **Stage 4** — **This is where she closes the tab.** Seven links and a Sign out
  button. She came to see whether it is built for doulas; the screen answers with
  a filing cabinet. Nothing on it says the word she was shopping for.
- **Stage 5** — If she pushes past stage 4, the Engagement page rescues her:
  Birth Plan is on it, in her own words, at three clicks. The finding is not that
  the page is bad — it is that it sits behind the screen she would have left at,
  and that reaching it costs her a credit and a Client she had to invent.
- **Stage 6** — "Can I get out again?" — no. Half her stated question has no
  answer anywhere in the product.
- **Stage 7** — Her spreadsheet cannot come with her, and cannot be retyped
  either. For a two-doula practice with two years of history, this is the
  expensive half of switching and the product has nothing to say about it.

**Verdict against "a pass means"**: a Practice with one test Client in it exists,
so the mechanical half passed. The intention to come back does not — she leaves
at stage 4, for a recorded reason. Both close the journey, and this journey
closes on the second one.
