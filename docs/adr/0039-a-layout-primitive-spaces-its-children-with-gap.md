# A layout primitive spaces its children with `gap`, not with their margins

`stack-l` spaces its children with `gap` on the container, under `display: flex; flex-direction: column`. It used to space them with `stack-l > * + * { margin-block-start: … }`, and that line — the one ADR-0003 calls "Stack's adjacent-sibling technique" — is what this decision amends. `reel-l` moves with it, for the same reason and in the same pass. `cover-l` deliberately does not; that is recorded below and gets its own answer in [#1220](https://github.com/markgoho/doula-cloud/issues/1220).

## The mechanism this replaces, and why it could not stay

A primitive's rules — both `primitives.css`'s zero-JS defaults and the per-instance rules `defineLayoutPrimitive` injects — live in `@layer utilities`. Every component's scoped Svelte `<style>` compiles into `@layer components`, which `app/src/lib/styles/app.css` declares *after* it: `@layer reset, tokens, base, utilities, components;`. A later layer beats an earlier one outright, whatever the specificity on either side.

So a component whose root element carried `margin: 0` — the ordinary reset for a bare `<p>` or `<h2>`, sitting in `Notice`, `Text`, `Heading`, `WarningText`, `DescriptionList` and roughly twenty others — canceled every ounce of spacing a Stack was trying to give it. Nothing about the CSS looks wrong at either end. The component is doing the normal thing; the primitive is doing the documented thing; the layer order is the one ADR-0003 chose on purpose. The spacing simply is not there, and #1105 measured it on `/signup` at 1440, 480 and 320: the `Notice` about the founder holding every role sat flush against the Password field while every `LabeledField` around it got its 27px / 21px / 20px.

A `gap` is not in that fight at all. It is the container's own geometry, applied between flex items as they are placed. A child has no declaration that can cancel it — not `margin: 0`, not `!important`, not a later layer — because nothing about the gap is expressed on the child.

## The candidates, and why the other three lose

**Move the primitives' injected rules into a layer after `components`.** Rejected. It fixes the symptom by inverting the thing ADR-0003 built the layer order to achieve: a page-layout utility would then beat every component style, so a component could no longer adjust anything a primitive touched. That is a larger cascade problem than the one being solved, and it arrives everywhere at once.

**Keep sibling margins and forbid a bare `margin: 0` on a component's root element.** Rejected, because it is not checkable. "Root element" is a fact about a Svelte component's markup, not about its CSS, and the rule would have to be enforced against CSS text — where `p { margin: 0 }` and `.hint { margin: 0 }` are indistinguishable in kind and only one of them is the root. `tokens.usage.spec.ts`'s shape does not reach this, and a rule with no gate is a comment.

**Keep sibling margins and win with `!important` in `@layer utilities`.** Rejected. It would work — `@layer` inverts `!important`, so an important declaration in the *earliest* layer beats a normal one in the latest — but reaching for `!important` is the thing the layer order exists to avoid, and ADR-0003 says so in as many words while explaining why there is no Shadow DOM here. A mechanism that needs `!important` to survive the cascade is a mechanism that will need it again.

**`gap` on the container.** Chosen. It is what Every Layout's own Stack does; ADR-0003 already cites their move to `gap` as a reason not to depend on their shipped code. It needs no layer change, no `!important` and no rule about what a component may write about itself.

## What the reset makes safe, and what flex changes

`reset.css` carries `*:not(dialog) { margin: 0 }` in `@layer reset`. Nothing in the app is deliberately margined unless it says so, so a container gap has nothing to double up with; a child that *does* deliberately margin itself adds to the gap rather than replacing it, which is a visible, local decision at the call site rather than a silent one.

Two consequences of `display: flex` are worth writing down, because both had to be paid:

- **Flex blockifies every child.** That retires a caveat Stack used to carry: an inline child (a bare `<label>`) or an inline-block one (a text input) shared a line with its neighbor and got no visible gap at all — what #451/#425 found in `LabeledField`. A caller no longer has to blockify its own children for the stack to space them.
- **A column flex container stretches its items across the inline axis**, where block layout let an inline-level child shrink-wrap. A child that must keep its intrinsic width says `align-self: start` at the call site. Which children those are was found by probing every direct `stack-l` child in a real browser — cloning each one into a plain block context and reading back its natural `display` — rather than by reading the CSS, because a component's own `display` is not visible from the stack and the computed value on a flex item is already blockified. Four: `DataTable`'s `.table-view`, where #542 deliberately removed `inline-size: 100%` so a table stops where its columns are satisfied and stretching put that width back by another route; and `Button`, `Link` and `Badge`, each sized by its own words everywhere else — a full-bleed submit button, a link whose hit area spans the page, and a status pill stretched across a row are all wrong in the same way. Those three scope the declaration to `:global(stack-l) > …` rather than writing it bare, because `align-self` is the *block* axis inside a row-direction parent (`grid-l`, `sidebar-l`, `switcher-l`) and this decision is only about the column one. `TextInput`, `Textarea` and `Select` are inline-level too and needed nothing: each already declares its own full width, which is also why this change does not touch what [#805](https://github.com/markgoho/doula-cloud/issues/805) is deciding about a text input's width. **That is a standing obligation, not a one-time sweep**: a new component whose root is inline-level and sized by its own content owes either the `:global(stack-l) > … { align-self: start }` rule or a written reason it wants to stretch. Nothing checks it — `primitives.usage.spec.ts` reads the primitives, not the components that go inside them — and a source gate on "an atom declaring `display: inline-flex`" would fire on every atom that is never a stack child, which is the routinely-suppressed shape `layout.usage.spec.ts` declines to build. So it is written here for the next author to read instead, and the probe above is how to answer it for real: mount the screen, clone each direct `stack-l` child into a plain block context, and read back its natural `display`.

The one real breakage in the app was `QuestionPage`'s label-as-h1 branch, where `.thing` sat as a direct `stack-l` child while overriding the stack's `--space-6` to `--space-7`. Under sibling margins that replaced; under `gap` it adds. The wider step is now a nested `stack-l space="var(--space-7)"`, and the `.thing` margin is scoped to the fieldset branch, where `.thing` is a `<fieldset>` child and no stack spaces it.

## `cover-l` is deliberately margined, and stays that way for now

Cover's centering *is* an auto margin — `cover-l > h1 { margin-block: auto }` — and `gap` cannot express "`space` at the ends and `auto` in the middle": a gap is uniform between items and writes nothing at the edges. The 2×space it puts between two adjacent non-centered children is deliberate as well, and a gap would halve it. So the answer that fits Stack does not transfer, and Cover carries the same latent exposure Stack just lost. That is recorded here rather than left silent, and [#1220](https://github.com/markgoho/doula-cloud/issues/1220) is where Cover gets its own answer.

## How this is checked

Two seams, because neither alone is honest.

`app/src/lib/primitives/stack.spec.ts` renders the defect: a `stack-l` whose second child resets its own margin in `@layer components`, measured against an otherwise identical stack whose child resets nothing. It asserts geometry — the second child's top edge less the first's bottom edge — and not a computed margin, because under the fix that child's `margin-block-start` is still `0px`; the space belongs to the container. Both stacks are measured in the same document against the real `app.css`, so the layer order under test is the one that ships.

`app/src/lib/styles/primitives.usage.spec.ts` reads the mechanism: no `margin*` declaration under a selector containing `>`, in `primitives.css` or in the rules each `primitiveSpecs` entry's own `css()` generates. `auto` is allowed, since an auto margin is alignment rather than spacing. A host rule is untouched — a primitive may margin itself; what it may not do is margin something it does not own. This is a source check rather than a rendering one for the same reason ADR-0025's rule 3 is: the cancellation is a property of a *pair* — a primitive that writes a child margin, and a component that resets its own — and either half can arrive long after the other, on a screen no rendering test happens to cover. `primitives:ignore` with a reason is the escape hatch, scoped to the rule block, exactly as `tokens:ignore` and `layout:ignore` already work here; `cover-l` is its only user.

## The enumeration, not a sample

#1105's third acceptance criterion asks for every component whose root element resets its margin, enumerated rather than sampled. There are two halves to it, and they are asymmetric.

**The 45 margin resets are fixed by the mechanism, all at once.** `margin: 0` appears in 33 files under `app/src`, and not one of them needs a change: a reset cancels a sibling margin and cannot cancel a container's gap, wherever the reset is written and whatever it is written on. That is the whole point of moving the spacing. Listed so the claim is checkable rather than asserted — `Heading`, `Notice`, `Text`, `Textarea`, `WarningText`, `AvatarMenu` (×2), `DescriptionList` (×2), `ErrorSummary`, `HistoryDisclosure`, `LabeledField`, `MembershipFields`, `MenuButton`, `PracticeSwitcher` (×2), `ClientFieldAnswers`, `DataTable` (×2), `MessageThread`, `StaffTopBar` (×2), `StepRail`, `CheckAnswers` (×3), `FormPage`, `OverviewHub`, `QuestionPage` (×2), `RecordDetail`, and the route styles in `account/+layout`, `account/+page`, `clients/+page`, `clients/search/+page`, `schedule/+page` (×2), `settings/+page`, `settings/payments/+page`, `style-guide/data-table/+page`, `style-guide/menu-button/+page`, and the layout exercise's two contact cards (×2 each).

**The declarations that had to be judged one at a time are the *non-zero* margins**, because those are the ones whose relationship to the stack inverted: a non-zero sibling margin used to *replace* the stack's spacing and now *adds* to it. A margin is only in that position if the element carrying it is a direct `stack-l` child. Every non-zero block-margin declaration under `app/src`:

| Where | Rule | Direct `stack-l` child? | Disposition |
| --- | --- | --- | --- |
| `QuestionPage` | `.thing { margin-block-start: --space-7 }` | **Yes**, in the label-as-h1 branch | **Fixed.** Nested `stack-l space="var(--space-7)"`; the margin is scoped to `fieldset .thing`, where it is a `<fieldset>` child |
| `DataTable` | `.record-view { margin-block-start: 0 }` | Yes | **Removed.** It existed to cancel a gap the hidden `.table-view` sibling used to create; a `display: none` child is not a flex item, so there is nothing left to cancel |
| `DateFields` | `legend`, `.hint`, `.error` | No — `<fieldset>` children | Unaffected |
| `MembershipFields` | `legend`, `fieldset.refused legend`, `.error` | No — `<fieldset>` children | Unaffected |
| `RadioGroup` | `legend`, `.error`, `.description` | No — `<fieldset>` and `<label>` children | Unaffected |
| `ClientFieldAnswers` | `legend` | No — `<fieldset>` child | Unaffected |
| `FormPage` | `legend` | No — `<fieldset>` child | Unaffected |
| `ErrorSummary` | `li + li` | No — `<ul>` children | Unaffected |
| `settings/payments/+page` | `li + li` | No — `<ul>` children | Unaffected |
| `StepRail` | `details > ol`, `.status`, `.questions` | No — `<details>` and `<li>` descendants | Unaffected |
| `MenuButton` | `.panel` (inside `@supports`) | No — an absolutely positioned popover | Unaffected |
| `Select` | `select::picker(select)` | No — a pseudo-element on the control | Unaffected |
| `MessageThread` | `input[type='file']` | No — a `<label>` child | Unaffected |
| `RecordDetail` | `.summary` | No — a `.record-header` child | Unaffected |
| `RecordDetail` | `section { scroll-margin-block-start }` | Yes, but `scroll-margin` is not a margin | Unaffected |
| `DataTable` | `.record-view dl + dl` | No — `.record-view` children | Unaffected |
| `CheckAnswers` | `.caption` | No — a `<span>` inside a row's heading | Unaffected |
| `QuestionPage` | `.caption`, `.hint` | No — an `<h1>` child and a wrapper `<div>` child | Unaffected |
| `schedule/+page` | `.controls` | No — a `<fieldset>` child | Unaffected |
| `(signed-out)/+page` | `.status` | No — an `<li>` child | Unaffected |
| `style-guide/+layout` | `.viewport` | No — a sibling of the `box-l` that holds the stack | Unaffected |

The fix was then measured on the real `/signup` at 1440, 480 and 320, which is where #1105 found it. The `Notice` now sits 26.9px / 20.9px / 20px below the Password field — the stack's own space at each width — while its own `margin-block-start` still computes to `0px`, which is the whole point: the space belongs to the container, and the child's reset has nothing left to cancel.

`reel-l` moved to `gap` in the same pass and has no callers in `app/src` yet, so nothing was exposed to its `margin-inline-start` and nothing had to be judged for it.
