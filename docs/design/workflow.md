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

## Five `execute` rules that are not in Pen's own skill

Each of these cost a rebuild on [#431](https://github.com/markgoho/doula-cloud/issues/431). They are
about the tool, not about design, and none of them is discoverable from the API documentation.

**Build a subtree in one declarative call, not by growing it.** `Insert` a node, then `Insert` its
children into the returned id, and the layout comes out wrong — children land outside their parent's
box and the frame renders blank or clipped, while `Get` reports bounds that do not match what the
canvas draws. The reliable shape is `id = Insert(parent, {…empty frame…})` followed by
`Replace(id, {…the whole tree, `children` nested…})`. Passing nested `children` to `Insert` directly
fails the same way that growing it does; only `Replace` settles the layout.

**`Replace` throws inside a reusable component.** Any descendant of a node marked `reusable: true`
rejects `Replace` with `TypeError: Cannot read properties of undefined (reading 'type')`, whatever the
replacement is — a bare text node fails as readily as a subtree. Inside a component, `Insert`,
`Update`, `Delete` and `Move` all work. When a component needs a nested structure changed, build that
structure as **its own root-level component** and `Insert` a single `ref` to it; a `ref` nests inside
a component without complaint. That is why `BrandLockup` exists as a component rather than as two
nodes inside `StaffTopBar`.

**`strokeWidth` on a `path` is node pixels, not `viewBox` units.** The `viewBox` scales the geometry
onto the node's box; the stroke is not scaled with it. A mark authored at `stroke-width: 14` in a
202-unit-wide `viewBox` needs roughly `height / 6.5` once it is drawn at 40x19, or it renders as a
blob. Re-derive the stroke for every size the mark is used at, rather than copying the number out of
the source SVG.

**There is no variant system, and theme axes are the substitute.** Pen has no Figma-style variants:
a component is one tree, and an instance customises it through `ref` properties and a `descendants`
map. What fills the gap is that **variables take a value per theme-axis value, and any node can be
pinned to one** through its `theme` property. `SetVariables` registers a new axis on the fly, so
`mark-stroke` carries `9`/`4`/`3` on a `size` axis of `lg`/`md`/`sm`, the `CloudMark` arcs reference
`$mark-stroke`, and an instance picks its weight with `theme: {size: "sm"}`. One component, three
sizes, no duplication — and it composes with the `mode` light/dark axis already in the document
rather than competing with it. Reach for this whenever instances differ by a *value* rather than by
structure; `descendants` is still the answer when they differ by content.

**Pin the component master too.** An unpinned node resolves the variable on whatever the axis
falls back to, so a master drawn at the small size but left unpinned renders with the large
value and looks broken the moment somebody opens the component — which is exactly how this was
found. Give the master an explicit `theme` matching the size it is drawn at, and never rely on
which axis value happens to come first. Note also that an instance scales its subtree from the
`ref`'s own `width`/`height`: if the component's *children* carry explicit sizes matching the
master, the override stops scaling them and every instance renders at one size.

**`phosphor` is a valid icon library on the canvas.** The schema's `Icon.library` accepts `lucide`,
`feather`, three Material Symbols variants and `phosphor`, so a drawing is not forced onto Lucide
stand-ins. #411's `weight: 300` rendering bug still stands, so a drawn icon is not evidence about the
shipped one either way; the code follows [#96](https://github.com/markgoho/doula-cloud/issues/96)
regardless of what the canvas shows.

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

## Autosave, and the one operation it misses

**The desktop app does autosave, and the trigger is a scenegraph change** — settled on
[#417](https://github.com/markgoho/doula-cloud/issues/417), against a healthy editor opened on the real
`docs/design/doula-cloud.pen`, which is the condition the earlier test could not rule out. The app's own
title bar states which it is: a document reads `— Auto-saved` once written, and `— Edited` while dirty.

**What is not established is the trigger, and it is not worth guessing at.** On #417: `SetVariables`
wrote 76 tokens and registered the `mode` axis, and the file stayed untouched on disk for four minutes
while `GetVariables()` read every one of them back. A save did land later, around a throwaway frame
insert. But a subsequent `Delete`, and then a net-zero insert-and-delete in one call, each failed to
flush within 90 seconds. One observation is not a mechanism, so treat the trigger as **unpredictable
from an agent's side**.

Two consequences, and they are the durable part:

> **No read can tell you whether your work is on disk.** `GetVariables()` and `Get()` both return the
> in-memory document and will happily confirm work that was never written. **`git status` on the `.pen`
> file is the only check that distinguishes saved from unsaved**, and it belongs at the end of every
> canvas pass.

> **⌘S in Pen is the only deterministic write.** There is no save in the `execute` API — `Export()`
> writes PNG, JPEG, WEBP, PDF and HTML, never `.pen`. An agent that has finished a canvas change should
> verify with `git status` and, if the file is unchanged, ask for the keystroke rather than waiting on
> an autosave that may not come.

A related trap: because the disk file can lag the live document by an arbitrary amount, whatever autosave
does eventually write is a **snapshot of some intermediate state**, not necessarily the state you left.
On #417 the flush captured a throwaway probe frame that had already been deleted in memory. Never commit
a `.pen` without confirming the diff is what you meant.

The CLI (`bunx pen --in … --out …`) is unaffected: it writes the file paths it is given, which is why
[ADR-0019](../adr/0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md) calls it the
file-writing path.
