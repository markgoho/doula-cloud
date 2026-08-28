# pen.dev is the working surface, and the code is the truth

Screens in this application are designed on a canvas before they become Svelte. This document records
which canvas, where the design lives, and — the part that actually matters — which side wins when the
canvas and the code disagree.

Decided on the wayfinder map
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405), ticket
[#412](https://github.com/markgoho/doula-cloud/issues/412). The step-by-step procedure lives in
[the design workflow](../design/workflow.md), deliberately not here: this decision should be stable,
and that procedure will be edited as we learn more.

## pen.dev is the working surface

Three tools were tried on real work. Only one survived a round trip.

**Stitch** ([#407](https://github.com/markgoho/doula-cloud/issues/407)) generates well and cannot read
back. Its MCP server registers **zero tools** in Claude Code, because `upload_design_md` carries a
dangling `#/$defs/ScreenInstance` reference that aborts the whole tool list. Driven over raw JSON-RPC it
generated a finished screen in 91 seconds and returned correct surgical `dom_operations` from
`edit_screens` — which then never reached any readable artefact. `get_screen` returned byte-identical
HTML across four polls and the canvas did not change. Stitch cannot read back its own edits, let alone
a person's.

**Claude Design** generated one aesthetic direction and the four-way comparison for
[#409](https://github.com/markgoho/doula-cloud/issues/409), and was ruled out by the account owner for
everything after that.

**pen.dev** ([#411](https://github.com/markgoho/doula-cloud/issues/411)) passed three hard gates on a
real screen. It emits idiomatic Svelte 5 — `svelte-autofixer` clean, `bun run check` at zero errors
across 669 files. A blind hand-edit on the canvas was read back correctly and carried into the generated
code. `tokens.css` imported into canvas Variables byte-exact, light and dark split intact.

**Stitch is dropped.** If a generate-a-look-from-nothing job ever returns, re-check `claude mcp list`
first — that defect is Google's to fix and it will land upstream, not here.

## Code is the truth

A pen.dev component has **no field anywhere in its schema** for a source path, a framework component
name, or any pointer to a codebase, and components cannot even be referenced across `.pen` files. There
is no binding available to build. The only bridge is an agent reading the canvas and writing Svelte, or
reading Svelte and drawing on the canvas — a one-shot translation with no id mapping and no drift
detection.

So one side has to be authoritative, and it is the code. `app/src/lib/components/**` and
`app/src/lib/styles/tokens.css` are canonical. A node on the canvas is a sketch of something, never the
definition of it. When the two disagree the code wins, and the canvas is re-imported from it.

This is not a preference about tooling; it is what makes the tool disposable. Because nothing in the
application depends on the `.pen` file, leaving pen.dev costs a deletion rather than a migration.

The direction that works is worth naming: importing `Badge.svelte` onto the canvas surfaced two real,
unprompted defects in the shipped component — `color-mix(in oklch, ...)` interpolating hue along the
short polar arc, which collapses four of five badge tints to nearly the same lavender, and Phosphor
`weight: 300` failing to render. Code → Design is a review technique, not just an import.

## One committed file

The design lives at `docs/design/doula-cloud.pen`, committed to trunk. **One file for the whole
application**, not one per screen or per surface.

`.pen` is plain UTF-8 JSON — pretty-printed, diffable, and reviewable, contrary to the Pencil MCP
server's own instruction that the files are encrypted. A new document diffs as a few hundred lines; an
incremental edit diffed as one clean appended hunk with no churn elsewhere.

Three things force the single file:

- **Components cannot cross files.** pen.dev's own guidance: *"you cannot reference components across
  files — if you want to use a component from a different file you must copy it over."* Split the
  document and every shared component becomes a copy that drifts.
- **`execute`'s `filePath` argument is ignored.** Verified: a call naming one existing `.pen` file
  returned the nodes of a completely different document — the one the desktop app had open. The MCP tool
  always targets the active canvas editor. With several files, an agent that believes it is editing one
  document is silently editing another. With one file there is nothing to get wrong.
- **Reuse gets cheaper inside one document.** `Copy` on a reusable frame creates a connected instance.
  The tenth screen is built by copying the fourth, which is minutes rather than the eight it took to
  generate the first from nothing — and only within a single file.

Committed rather than ignored, because an uncommitted design is one nobody else can read back, and
because the design and the code that implements it belong in the same commit. The trade-off accepted
here is verbosity: the JSON gives every property its own line, so a change that would be forty lines of
Svelte is a few hundred lines of `.pen`. Honest noise — every added line is a real node or a real
property — but noise.

## What would make us leave

Four triggers. Each is cause to **reconsider**, not an automatic exit:

1. **pen.dev stops being free.** No Anthropic or agent API key is configured on this machine and the CLI
   authenticates by stored pen.dev session, so pen.dev currently pays for the model tokens of every
   design session. A paid tier ends that, and it is a fresh decision rather than a renewal.
2. **The surface churns again.** The MCP surface moved mid-ticket during
   [#408](https://github.com/markgoho/doula-cloud/issues/408) (1.2.4 → 1.2.7) and the export tool
   disappeared entirely. A second breaking change to a capability a ticket depends on is a trigger.
3. **The round trip breaks.** A hand-edit that no longer reads back removes the one thing that separated
   pen.dev from Stitch.
4. **The desktop dependency blocks someone.** The canvas is a macOS Electron app. It is a one-machine,
   one-operating-system dependency, and a second contributor may not have it.

Cost of acting on any of them: delete `docs/design/doula-cloud.pen`. No component, no token, and no
route depends on it. That is the whole point of the previous section.

## Considered and rejected

- **One `.pen` per screen or per surface.** Rejected on the `filePath` and cross-file-component findings
  above. Smaller diffs are not worth an agent editing the wrong document.
- **Treating the canvas as the source of truth**, with code drift as a defect. Rejected: no mechanism
  exists to detect that drift, so the rule would be unenforceable, and it would trap value in a tool
  chosen four months before launch.
- **Keeping Stitch for a named job.** Rejected. The brief is settled and binding, so the one job Stitch
  demonstrated is finished. A tool kept "just in case" becomes an obligation nobody revisits.
- **Generating `DESIGN.md` as an interchange format.** Already rejected on the map: it is one vendor's
  format, covers colour and type only, and pen.dev's documentation never mentions it. `tokens.css` is
  the machine-readable truth and [the brief](../design/brief.md) is the prose one.
