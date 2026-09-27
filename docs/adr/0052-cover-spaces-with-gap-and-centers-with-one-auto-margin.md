# Cover spaces its children with `gap`, and centers with one auto margin

`cover-l` spaces its children with `gap` on the container, like `stack-l` ([ADR-0039](0039-a-layout-primitive-spaces-its-children-with-gap.md)). The centered child keeps `margin-block: auto`. Cover no longer writes `> * { margin-block: <space> }` or the two end-caps that went with it. The step between two adjacent non-centered children is now one `space`, not two. Decided on [#1220](https://github.com/markgoho/doula-cloud/issues/1220), 2026-09-27. This amends ADR-0039's Cover section, which kept Cover margined.

**Amended below, 2026-09-27 ([#1482](https://github.com/markgoho/doula-cloud/issues/1482)):** the centered child no longer carries the auto margin. Two growing spacers on the cover do the centering. The spacing decision above stands.

## Why the Stack answer does transfer

ADR-0039 said a `gap` cannot express "`space` at the ends and `auto` in the middle", so Cover had to keep its margins. That put two different jobs into one mechanism:

- **Spacing** is the distance between two children. A `gap` does this, and a child cannot cancel it.
- **Centering** is where the free space goes. The centered child's auto margins do this. An auto margin is alignment, not spacing, and `primitives.usage.spec.ts` already allows it for that reason.

When the two jobs are separate, both fit together. In a flex column, the gap sits between every pair of items first. Then the auto margins on the centered child take all the free space and share it equally above and below it. So the centered child sits midway between its neighbors, with one gap on each side of it. The ends need no rule: the first and last children sit against the padding, because a gap writes nothing at the edges. The old end-caps did only that.

## Why one `space`, not two

The old margins gave two adjacent non-centered children 2×`space`, because flex margins do not collapse. ADR-0039 called that deliberate. Nothing supports that: it is how Every Layout's source behaves, and no screen depends on it. `cover-l` has no callers in `app/src` or `site/src`. With `gap`, `space` means the same thing on Cover as on Stack, Cluster, Grid, Sidebar, Switcher and Reel: the distance between two children. The default stays `var(--space-4)`. A caller that wants a wider step sets `space`.

## The candidates, and why the others lose

**An inner wrapper.** Cover would own the centering and the padding, and a nested `stack-l` would own the spacing. Rejected. It changes what a caller writes, and it cannot center one child of the wrapper, so the centered child has to sit outside it. It adds markup to solve a problem that one `gap` solves.

**Keep the margins and forbid `margin: 0` on a Cover child.** Rejected, for the reason ADR-0039 rejected it for Stack: "root element" is a fact about markup, not about CSS, so no gate can check it.

**Keep the margins and double the default.** Not needed. The step halves only if the goal is to keep 2×`space`, and nothing asks for that.

## What stays exposed

The centered child's `margin-block: auto` is still a child margin in `@layer utilities`. A centered child that resets `margin: 0` on its own root element in `@layer components` wins the cascade and is no longer centered. `Heading` does exactly that. The gate allows `auto`, so it will not catch this. [#1482](https://github.com/markgoho/doula-cloud/issues/1482) is where that gets its own answer. Every candidate there changes how a centered child with a visible box renders, so it is a separate decision, not part of this one. The answer is the amendment at the end of this record.

## How this is checked

`app/src/lib/primitives/cover.spec.ts` renders Cover in a real browser at 1440, 480 and 320 and measures geometry, as `stack.spec.ts` does. It asserts four things. Two adjacent non-centered children are one `space` apart. A non-first child that resets `margin: 0` in `@layer components` gets the same step as one that does not. The centered child sits midway between its neighbors, more than one `space` from each. The first and last children sit against the padding. `space` is the cover's own computed padding, because every `--space-N` is a `clamp()` with a `cqi` and has no fixed pixel value.

`app/src/lib/styles/primitives.usage.spec.ts` no longer exempts anything. The three `primitives:ignore` markers on Cover's rules and the `EXEMPT` map that excused `cover-l`'s generated rules are gone, and so is the test that checked the exemption was still used.

## Amendment, 2026-09-27: two growing spacers center the child, not its auto margin ([#1482](https://github.com/markgoho/doula-cloud/issues/1482))

The centered child's `margin-block: auto` goes. `cover-l::before` and `cover-l::after` become flex items with `content: ''` and `flex-grow: 1`, and `order` puts them on each side of the centered child. The spacers share the free space equally, so the centered child sits midway between its neighbors. Decided on #1482, 2026-09-27.

### The defect

The auto margin is a child declaration in `@layer utilities`. A component's scoped `<style>` compiles into `@layer components`, which comes later, so a centered child whose root element sets `margin: 0` wins the cascade and loses its centering. `Heading` does that (`Heading.svelte`'s `.variant-page`, `.variant-section`, `.variant-card`). Triage rendered it in `cover.spec.ts` at 1440, 480 and 320: with the centered `h1` reset in `@layer components`, it sat exactly one `space` below its predecessor and one `space` above its successor, and all the free space went to the bottom of the cover. This is the same cancellation ADR-0039 fixed for Stack, on the one child declaration Cover still had.

### What the mechanism is

In source order, the spacers are the cover's first and last flex items. `order` places them:

- Every child that comes before the centered child: `order: -1`.
- `::before`: `order: 0`.
- The centered child: `order: 1`.
- `::after`: `order: 2`.
- Every child that comes after the centered child: `order: 3`.

`order` changes only the visual order. The DOM order, the reading order and the focus order stay the same, because only the two pseudo-elements move relative to the children, and a pseudo-element is not in the accessibility tree or the focus order.

The spacers exist only when the cover has a centered child. A cover without one keeps its children at the top with no extra gaps at the end.

### Why the spacers, and not the other two candidates

**`flex-grow: 1` on the centered child with block-level `align-content: center`.** Rejected. The centered child's box grows to fill all the free space, so a centered child with a border or background becomes a tall band and not a centered box. The founder chose on #1482 that a centered child keeps its own size. Block-level `align-content` is also new: Baseline newly available since March–April 2024, and widely available only from about October 2026.

**Keep the auto margin and gate a `margin` reset on a centered child.** Rejected, for the reason ADR-0039 and this record already give: "root element" is a fact about markup, not CSS, so no gate can check it.

### What the spacers cost

- **Two gaps on each side of the centered child, not one.** A spacer is a flex item, so the `gap` goes on both sides of it. When the cover has free space, the spacers absorb it and this does not show. When the cover has no free space, the centered child is 2×`space` from each neighbor, not 1×. The distance is the same on both sides, so the child is still centered.
- **`order` is still a child declaration in `@layer utilities`.** A component could override it. This change does not remove every child declaration. It moves the centering off the declaration that components reset. `margin: 0` is in 33 files under `app/src`. No component sets `order`.
- **The centered child keeps its own size in the block direction.** In the inline direction it stretches like every other item in a column flex container, as it did under the auto margin. Nothing about its box changes.

### How this is checked

`app/src/lib/primitives/cover.spec.ts` adds three assertions at 1440, 480 and 320:

- A centered child that sets `margin: 0` on its own root in `@layer components` sits midway between its neighbors, more than one `space` from each.
- A centered child with a visible box (a border or background) keeps its content height. Its box is not stretched to fill the free space.
- A cover with no centered child puts its last child one `space` from the bottom padding edge when the cover has no free space, which shows that no spacer is present.

`app/src/lib/styles/primitives.usage.spec.ts` drops its `auto` allowance and refuses a child `margin` of any value. Cover's centered child was the only auto child margin in any primitive. `center-l`'s `margin-inline: auto` is on the host, which the gate never read as a child margin.
