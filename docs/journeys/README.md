# Journeys

One primary journey map per [Persona](../personas/). Each file slug matches its
persona file slug one-to-one, and the test plans (`docs/test-plans/`) will match
both.

## What a journey map is here

**Not a task flow.** The destination of this effort is test plans, and that pulls
hard toward "journey map = an ordered list of clicks". Every map therefore carries
two layers:

- an **experience layer** — what the Persona is thinking, what hurts;
- an **interaction layer** — the concrete, numbered steps through the product.

Test plans derive from the interaction layer alone. `journey-gap` issues derive
from **both**, and the highest-value gaps come from the experience layer.

## Fixed structure

Every map uses the same five sections in the same order, so a test plan can cite a
step by id (`Renata 3.2`) and a gap by id (`RA-G4`):

1. **Header** — persona link, goal, entry point, done looks like.
2. **Moment of truth** — the one make-or-break moment in this journey. This is the
   lead for prioritising the gap backlog.
3. **Words** — the domain term beside the Persona's own word for it. `CONTEXT.md`
   is the language of the model and of the team; it is not automatically the copy
   on screen. Where the two diverge sharply, that divergence is a finding.
4. **Stages** — `Stage N — Title`, each with the experience layer first (**Thinking**,
   **Pain points**), then the interaction layer as numbered steps `N.1`, `N.2`.
5. **Gaps found** — a table of `<initials>-G<n>` rows, each naming its stage and
   which layer it came from.

A sixth section, **Open decisions**, is optional: questions a journey exposes but
cannot answer alone. These are not gaps and must not become `journey-gap` issues.

### One gap, one ID

A gap gets its ID on the map that **owns** it — normally the first map where it
bites hardest. Every other map that hits the same root **cites that ID** and never
mints a new one. This is what makes the deduplicated `journey-gap` backlog
possible; without it the same missing capability arrives three times under three
names. A gap that is genuinely a different question, even on the same screen, gets
its own ID.

## Status

These are drafts against **proto-personas** — assumptions grounded in the schema
and handlers, not in research. Every gap below is a hypothesis about the product,
written from reading the code, and **not one of them has been executed yet**. The
first run (`docs/test-plans/`) is the first real evidence.

## Practice side

| Map | Persona | Moment of truth |
| --- | --- | --- |
| [evaluator-doula.md](evaluator-doula.md) | Tasha Bell | The first screen after she creates a Practice |
| [solo-birth-doula.md](solo-birth-doula.md) | Maya Okonkwo | The Contract comes back signed without leaving the app |
| [practice-owner.md](practice-owner.md) | Renata Alvarez | Two Clients in labour the same night — who is free? |
| [non-doula-admin.md](non-doula-admin.md) | Dee Whitlock | Finishing the paperwork without the doula |
| [employed-doula.md](employed-doula.md) | Priya Raman | The Birth Plan, on a phone, in a hospital corridor |

## Client side

| Map | Persona | Moment of truth |
| --- | --- | --- |
| [loss-client.md](loss-client.md) | Nadia Haddad | The first screen after three weeks away |
| [first-time-client.md](first-time-client.md) | Hannah Sorensen | Printing the Birth Plan and handing it to a stranger in scrubs |
| [returning-postpartum-client.md](returning-postpartum-client.md) | Camille Boyd | "A portal account already exists for this identity" |

Nadia's map was written **first**, ahead of the two it overlaps, per the method
standard: walk the stress case before the journeys it crosses.
