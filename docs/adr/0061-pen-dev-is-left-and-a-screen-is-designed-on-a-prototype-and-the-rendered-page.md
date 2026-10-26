# pen.dev is left, and a screen is designed on a prototype and the rendered page

[ADR-0019](0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md) made pen.dev the working surface and the code the truth, and named four triggers that would cause the choice to be reconsidered. On 2026-10-25 the founder decided to leave pen.dev and delete `docs/design/doula-cloud.pen`, its generated export and the script behind it ([#1993](https://github.com/markgoho/doula-cloud/issues/1993)). Nothing replaces the canvas. A screen is designed where it was already being designed: on a throwaway prototype published as an artifact when a visual decision has to be made, and on the rendered page for review. `app/src/lib/styles/tokens.css` and `app/src/lib/components/**` remain the only source.

## Why now

- **The canvas's own exit test had passed.** [The design workflow](../design/workflow.md) said every new route goes through the canvas until the Templates ship, the shell ships, and three consecutive routes are built with no canvas step. The Templates shipped on [#422](https://github.com/markgoho/doula-cloud/issues/422), the shell on [#1113](https://github.com/markgoho/doula-cloud/issues/1113), and about forty routes landed between 2026-09-01 and 2026-10-24 against four commits to the `.pen` file, none of which designed a screen.
- **The only remaining canvas work was drift repair.** [#1765](https://github.com/markgoho/doula-cloud/issues/1765) and [#1941](https://github.com/markgoho/doula-cloud/issues/1941) existed to copy decisions the code had already made back onto a drawing nobody built from. ADR-0019 accepted that cost as the price of a working surface; with no screen designed on the canvas since September, it was the whole cost.
- **A named trigger fired.** pen.dev sells paid tiers since this ADR was written (Pro $16 and Ultra $48 per user per month on 2026-10-25), which ADR-0019 said makes staying "a fresh decision rather than a renewal". The generative half of the tool, the `@pen.dev/cli`, had already been removed on 2026-09-25 ([#1469](https://github.com/markgoho/doula-cloud/issues/1469)).
- **The visual system is about to move.** The founder expects to revisit the look during the November and December 2026 pilot, through tokens. A canvas during that period doubles every change.

## What designs a screen now

- **A route that picks a Template goes straight to code.** [ADR-0018](0018-templates-are-a-design-system-layer-with-two-named-exits.md) and [ADR-0058](0058-atomic-design-names-the-tiers-and-two-consumers-that-should-change-together-earn-a-variant.md) hold the tiers and the bar.
- **A visual decision is made on a prototype the founder can open anywhere.** The `/prototype` skill and [`docs/agents/prototype.md`](../agents/prototype.md) say how one is built and published as an artifact. The type scale ([ADR-0060](0060-the-visual-craft-books-stay-evidence-and-the-type-scale-is-one-ratio-from-16px.md), [#1926](https://github.com/markgoho/doula-cloud/issues/1926)) was decided this way, on real screens, not on the canvas.
- **Review happens on the rendered page**, at a desktop and a narrow width, on the real route. [ADR-0025](0025-layout-is-verified-across-the-continuum.md) is the check.

## What is lost

Code → Design as a review technique. Importing `Badge.svelte` onto the canvas found the `color-mix(in oklch, ...)` hue defect in August ([#411](https://github.com/markgoho/doula-cloud/issues/411)). It was used once. The research that recorded it stays under `docs/research/`.

## Considered and rejected

- **Carrying the canvas into Claude Design.** The Design artifact type keeps its artboards as plain HTML files that can be committed, reads a hand edit back, and needs no desktop app, so it covers what pen.dev was used for. It was rejected for the same reason the canvas is left: an artboard is a second drawing of a screen the Svelte already renders, and it goes stale unless a ticket keeps it in step. Translating a `.pen` file that was already behind the code would have copied the staleness into a new tool. The founder had also ruled out Claude Design for this purpose on 2026-08-28 ([#409](https://github.com/markgoho/doula-cloud/issues/409)).
- **A Design System artifact synced read-only from the code.** It fits "code is the truth" and would give a contributor without the repo a browsable reference to `tokens.css` and the components. Not built now: the style guide routes already show every component on a real page, and there is no second contributor to read it. Revisit when there is.
- **Keeping the `.pen` file as history.** Rejected. A committed design file reads as a source, and two tickets had already been written to keep it current. The research write-ups and this ADR are the history.
