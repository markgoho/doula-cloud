# Candidate aesthetic directions — issue #409

Raw generator output kept as evidence for the design-brief decision on
[#409](https://github.com/markgoho/doula-cloud/issues/409), a sub-ticket of the
[Holistic application design](https://github.com/markgoho/doula-cloud/issues/405) map.

Three directions were generated with **Google Stitch**, driven headlessly over raw JSON-RPC
(the MCP server still registers zero tools in Claude Code — see
[`../../research/stitch-mcp-agent-surface.md`](../research/stitch-mcp-agent-surface.md)). A fourth,
**Ledger & Ink**, was drawn in Claude Design and has no Stitch artefact.

Every direction was given the *same* screen brief — the staff Overview Hub, desktop, light theme,
top bar with flat nav and a Practice switcher, four groups of destination links, and a Recent
activity feed — and differs only in the aesthetic paragraph appended to it.

| File | Direction | Stitch project |
|---|---|---|
| `nurture-and-node.*` | Nurture & Node — terracotta on warm cream, Playfair Display + Inter | `17652880749527313283` |
| `plum-dusk-evolved.*` | Plum Dusk, evolved — the incumbent plum, disciplined; Hanken Grotesk | `2499324920818221214` |
| `quiet-operator.*` | Quiet Operator — deep pine on cool near-white, Inter at 14px | `5071869623364409198` |

`*.png` is Stitch's own render at 2560×2048. `*.DESIGN.md` is the design system Stitch emitted with
it: YAML front matter that maps name-for-name onto a CSS custom-property block, plus a named type
scale and a prose style guide. The generated HTML is deliberately **not** kept — it is Tailwind
Play-CDN markup with all styling in `class=` attributes, which is the worst possible base for
Svelte's scoped `<style>`.

The four directions were compared side by side, redrawn in one medium from these tokens, on a
Claude Design canvas linked from #409.
