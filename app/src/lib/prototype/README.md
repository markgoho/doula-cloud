# PROTOTYPE — #371, how a returning Client is found

Throwaway. Nothing here goes to trunk; the branch is the primary source.

## Run it

```
cd app
bun install   # only if node_modules is missing
bun run dev
```

Then open `/practices/p1/clients/new?variant=A` on whichever port Vite
prints (5173 unless it is already taken).

No API, no database, no login: every Client is stubbed in
`reuse/fixtures.ts`, and the practice id in the URL is ignored. Without
`?variant=`, the route is the real "Add a Client" page, untouched.

## What to flip

- **Variant** — the bar at the bottom, or `←` / `→`. Four structurally
  different answers: `A` match at intake, `B` search first, `C` start
  from her page, `D` one box plus a review sheet.
- **Case being typed** — the five cases the ticket names, including the
  two an exact-email lookup gets wrong: Priya (same woman, new email)
  and Dana (shares a household email with her mother).
- **What the prompt may print** — confirm-only, name only, or name plus
  history. This is a decision the ticket says must not be defaulted, so
  it is a switch here rather than a choice already made.

Each flow ends in a black box saying what was written and whether a
Credit was spent.
