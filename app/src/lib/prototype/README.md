# PROTOTYPE — #372, what the intake screen asks for

Throwaway. Nothing here goes to trunk; the branch is the primary source.

## Run it

```
cd app
bun run dev
```

Then open `/practices/p1/clients/new?variant=D`. Without `?variant=`, the route
is the real "Add a Client" page, untouched.

No API, no database, no login: everything is stubbed in `intake/fixtures.ts`.

## Where this got to

The first commit carried three shapes on a variant bar — one page and one
submit, two steps with a save between, minimal-create-then-finish. They are
still in that commit, which is what makes them the primary source.

The shape that replaced them is **D**: the third one's fast front door with the
second one's ability to keep going, laid out one thing per page (GOV.UK, Adam
Silver) with a task-list hub after the save.

- Three short pages: her name, how to reach her, her date of birth.
- **The save falls after the third page**, not the first. Everything after the
  save crosses #373's edit path, which blocks and offers only *a different
  person* — never a merge. A match key deferred past the save makes a duplicate
  that can no longer be undone, so the save waits for all four keys.
- Then a hub: what is recorded, what is not, one short page behind each row,
  and two exits — *Ask to start work with her* and *Leave it here for now*.
  "Save record" is not an action there; the save already happened, so the hub
  states it as a fact.

## Still open

Where the save falls in the three pages, the voice of the two request actions
(#374 makes them two, not one — a Doula asks, an Owner starts directly), and
the postpartum-only and returning-Client walks.

Each flow ends in a black box: what was written to `clients`, whether an
`engagement_requests` row was written, and how many Credits moved — always zero
before approval (#393).
