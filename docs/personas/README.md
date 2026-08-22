# Personas

Eight people the journey maps (`docs/journeys/`) and test plans (`docs/test-plans/`)
refer to by name. Each file names one archetype. Rename the person freely; keep the
file slug, because journey and test-plan files correspond to it one-to-one.

## These are proto-personas

They are built from assumptions and from what the schema and handlers actually do —
**not** from interviews, surveys, or analytics. That is a legitimate and named practice,
but it carries an obligation: treat every one of them as a hypothesis to be falsified,
never as evidence of what users do. Do not cite them as user research.

Two consequences:

- The first execution of the test plans is the first real evidence. Where a persona is
  contradicted by it, the persona is wrong, not the finding.
- Nadia Haddad ([loss-client.md](loss-client.md)) is the persona most likely to be wrong
  and most costly if she is. Her journey map should not be finalised on assumption
  alone — it wants input from a doula who has supported a client through a loss.

Every file answers four questions the journey maps need:

- **Surface / Roles** — which app they use and what they may do there.
- **Entry point** — where their journey starts.
- **Primary journey** — the one goal that defines their journey map.
- **Done looks like** — the end state that closes the journey.

A `Watch for` list ends each file. Those are the friction points the journey map is
expected to hit; several are known gaps in the current schema, not oversights in the
persona.

## Needs, never capabilities

A persona file states what the person **needs**. It must never state what the product
**does** — not in "Primary journey", not in "Done looks like". Those two sections
describe the person's own goal and their own idea of finished, and a reader must be
able to trust that none of it is a claim about the code.

Every claim about what the code does belongs in `Watch for`, with a `file:line` or a
migration name. Two files broke this rule and were corrected while the practice-side
journey maps were drafted: `practice-owner.md` asserted that a Doula is assigned to
an Engagement (no such column exists), and `non-doula-admin.md` named
`payments/invoice.go` as owner-gated (it is not). Both errors read as settled fact
and would have sent a test plan looking for a screen that was never built.

## Practice side

| File | Archetype | Person |
| --- | --- | --- |
| [solo-birth-doula.md](solo-birth-doula.md) | Owner + Admin + Doula in one person | Maya Okonkwo |
| [practice-owner.md](practice-owner.md) | Multi-doula practice owner | Renata Alvarez |
| [employed-doula.md](employed-doula.md) | Employed doula, no admin rights | Priya Raman |
| [non-doula-admin.md](non-doula-admin.md) | Office manager who never works a Visit | Dee Whitlock |
| [evaluator-doula.md](evaluator-doula.md) | Prospect deciding whether to sign up | Tasha Bell |

## Client side

| File | Archetype | Person |
| --- | --- | --- |
| [first-time-client.md](first-time-client.md) | First pregnancy, full birth engagement | Hannah Sorensen |
| [returning-postpartum-client.md](returning-postpartum-client.md) | Second baby, postpartum support only | Camille Boyd |
| [loss-client.md](loss-client.md) | Pregnancy ends in loss mid-engagement | Nadia Haddad |
