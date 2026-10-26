# The design workflow

How a screen goes from a ticket to Svelte on trunk, and how a visual decision gets made along the way.

The decisions behind this are [ADR-0019](../adr/0019-pen-dev-is-the-working-surface-and-code-is-the-truth.md), which made the code the truth, and [ADR-0061](../adr/0061-pen-dev-is-left-and-a-screen-is-designed-on-a-prototype-and-the-rendered-page.md), which left the pen.dev canvas. This document is the procedure, and it is expected to change as we learn. The ADRs are not.

Everything obeys [the design brief](brief.md). A screen that wants to depart from the brief changes the brief first.

## The pieces

| | What it is | Where it lives |
|---|---|---|
| `app/src/lib/styles/tokens.css` | Every color, type, space and radius value the app uses. The machine-readable truth. | Trunk. |
| `app/src/lib/components/**` | Atoms, molecules, organisms and Templates. The definition of every piece of UI. | Trunk. |
| The style guide, `app/src/routes/style-guide/` | One page per component, mounted with hostile content. What the continuum check sweeps. | Trunk, DEV only. |
| A prototype | A throwaway route on a `prototype/<issue>-<slug>` branch, built as one HTML file and published as a claude.ai artifact. | The branch, never merged. See [`docs/agents/prototype.md`](../agents/prototype.md). |

There is no design file. A drawing of a screen that the Svelte already renders is a second description of it, and the two drift. When a question is about how something looks, the answer is a rendered page, not a picture of one.

## Designing a screen

1. **Pick the Template.** A screen that instantiates `OverviewHub`, `RecordDetail`, `FormPage`, `QuestionPage`, `CheckAnswers`, `ListPage`, `DocumentPage`, `EntryPage`, `LandingPage` or `ErrorPage` is arranging regions, not inventing a page. [ADR-0018](../adr/0018-templates-are-a-design-system-layer-with-two-named-exits.md) holds the tiers and the bar for a new component or variant; [`govuk-alignment.md`](govuk-alignment.md) holds the pattern for a screen that asks for something or reports a failure. Most screens stop here and go to code.
2. **When the screen is a new archetype, or a visual choice has to be made, build a prototype.** The `/prototype` skill decides what to build. [`docs/agents/prototype.md`](../agents/prototype.md) says where it lives, how the single-file hash build makes it an artifact, and which gates it trips. Publish the artifact before asking the first question; a choice made without seeing the prototype is not a review. Show a visual choice in context, on a real screen with real content, never as a swatch or a paragraph.
3. **Record the choice** where it belongs: a token in `tokens.css`, a component or variant, a line in the brief, or an ADR when the choice is hard to reverse and would surprise a later reader. The prototype branch stays as the record of what was compared.

## Carrying a design into code

1. **Write Svelte against the repo's own rules.** Atoms, molecules, organisms and Templates come first. Raw `<a>` and `<button>` are forbidden outside the atoms by `svelte/no-restricted-html-elements`.
2. **When the design needs something an atom cannot do, grow the atom.** Do not bypass the rule and do not hand-roll the markup. On [#411](https://github.com/markgoho/doula-cloud/issues/411) this produced a real improvement: `Link.svelte` gained an icon slot, a `current` state and a `card` variant, and landed on trunk on its own merit.
3. **Tokens come from `tokens.css`, never from a component.** A component names a semantic token; it never holds a value of its own. A new value is a new token first, and the brief says why it exists.
4. **Verify**: `bun run check`, `bun run lint`, `bun run test` in `app/`, with the coverage gate intact.
5. **Then open it in a browser**, at a desktop and a narrow width, on the real route and not only the style guide. Every ticket that skipped this missed something the whole suite passed over: `LabeledField`'s stacked orientation did not stack ([#425](https://github.com/markgoho/doula-cloud/issues/425)), and `GET /clients` did not mean what the empty state assumed ([#423](https://github.com/markgoho/doula-cloud/issues/423)). `bun run dev:full` brings up the stack; seed a fixture the way `e2e/stack.ts` does and log in as a real person. [`docs/testing/walk-a-screen-locally.md`](../testing/walk-a-screen-locally.md) has the steps.

   **This machine's Chrome inverts page colors** with an extension, so a screenshot taken through it is evidence about layout and never about the palette. For color, use a clean browser or read the computed value rather than the picture.

## Committing

The Svelte change and its style-guide page go in the same PR. A prototype branch never merges; what it decided lands as a token, a component or a brief line, and the PR body names the prototype's issue and artifact.
