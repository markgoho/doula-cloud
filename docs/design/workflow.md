# The design workflow

How a screen goes from a ticket to Svelte on trunk, and how a hand-correction on the canvas gets back
into the code.

The decision behind this — pen.dev as the working surface, code as the truth, one committed `.pen` file
— is [ADR-0019](../adr/0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md). This document is
the procedure, and it is expected to change as we learn. The ADR is not.

Everything obeys [the design brief](brief.md). A screen that wants to depart from the brief changes the
brief first.

## The pieces

| | What it is | Where it writes |
|---|---|---|
| `docs/design/doula-cloud.pen` | The whole application's design. One file. | Trunk. |
| The desktop canvas | `/Applications/Pen.app`. Where a person edits by hand. | The app's memory, then disk on save. |
| `mcp__pencil__execute` | A JavaScript API — `Insert`, `Update`, `Copy`, `Get`, `SetVariables` — that an agent calls from this session. Deterministic; it is not a generative model choosing a layout. | The **active canvas editor**. Its `filePath` argument is ignored. |
| `bunx pen --in … --out …` | The CLI. Spawns a separate Claude Agent SDK session that makes those same `execute` calls. | The file paths given. |

Two consequences worth holding on to:

- **`execute` edits whatever the app has open.** Before an agent touches the canvas, the app must have
  `doula-cloud.pen` as its active editor. Confirm with `get_app_state`, which prints the active path.
- **The CLI is the file-writing path.** `execute` changes are in memory until the canvas is saved.

## Designing a screen

1. **Open the file.** `doula-cloud.pen` in the desktop app. The agent confirms with `get_app_state` that
   the active editor is that path and no other.
2. **Look for an existing screen to copy.** This is the first move, not a fallback. `Copy` on a reusable
   frame creates a connected instance, so a screen that reuses `QuickCard` or an activity row inherits
   later edits to it. Generating from nothing is for a genuinely new archetype.
3. **Draw it**, against the brief and against the Templates in `app/src/lib/components/templates/`. A
   screen that instantiates an existing Template is arranging regions, not inventing a page.
4. **Check it before showing it.** `Get` with a visitor reports `ctx.bounds` and `ctx.problems`, which
   catches clipping and collapsed layout without a screenshot. Screenshot only to judge colour, type and
   alignment.
5. **Save**, so the `.pen` change is on disk and in `git diff`.

## Hand-correcting on the canvas

Open the canvas and change what looks wrong. That is the point of the tool.

- **Save when done.** Until the file is saved, the change exists only in the app.
- **Either say so, or expect it to be found.** Every design ticket starts by running `git diff` on
  `doula-cloud.pen`, so an unmentioned edit is picked up at the start of the next piece of work. Telling
  the agent directly gets it handled now instead.
- **You do not have to say what changed.** Reading the correction back cold is a tested capability, not
  an assumption: on [#411](https://github.com/markgoho/doula-cloud/issues/411) a blind reorder of the
  quick-link cards was read back correctly and carried into the code.

## Carrying a design into code

1. **Read the canvas**, not a screenshot of it. `Get` returns schema data with resolved bounds.
2. **Write Svelte against the repo's own rules.** Atoms, molecules, organisms and Templates come first.
   Raw `<a>` and `<button>` are forbidden outside the atoms by `svelte/no-restricted-html-elements`.
3. **When the design needs something an atom cannot do, grow the atom.** Do not bypass the rule and do
   not hand-roll the markup. On #411 this produced a real improvement: `Link.svelte` gained an icon slot,
   a `current` state and a `card` variant, and landed on trunk on its own merit.
4. **Tokens come from `tokens.css`, never from the canvas.** Import runs one way, CSS → canvas Variables,
   and it is byte-exact. There is no export mechanism: `Export()` writes images, PDF and HTML, never CSS
   custom properties. Turning canvas Variables back into CSS is an agent translating by hand, which is
   how a token drifts. Change `tokens.css`, then re-import.
5. **Verify**: `bun run check`, `bun run lint`, `bun run test` in `app/`, with the coverage gate intact.

## Committing

The `.pen` change and the Svelte change **go in the same commit**. A design and the code implementing it
should never disagree in history, and one commit makes the pair reviewable together.

## When the canvas can be skipped

Every new route goes through the canvas for now.

That rule expires when all of these are true: the three Templates
([#422](https://github.com/markgoho/doula-cloud/issues/422)) have shipped, the application shell has
shipped, and three consecutive routes have been built by instantiating a Template with no canvas step.
After that, the canvas is for genuinely novel surfaces — a new archetype, a new embedding context — and
a route that picks an existing Template goes straight to code.

The test exists because "once the design system is mature" is not a thing anyone ever declares.

## Known unknown

**Whether the desktop app autosaves.** Not settled. A canvas edit made through `execute` did not reach
disk in testing, but the editor in that test had been opened against a file that no longer existed, so
the result does not distinguish "no autosave" from "editor detached". Settle it on the first real screen
and correct this document.
