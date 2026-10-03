# Journeys

One primary journey map per [Persona](../personas/). Each file slug matches its persona file slug one-to-one, and the test plans (`docs/test-plans/`) will match both.

## What a journey map is here

**Not a task flow.** The destination of this effort is test plans, and that pulls hard toward "journey map = an ordered list of clicks". Every map therefore carries two layers:

- an **experience layer** — what the Persona is thinking, what hurts;
- an **interaction layer** — the concrete, numbered steps through the product.

Test plans derive from the interaction layer alone. `journey-gap` issues derive from **both**, and the highest-value gaps come from the experience layer.

## Fixed structure

Every map uses the same five sections in the same order, so a test plan can cite a step by id (`Renata 3.2`) and a gap by id (`RA-G4`):

1. **Header** — persona link, goal, entry point, done looks like.
2. **Moment of truth** — the one make-or-break moment in this journey. This is the lead for prioritizing the gap backlog.
3. **Words** — the domain term beside the Persona's own word for it. `CONTEXT.md` is the language of the model and of the team; it is not automatically the copy on screen. Where the two diverge sharply, that divergence is a finding.
4. **Stages** — `Stage N — Title`, each with the experience layer first (**Thinking**, **Pain points**, **Budget**), then the interaction layer as numbered steps `N.1`, `N.2`.
5. **Gaps found** — a table of `<initials>-G<n>` rows, each naming its stage and which layer it came from.

A sixth section, **Open decisions**, is optional: questions a journey exposes but cannot answer alone. These are not gaps and must not become `journey-gap` issues.

### The Budget line

Thinking and Pain points say what a stage costs the Persona in words, and every gap they produce is shaped "a capability is missing". So a walk can prove she got through and still cannot say what it cost her. The **Budget** says that as a number ([#1683](https://github.com/markgoho/doula-cloud/issues/1683)). It sits in the experience layer, after Pain points and before the numbered steps, as a list with one budget line per term:

```markdown
**Budget**:

* **Screens**: 2 (Hick's Law, Flow)
* **Decisions**: 4 (Hick's Law, Miller's Law)
* **Memory**: yes (Working Memory)
* **Time**: 400 ms, 100 ms (Doherty Threshold)
* **Confirmation**: yes (Peak-End Rule)
* **Cold**: yes (Paradox of the Active User)
* **320px**: yes (ADR-0024)
```

The budget lines take `*` bullets and the numbered steps keep `-`, because two lists with the same marker and only a blank line between them render as one list.

The seven terms, in this order, each stated in the words [The Laws of UX](../design/brief.md#the-laws-of-ux) section of the design brief already uses, and each line cites the law that sets it:

| Term | Value | What it holds the stage to | Law |
| --- | --- | --- | --- |
| **Screens** | a whole number | At most this many screens crossed from the stage's entry to its end. The entry is the screen the stage's first step starts on; a screen the stage only passes through counts, and a screen outside the product (an email, Stripe's hosted pages) does not. | Hick's Law, Flow |
| **Decisions** | a whole number, never above 7 | At most this many decisions on one screen at once — a field to fill, an option to choose, a button that changes what happens next. | Hick's Law, Miller's Law |
| **Memory** | `yes` | Nothing the Persona must remember from an earlier step to complete a later one: an earlier answer a later step depends on is still on screen. | Working Memory |
| **Time** | `400 ms, 100 ms` | Each routine act in the stage completes in under 400 ms and is acknowledged within 100 ms. Over 400 ms, the work is done optimistically or with a skeleton, never under a spinner. | Doherty Threshold |
| **Confirmation** | `yes` | The stage ends in a confirmation that says what happened and what is next. On a stage that only reads, the end is the thing read, named so she knows she has the right one — whose it is and when it last changed. | Peak-End Rule |
| **Cold** | `yes` | Usable cold, on the first visit, with no tour and no tooltip; help is inline, at the field. | Paradox of the Active User |
| **320px** | `yes` | Every screen in the stage is complete and usable at 320px wide. | [ADR-0024](../adr/0024-layout-is-intrinsic-and-320px-is-a-conformance-commitment.md) |

**A budget is a number or a yes-or-no, never prose.** A reason a stage needs a looser number goes in Pain points, not on the budget line. A budget is a **target**, not a measurement: the walk measures it, and a number one walk recorded (Priya Raman's 0.83 screens of scroll and 1.7 s on a Pixel 7, [#237](https://github.com/markgoho/doula-cloud/issues/237)) is evidence that stays in the plan — in its Run log, or in the note on the budget step it bears on — never on the map.

**A stage carries the lines that apply to it, and the moment-of-truth stage carries all seven.** Screens and 320px apply to every stage with a screen of the Persona's in it. Decisions applies where she fills or chooses something, Memory where a later step needs an earlier answer, Time where the act is one she repeats, Confirmation where the stage changes something, and Cold where she meets the screen for the first time. A stage with no screen of hers in it (an act another Persona does for her) carries no Budget.

Each budget line becomes one step in the test plan, and a budget step that fails becomes a `journey-gap` on the map that owns the stage, with the budget line quoted. [`docs/test-plans/README.md`](../test-plans/README.md)'s "Budget steps" section says how each one is marked and measured.

### One gap, one ID

A gap gets its ID on the map that **owns** it — normally the first map where it bites hardest. Every other map that hits the same root **cites that ID** and never mints a new one. This is what makes the deduplicated `journey-gap` backlog possible; without it the same missing capability arrives three times under three names. A gap that is genuinely a different question, even on the same screen, gets its own ID.

## Status

These are drafts against **proto-personas** — assumptions grounded in the schema and handlers, not in research.

**All nine have now been walked** ([#209](https://github.com/markgoho/doula-cloud/issues/209)), so the gaps below are no longer hypotheses read out of the code: each one was attempted against the running product. The walks minted eleven new gap IDs, narrowed three, and falsified one map's reasoning outright (Dee's stage 5).

Every gap now carries an **Issue** column in its map's `## Gaps found` table, pointing at the `journey-gap` issue that owns the work — except where another wayfinding map owns the capability outright ([#225](https://github.com/markgoho/doula-cloud/issues/225) for roles, employment type, attachment and Offer; [#212](https://github.com/markgoho/doula-cloud/issues/212) for the Client register).

## Practice side

| Map | Persona | Moment of truth |
| --- | --- | --- |
| [evaluator-doula.md](evaluator-doula.md) | Tasha Bell | The first screen after she creates a Practice |
| [solo-birth-doula.md](solo-birth-doula.md) | Maya Okonkwo | The Contract comes back signed without leaving the app |
| [practice-owner.md](practice-owner.md) | Renata Alvarez | Two Clients in labor the same night — who is free? |
| [non-doula-admin.md](non-doula-admin.md) | Dee Whitlock | Finishing the paperwork without the doula |
| [employed-doula.md](employed-doula.md) | Priya Raman | The Birth Plan, on a phone, in a hospital corridor |
| [contractor-doula.md](contractor-doula.md) | Lena Vasquez | The offer, before she has said yes |

## Client side

| Map | Persona | Moment of truth |
| --- | --- | --- |
| [loss-client.md](loss-client.md) | Nadia Haddad | The first screen after three weeks away |
| [first-time-client.md](first-time-client.md) | Hannah Sorensen | Printing the Birth Plan and handing it to a stranger in scrubs |
| [returning-postpartum-client.md](returning-postpartum-client.md) | Camille Boyd | Both of her Engagements, in one list, under the login she already has |

Nadia's map was written **first**, ahead of the two it overlaps, per the method standard: walk the stress case before the journeys it crosses.
