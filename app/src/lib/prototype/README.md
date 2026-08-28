# PROTOTYPE — #372, what the intake screen asks for

Throwaway. Nothing here goes to trunk; the branch is the primary source.

## Run it

```
cd app
bun run dev
```

Then open `/practices/p1/clients/new?variant=A`. Without `?variant=`, the route
is the real "Add a Client" page, untouched.

No API, no database, no login: everything is stubbed in `intake/fixtures.ts`.

## What it is standing in for

The search that fronts intake is already settled (#371), so this prototype
starts one step later: the search came back empty, and this is the form it
sends you to. Twelve structural columns (ADR-0017), one Practice's own field
template, and the Engagement Request's own two facts — the kind (#308) and a
nullable due date (#353).

## What to flip

- **Variant** — the bar at the bottom, or `←` / `→`.
  - `A` one page, one submit — record and ask commit together.
  - `B` two steps with a real save between them — step 1 commits her record
    alone, step 2 is the ask.
  - `C` minimal create, finish on her page — two fields on the phone, then a
    completeness strip on her record; the Request never chains.
- Nothing else. The case, what the form demands, and the field order are pinned and decided after the shape, one at a time.

Each flow ends in a black box: what was written to `clients`, whether an
`engagement_requests` row was written, and how many Credits moved (always zero
— the Credit locks at approval, #393).
