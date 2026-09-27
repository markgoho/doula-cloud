# Cover spaces its children with `gap`, and centers with one auto margin

`cover-l` spaces its children with `gap` on the container, like `stack-l` ([ADR-0039](0039-a-layout-primitive-spaces-its-children-with-gap.md)). The centered child keeps `margin-block: auto`. Cover no longer writes `> * { margin-block: <space> }` or the two end-caps that went with it. The step between two adjacent non-centered children is now one `space`, not two. Decided on [#1220](https://github.com/markgoho/doula-cloud/issues/1220), 2026-09-27. This amends ADR-0039's Cover section, which kept Cover margined.

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

The centered child's `margin-block: auto` is still a child margin in `@layer utilities`. A centered child that resets `margin: 0` on its own root element in `@layer components` wins the cascade and is no longer centered. `Heading` does exactly that. The gate allows `auto`, so it will not catch this. [#1482](https://github.com/markgoho/doula-cloud/issues/1482) is where that gets its own answer. Every candidate there changes how a centered child with a visible box renders, so it is a separate decision, not part of this one.

## How this is checked

`app/src/lib/primitives/cover.spec.ts` renders Cover in a real browser at 1440, 480 and 320 and measures geometry, as `stack.spec.ts` does. It asserts four things. Two adjacent non-centered children are one `space` apart. A non-first child that resets `margin: 0` in `@layer components` gets the same step as one that does not. The centered child sits midway between its neighbors, more than one `space` from each. The first and last children sit against the padding. `space` is the cover's own computed padding, because every `--space-N` is a `clamp()` with a `cqi` and has no fixed pixel value.

`app/src/lib/styles/primitives.usage.spec.ts` no longer exempts anything. The three `primitives:ignore` markers on Cover's rules and the `EXEMPT` map that excused `cover-l`'s generated rules are gone, and so is the test that checked the exemption was still used.
