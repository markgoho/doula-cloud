# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`GLOSSARY.md`** at the repo root, or
- **`GLOSSARY-MAP.md`** at the repo root if it exists — it points at one `GLOSSARY.md` per context. Read each one relevant to the topic.
- **`docs/adr/`** — read ADRs that touch the area you're about to work in. In multi-context repos, also check `src/<context>/docs/adr/` for context-scoped decisions.

**Look up one `GLOSSARY.md` term instead of reading the file whole.** Each entry is one line that opens with the term in bold, and a line can run to 4,000 characters, so a plain `grep -n` returns tens of thousands. Match the exact bold name and cut the output to a width:

```sh
grep -n '^\*\*Engagement\*\*:' GLOSSARY.md | cut -c1-600      # the entry itself
command grep -no '.\{0,80\}Engagement Request.\{0,80\}' GLOSSARY.md    # where other entries use the term
```

`command` matters on the second line: in a Claude Code shell `grep` is a function that runs `ugrep`, which refuses a bounded `.{0,80}` over UTF-8 text as too complex. Widen the `cut` only when the cut-off text is the part you need.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## Numbering a new ADR

The next number is one past the highest already in `docs/adr/`, and a number is never reused. **Take it as late as you can** — right before the PR lands, not when the branch is cut. Two PRs that each pick a number early and merge minutes apart both pick the same one, which is how ADR-0033 came to name two decisions ([#1053](https://github.com/markgoho/doula-cloud/issues/1053)).

`app/src/lib/adrNumbers.usage.spec.ts` fails the build on a repeated number. It catches the collision on the PR when the number is already taken on trunk, and on trunk's own run when two PRs took it concurrently — the second case is what the "as late as you can" rule is for, since neither of those PRs holds a duplicate to fail on. A trunk failure opens the alarm issue through `.github/workflows/trunk-red.yml`, so either way it surfaces the same day rather than months later in a reader's head. Renumbering afterwards means finding and repointing every citation, which is the expensive half.

## File structure

Single-context repo (most repos):

```
/
├── GLOSSARY.md
├── docs/adr/
│   ├── 0001-event-sourced-orders.md
│   └── 0002-postgres-for-write-model.md
└── src/
```

Multi-context repo (presence of `GLOSSARY-MAP.md` at the root):

```
/
├── GLOSSARY-MAP.md
├── docs/adr/                          ← system-wide decisions
└── src/
    ├── ordering/
    │   ├── GLOSSARY.md
    │   └── docs/adr/                  ← context-specific decisions
    └── billing/
        ├── GLOSSARY.md
        └── docs/adr/
```

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `GLOSSARY.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_
