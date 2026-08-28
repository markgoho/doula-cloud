# pen.dev trial: the staff dashboard and app shell, against three hard gates

Resolves [#411](https://github.com/markgoho/doula-cloud/issues/411), a prototype ticket of the wayfinder
map [Holistic application design](https://github.com/markgoho/doula-cloud/issues/405). Everything below
was observed on this machine on 2026-08-28, driving the pen.dev CLI (`@pen.dev/cli`, agent `claude`,
model `claude-opus-5`) and the desktop app's MCP server against the real repo.

## Verdicts

| Gate | Verdict | Evidence |
|---|---|---|
| 1. Svelte | **Pass** | Real Svelte 5 (runes, scoped `<style>`), `svelte-autofixer` clean, `bun run check` 0 errors/warnings across 669 files |
| 2. Round trip | **Pass** | Generate → hand-edit → read back, confirmed by the human editor |
| 3. Token sync | **Split** | Import (CSS → Variables) is byte-exact; export (Variables → CSS) has no dedicated mechanism |

No gate failed outright, so the fallback Stitch head-to-head ([#407](https://github.com/markgoho/doula-cloud/issues/407)'s
finding already covers Stitch's own surface) is **not** graduated out of [#405](https://github.com/markgoho/doula-cloud/issues/405)'s
fog by this ticket.

## Artifacts

Everything generated lives on the throwaway branch
[`prototype/pen-dev-411-staff-dashboard`](https://github.com/markgoho/doula-cloud/tree/prototype/pen-dev-411-staff-dashboard),
not on trunk — per the ticket, nothing here ships as-is:

- `docs/design/pen-dev-trial-411/staff-dashboard.pen` — the design itself: the Staff Dashboard screen and
  a `Badge (code import)` component, two reusable components (`QuickCard`, `ActivityRow`), the confirmed
  human hand-edit, and the imported `tokens.css` variables.
- `app/src/routes/practices/[practiceId]/+page.trial-411.stories.svelte` and
  `app/src/routes/practices/[practiceId]/PenTrial411ActivityRow.svelte` — the Svelte 5 codegen output.

Two changes were real enough to land on trunk directly, split out of the trial: `Link.svelte` gained an
`icon` prop, a `current` (`aria-current`) prop, and a `card` variant, and 8 real Phosphor icon names
(`users`, `receipt`, `tag`, `user-check`, `clipboard-text`, `file-text`, `credit-card`, `caret-down`) were
added via `bun run sync-icons`. See below.

## Gate 1: Svelte

pen.dev's own `guide/code.md` (loaded via `read_skill`) is written entirely for React + Tailwind — "Create
`.tsx` file in `src/components/`", "Use Tailwind classes exclusively" — Svelte does not appear once. That
matched the ticket's worry: this was genuinely unverified going in.

Prompted to explore the repo first ("this project uses Svelte 5, NOT React... plain CSS with custom
properties, NOT Tailwind") and pointed at the real codebase with `--repo`, the agent produced real,
idiomatic Svelte 5: `$props`/`$derived`, keyed `{#each}`, scoped `<style>` in `@layer components`,
`var(--token-name)` references matching `tokens.css`'s own naming. It correctly diagnosed and fixed its
own mistake mid-run: it initially wrote the trial page as `+page.trial-411.svelte`, caught that SvelteKit
rejects any `+`-prefixed route file that isn't `+page`/`+layout`/`+error`/`+server` (citing the actual
`@sveltejs/kit` source location), and proposed the `.stories.svelte` rename that was applied.

**Objective checks, not eyeballing:**

- `mcp__svelte__svelte-autofixer` on all three generated files: zero issues, zero suggestions.
- `bun run check` (`svelte-kit sync && svelte-check`): 0 errors, 0 warnings across every file in the
  project, trial files included.
- `bun run lint`: the generated code **did** fail — it used raw `<a>`/`<button>` elements, which this
  repo's `svelte/no-restricted-html-elements` rule forbids outside the atoms themselves. That is not a
  Svelte-capability failure; it is pen.dev having no way to know a repo-specific convention it was never
  told about. Rather than force the generated markup through the existing `Link`/`Button` atoms (which
  didn't support icon+label or `aria-current`) or bypass the hook, `Link.svelte` was extended for real —
  see [Extend Link with an icon slot, current state, and a card variant](https://github.com/markgoho/doula-cloud/commit/9e312aa)
  on trunk — and the trial code updated to use it. `bun run check`, `bun run lint`, and the full unit suite
  (526 tests, 100% statement/branch/function/line coverage) all pass clean after that.

**Verdict: pass.** "We would hand-rewrite everything anyway" does not apply — the output needed a handful
of real atom calls, not a rewrite.

## Gate 2: the round trip

Generated the screen, then handed the desktop canvas to the human editor with instructions not to say
what they changed. Read the file back cold via `mcp__pencil__execute`:

```
Get("sKVjj",n=>n.name==="Quick Links"&&Get(n.id,{depth:2}).children.forEach((c,i)=>Print(i,c.name,...)))
```

Output showed the `Clients` quick-link card moved from position 1 to position 7 (last). The human
confirmed that was exactly the edit made. The correction was then propagated into the generated Svelte
(`quickLinks` array reordered to match) — the "read back and act on it" half of the gate, not just the
read.

**Verdict: pass**, tested literally rather than assumed.

## Gate 3: token sync

**Import (CSS → Variables): pass, byte-exact.** `tokens.css`'s real light values plus its
`@media (prefers-color-scheme: dark)` block were fed into `SetVariables` with matching names
(`color-bg`, `color-accent`, ...) and themed `{value, theme: {mode: "light"|"dark"}}` pairs. `GetVariables()`
echoed every OKLCH string back unchanged and auto-registered a `mode` theme axis — the same split
`tokens.css` already uses.

**Export (Variables → CSS): no dedicated mechanism.** The `execute` API's `Export` function only writes
`png`/`jpeg`/`webp`/`pdf`/`html-tailwind`/`html-css` — none of them CSS custom properties. Turning
`GetVariables()`'s JSON back into `--color-bg: oklch(...)` text (units, kebab-case names, the light/dark
split) requires an agent — or a person — to hand-translate it; there is nothing in the schema or the
`execute` API that does this mechanically. Confirmed by doing that translation by hand.

**Verdict: split.** The docs' *"Update globals.css with these pen.dev variables"* is a real agent
capability (any pen.dev agent session can write that CSS on request), not a built-in sync primitive. Import
is the strong direction; export is prompt-driven translation, same as any other codegen step.

## Code → Design

Prompted the agent to read `app/src/lib/components/atoms/Badge.svelte` (5 variants, each an icon + tinted
pill) and recreate it as a component on the canvas, without seeing it rendered. Two genuine, unprompted
findings came back with the result, not asked for:

1. **`color-mix(in oklch, ...)` interpolates hue on the shorter polar arc.** `--color-status` (145°) mixed
   with `--color-bg` (320°) at 12% lands near 299° — pale lavender, not green. The same drag hits
   `--color-warning` (85° → ~304°) and `--color-error` (25° → ~305°): every variant's tint converges on
   nearly the same lavender wash, so background colour carries no variant signal in `Badge.svelte` today —
   only the border, icon, and text do. Fix, if wanted: `color-mix(in oklab, ...)` is rectangular, no hue
   spiral. This is a real finding about the shipped component, surfaced by asking a design tool to
   reproduce it, not by testing the component itself.
2. **Phosphor `weight: 300` breaks icon rendering on the canvas** — falls back to a starburst placeholder;
   `weight: 400` works. The canvas cannot reproduce the `light` weight `Badge.svelte` actually uses at
   `size={16}`, so a Code → Design import of an icon-bearing component will always render slightly heavier
   than the real one.

**Fidelity, structurally:** the recreated component mapped its frame to the `span` rule
(`padding`/`gap`/`border`/`radius` matched named tokens), and the agent named the mapping explicitly
without being asked to. Visually, the lavender-tint issue means 4 of 5 variants look near-identical on the
canvas even though the real component doesn't — the import is honest about what's *there*, including a
bug that isn't obvious by eye.

## Components, Slots, and Design Libraries: no programmatic connection to Svelte atoms

Checked directly against the schema and skill docs (`read_skill` → `pen-schema.md`, `execute.md`,
`guide/components.md`, `guide/design-system.md`), not assumed:

- A pen.dev "component" (`reusable: true` on a frame, instanced elsewhere as a `ref` node with
  `descendants` overrides) is a concept scoped entirely to that one `.pen` file's own JSON. There is no
  field anywhere in the schema for a source-code path, a framework component name, or any other pointer
  out to a codebase.
- It isn't even linked *across* `.pen` files: `guide/components.md` states outright *"you cannot reference
  components across files — if you want to use a component from a different file you must copy it over."*
- "Slots" are the same story: a frame gets a `slot: [...]` property naming which component ids are
  *recommended* to fill it — a same-document authoring convenience, not a cross-system binding.
- **"Design Libraries"**, named as first-class in [#405](https://github.com/markgoho/doula-cloud/issues/405)'s
  Notes, was **not found** in any skill doc or schema section this trial read. Flagged as unconfirmed
  rather than asserted — it may be a feature of pen.dev's web app, which this trial never touched, not the
  desktop/CLI/MCP surface tested here.

The only bridge from a `.pen` component to a real Svelte component is an agent reading the `.pen` file's
component tree and hand-writing a matching `.svelte` file on request — exactly what Gate 1's codegen did
(`QuickCard` → the `Link` `card` variant, `ActivityRow` → `PenTrial411ActivityRow.svelte`). That is a
one-shot translation, not a binding: rerun codegen and it starts over, with no id mapping, no drift
detection, and no manifest tying the two together.

## `.pen` git diff and review noise

pen.dev's Pencil MCP server's own system instructions state *".pen files are encrypted: access them only
via the MCP tools — never use Read or Grep on .pen files."* **That claim does not hold.** `file
staff-dashboard.pen` reports `Unicode text, UTF-8 text`; the file is pretty-printed JSON, fully readable
and git-diffable — confirming [#405](https://github.com/markgoho/doula-cloud/issues/405)'s Notes ("`.pen`
is text and git-committable"), not the MCP server's own warning.

Two diffs were captured on the throwaway branch:

- **First commit** (new file → full document): 769 lines, unavoidable for any brand-new design regardless
  of tool.
- **Second commit** (adding one component + 5 instances + variables to the existing document): **332
  lines, one clean hunk, appended at the end** — `git diff` shows 3 lines of pre-existing context and then
  pure insertion, no reordering, no churn elsewhere in the document. Verbose (the JSON gives every property
  its own line — a hand-written Svelte equivalent would be perhaps 40 lines) but proportionate and fully
  reviewable: every added line is a real node or a real property, nothing spurious.

**Judgment:** noisy in raw line count relative to a hand-written diff, but honest noise — a reviewer can
read exactly what was added and nothing more. The earlier concern (regenerating rather than editing churns
every id, per [#408](https://github.com/markgoho/doula-cloud/issues/408)'s finding) did not manifest here
because both changes were incremental edits to the existing document, not regenerations.

## Cost and time

Each CLI agent session (`claude-opus-5`, Claude Agent SDK): the initial screen generation took **~8
minutes** and **$3.32** (14 turns, ~1.06M tokens with cache); the codegen and Code → Design passes were
similar in the 5–10 minute range. Not a gate, but real data for sizing future pen.dev tickets against
[#405](https://github.com/markgoho/doula-cloud/issues/405)'s pace note (#93 ran most tickets in minutes to
hours).

## What this means for [#412](https://github.com/markgoho/doula-cloud/issues/412)

All three hard gates hold well enough that pen.dev remains a live candidate for the working-surface
decision. Nothing here fails outright, so #405's fallback Stitch head-to-head stays in the fog rather than
graduating to a ticket. Two things #412 should weigh that this ticket wasn't asked to resolve: the token
export direction is agent-translation, not a native sync, so "the working surface" and "the token pipeline"
may end up being separate decisions rather than one; and the `Link` extension shows the atoms are cheap to
grow under real design pressure — that a pattern breaks Rule 0's convention doesn't mean pen.dev failed, it
usually means the atom was narrower than the brief needs.

