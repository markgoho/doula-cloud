<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		label: string;
		/*
		 * `link`: inline prose that reads as a link rather than a control
		 * with its own chrome -- the GOV.UK phase banner's control
		 * (`molecules/PilotBanner.svelte`, #1522), which sits inside a
		 * sentence rather than beside one. `size` has no effect on it: a
		 * link-in-a-sentence takes exactly the room its own words need,
		 * never the sm/md/lg padding scale the other variants share.
		 *
		 * `menu`: a row in a menu button's panel, the avatar menu's Sign out
		 * (#2141). The full width of the panel and the same weight as the
		 * `Link` rows beside it, which is why it takes no size scale either.
		 */
		variant?: 'primary' | 'secondary' | 'destructive' | 'bare' | 'link' | 'menu';
		size?: 'sm' | 'md' | 'lg';
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		/*
		 * Unavailable, but still focusable (#1920): `aria-disabled` in place
		 * of `disabled`, and a press does nothing. For a button that can be
		 * pressed into its own unavailable state while it has focus -- a
		 * field editor's *Move down* that has just moved its row to the end
		 * of the list. A `disabled` button there drops focus to the page,
		 * and the person is told nothing. Here focus stays, and a screen
		 * reader reads the button as unavailable.
		 */
		ariaDisabled?: boolean;
		/*
		 * The button's own action is in flight. It takes the `ariaDisabled`
		 * form above, not `disabled` (#2119): a `disabled` button drops focus
		 * to the page, so a keyboard user who pressed "Load more" or a
		 * submit, and whose action then failed, started the next Tab at the
		 * top of the page. Here focus stays, a screen reader reads the button
		 * as busy and unavailable, and a press does nothing -- a submit's
		 * press is canceled too, so the form is not sent twice.
		 */
		loading?: boolean;
		icon?: IconName;
		/*
		 * Where the icon sits relative to the label. `end` exists for a
		 * disclosure caret, which has to follow the thing it discloses --
		 * a caret before the Practice name would read as a bullet (#452).
		 */
		iconPosition?: 'start' | 'end';
		iconOnly?: boolean;
		/*
		 * Arbitrary content in place of the icon, for a trigger whose face
		 * is a component rather than a glyph -- the shell's avatar button
		 * (#452). `label` is still rendered as real DOM text, hidden by
		 * `iconOnly` when the visual carries the meaning, so the accessible
		 * name comes from the document rather than from aria-label.
		 */
		visual?: Snippet;
		/*
		 * The id of a `popover` element this button toggles. Native popover
		 * invocation, so the top layer, light dismiss, Escape and returning
		 * focus to this button are the browser's job rather than ours.
		 */
		popoverTarget?: string;
		expanded?: boolean;
		/*
		 * The id of the element this button discloses, when that element
		 * is not the `popoverTarget` above -- `PilotBanner`'s control names
		 * a caller's own `Drawer` (#1521) this way, since a `Drawer` opens
		 * through a bound `open` prop rather than through native popover
		 * invocation.
		 */
		ariaControls?: string;
		/*
		 * Same mechanism as `Link`'s own `describedBy` (#515): a repeated
		 * per-row Button -- DataTable's rowActions, the Client Fields' archived
		 * list's "Collect again" -- reads the same bare word on
		 * every row to a screen-reader user tabbing through or scanning a
		 * rotor's controls list. The caller joins this to a visually-hidden
		 * sibling naming the row, the same way CheckAnswers's Change links
		 * do; the atom stays closed over children so the fix is one prop
		 * rather than a second way to pass content in.
		 */
		describedBy?: string;
		/*
		 * The opener of an in-place form takes focus back after Cancel
		 * (#2103), the way `Heading`'s own `focusOnAppearing` takes it when a
		 * form appears. Cancel removes the form, and the Cancel button with
		 * it, so focus would fall to the page; GOV.UK returns the reader to
		 * where the change was asked from. Most openers are drawn again only
		 * once their form has gone, so this focuses the button the moment it
		 * renders; a row's Change that stays on screen takes focus the moment
		 * this turns true. The caller sets it from Cancel and clears it when
		 * the form opens again, so a save, whose status takes focus, never
		 * moves focus here.
		 */
		focusOnAppearing?: boolean;
		onClick?: (event: MouseEvent) => void;
	}

	let {
		label,
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		ariaDisabled = false,
		loading = false,
		icon,
		iconPosition = 'start',
		iconOnly = false,
		visual,
		popoverTarget,
		expanded,
		ariaControls,
		describedBy,
		focusOnAppearing = false,
		onClick
	}: Properties = $props();

	function takeFocus(element: HTMLButtonElement) {
		element.focus();
	}

	const isUnavailable = $derived(ariaDisabled || loading);

	/*
	 * An unavailable button still receives the click: a press with the
	 * mouse, Enter or Space, or a form's implicit submission from a field.
	 * Canceling it stops what the browser would do next -- send the form,
	 * toggle the popover -- as well as the caller's own action.
	 */
	function handleClick(event: MouseEvent) {
		if (isUnavailable) {
			event.preventDefault();
			return;
		}
		onClick?.(event);
	}

	const iconSize = $derived(size === 'sm' ? 16 : (size === 'lg' ? 24 : 20));
	// `link` and `menu` carry no size scale of their own (see the prop
	// comment above), so the `size-*` class -- and the padding/min-height
	// it sets -- is left off rather than fought with a second,
	// higher-specificity rule.
	const buttonClass = $derived(variant === 'link' || variant === 'menu' ? variant : `${variant} size-${size}`);
	/*
	 * Which buttons fill a narrow container (#2145): a screen's or a
	 * section's own action, a box with words in it. `sm` is the in-row
	 * size (an Invoice's Void, a field's Move up, an Offer's Withdraw), and
	 * a row's actions stay as wide as their words beside the row they act
	 * on. `bare`, `link` and `menu` have no box of their own to fill with,
	 * and a glyph-only button has no words to center in one.
	 */
	const canFillNarrow = $derived(
		['primary', 'secondary', 'destructive'].includes(variant) && size !== 'sm' && !iconOnly
	);
</script>

<button
	{type}
	class={buttonClass}
	class:icon-only={iconOnly}
	class:fills-narrow={canFillNarrow}
	{disabled}
	aria-disabled={isUnavailable || undefined}
	aria-busy={loading}
	aria-expanded={expanded}
	aria-controls={ariaControls}
	aria-describedby={describedBy}
	popovertarget={popoverTarget}
	onclick={handleClick}
	{@attach focusOnAppearing && takeFocus}
>
	{#if loading}
		<span class="spinner" aria-hidden="true"></span>
	{:else if visual}
		{@render visual()}
	{:else if icon && iconPosition === 'start'}
		<Icon name={icon} size={iconSize} weight="light" />
	{/if}
	<span class="label" class:visually-hidden={iconOnly}>{label}</span>
	{#if !loading && !visual && icon && iconPosition === 'end'}
		<Icon name={icon} size={iconSize} weight="light" />
	{/if}
</button>

<style>
	@layer components {
		/*
		 * `stack-l` is a column flex container (ADR-0039), and a flex item
		 * is stretched across the inline axis -- so a button, which is
		 * inline-level and sized by its own words everywhere else, would
		 * run the full width of any stack it is dropped into. `start` hands
		 * it back its intrinsic width. Scoped to `stack-l` rather than
		 * written bare, because `align-self` is the BLOCK axis inside a
		 * row-direction parent (`grid-l`, `switcher-l`) and
		 * this decision is only about the column one.
		 */
		:global(stack-l) > button {
			align-self: start;
		}

		/*
		 * The brief's Density rule: no hit target under 44px, in either
		 * direction, at any width (#1805). The fluid scale shrinks padding
		 * and type at a narrow width, so the floor is a fixed
		 * `min-*-size` on every variant rather than a sum of padding and
		 * line height that a narrow screen can take below it.
		 *
		 * The inline floor is on `.label` below, not here, for the reason
		 * `Link.svelte`'s own `a` rule gives (#1851): an explicit
		 * `min-inline-size` on a flex item replaces its own minimum, its
		 * longest word, so a button in a row with too little room shrank to
		 * 44px and that word drew over the next control.
		 */
		button {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			gap: var(--space-2);
			min-block-size: var(--hit-target-min);
			font-family: var(--font-family-base);
			font-weight: var(--font-weight-medium);
			border-radius: var(--radius);
			border: var(--border-thin) solid transparent;
		}

		button:disabled,
		button[aria-disabled='true'] {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		/*
		 * An unavailable button with its own chrome is a dashed outline in
		 * the muted color with no fill (#2019), not its own fill faded: a faded
		 * plum read as pressed or hovered, and in dark mode it was a
		 * low-contrast mauve. The outline is drawn at full opacity, so its
		 * words stay legible in both themes. A *loading* button is excluded
		 * (`aria-busy`): it is unavailable only while its own request is in
		 * flight, and it keeps its fill so the button just pressed still
		 * looks pressed (the spinner's comment below). `bare` and `link`
		 * have no box to outline, so they keep the faded rule above.
		 */
		button:is(.primary, .secondary, .destructive):is(:disabled, [aria-disabled='true']):not(
				[aria-busy='true']
			) {
			opacity: 1;
			color: var(--color-on-surface-variant);
			background-color: transparent;
			border-color: var(--color-outline);
			/* Dashed, because a solid outline is what an available
			   `secondary` already draws: the two would read the same. */
			border-style: dashed;
		}

		button:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		/*
		 * The 44px inline floor, on the words (#1851). `--button-inset` is
		 * each variant's own inline padding, so the label's floor is what
		 * is left of 44px once both sides of it are drawn: a short label
		 * takes exactly the 44px box it did, and a long one takes no room
		 * it did not.
		 */
		.label {
			min-inline-size: calc(var(--hit-target-min) - 2 * var(--button-inset, 0rem));
		}

		/* A button that is only a glyph has no visible label to hold the
		   floor, so it holds it itself. It has no words to shrink below:
		   at 44px its glyph still sits inside its own padding. */
		button.icon-only {
			min-inline-size: var(--hit-target-min);
		}

		/* `sm` and `md` differ in padding and type only: each is held to the
		   44px floor above, which is taller than the 32px and 40px minimums
		   they once had. */
		button.size-sm {
			--button-inset: var(--space-3);
			padding: var(--space-1) var(--button-inset);
			font-size: var(--text-body-sm-size);
		}

		button.size-md {
			--button-inset: var(--space-4);
			padding: var(--space-2) var(--button-inset);
			font-size: var(--text-body-size);
		}

		button.size-lg {
			--button-inset: var(--space-6);
			min-block-size: var(--control-height-lg);
			padding: var(--space-3) var(--button-inset);
			font-size: var(--text-subheading-size);
		}

		button.primary {
			color: var(--color-on-primary);
			background-color: var(--color-primary);
		}

		button.primary:not(:disabled, [aria-disabled='true']):hover {
			background-color: var(--color-primary-hover);
		}

		button.secondary {
			color: var(--color-on-surface);
			background-color: transparent;
			border-color: var(--color-outline);
		}

		button.secondary:not(:disabled, [aria-disabled='true']):hover {
			background-color: var(--color-outline-variant);
		}

		/* --color-error's lightness tracks --color-primary's per theme (light:
		   dark-on-light, dark: light-on-dark), so --color-on-primary
		   stays legible here too -- no separate error-contrast token needed. */
		button.destructive {
			color: var(--color-on-primary);
			background-color: var(--color-error);
		}

		button.destructive:not(:disabled, [aria-disabled='true']):hover {
			opacity: 0.85;
		}

		/*
		 * Inline prose, not a control with its own box (#1522): no
		 * padding, no border, no background. `display: inline` is
		 * requested rather than the base rule's `inline-flex`, but a
		 * `<button>` stays an atomic, shrink-to-fit box in every engine
		 * regardless -- measured directly: inside a narrow flex item, it
		 * still wraps its own label across lines while sizing itself
		 * against nearly the *container's* width rather than the
		 * remaining space on the sentence's own last line, so it drops to
		 * a line of its own rather than reading as running text. `inline`
		 * is kept anyway (over the base `inline-flex`) because it turns
		 * off `align-items`/`justify-content`/`gap`, which have nothing
		 * to apply to here -- a single text child -- and because it is
		 * the honest name for what this variant is trying to be, even
		 * though the element declines it.
		 *
		 * `text-align: inherit` overrides the UA stylesheet's own
		 * `text-align: center` on `button` -- without it, that shrink-
		 * to-fit box centers its wrapped label under the sentence above
		 * it rather than reading left-aligned like the prose beside it
		 * (confirmed on `/style-guide/pilot-banner` at 320px). `border: 0`
		 * rather than only `border-color: transparent`: the base rule's
		 * border is a real `--border-thin`, and a transparent border
		 * still occupies that space.
		 *
		 * `Link.svelte`'s `.primary` sets the same color and hover
		 * treatment for the same reason: this reads as that kind of link,
		 * not as a button that happens to look like one.
		 */
		button.link {
			display: inline;
			padding: 0;
			border: 0;
			background-color: transparent;
			color: var(--color-primary);
			font-weight: var(--font-weight-normal);
			text-align: inherit;
			text-decoration: underline;
		}

		button.link:not(:disabled, [aria-disabled='true']):hover {
			color: var(--color-primary-hover);
		}

		/*
		 * The 44px floor is kept for a `link` that stands on its own -- the
		 * agreement screen's "Sign out", an entry in a list (#1828) -- and
		 * dropped for one inside a paragraph, which is part of a sentence:
		 * the brief excepts that one the way WCAG 2.2's 2.5.8 does
		 * ("Inline"), and `Link.svelte` draws the same line at the same
		 * place. A `<button>` is an atomic box even styled `inline`, so
		 * here the floor has to be taken off rather than being ignored.
		 */
		:global(p) button.link {
			min-block-size: 0;
		}

		:global(p) button.link .label {
			min-inline-size: 0;
		}

		/* A button in a table row takes the row's height as its floor, in
		   both directions (#1843, the brief's Density section):
		   `--row-target-min` is the row less its 1px rule, 39px when every
		   input is fine and 44px when any input can tap, so the row and
		   its targets are always the same height. */
		:global(td) button {
			min-block-size: var(--row-target-min);
		}

		/* Written on the two sizes because each size's own `padding`
		   rule above is more specific than a bare `td button`. */
		:global(td) button:is(.size-sm, .size-md) {
			/* What the target leaves around one line, over its two edges,
			   not the size's own `--space-1` (#1945): at the 16px type
			   floors (ADR-0060) one line of `body-sm` plus that padding and
			   the two borders came to 42.8px at the ramp's ceiling, so the
			   row grew past 40px. Derived from `1lh`, one line fits the
			   target at every width, and a label that wraps keeps the same
			   gap. Rounded down to a whole pixel for the reason
			   `DataTable.svelte`'s `td.content` gives. */
			padding-block: round(
				down,
				calc((var(--row-target-min) - 2 * var(--border-thin) - 1lh) / 2),
				var(--border-thin)
			);
		}

		:global(td) button.icon-only {
			min-inline-size: var(--row-target-min);
		}

		:global(td) .label {
			min-inline-size: calc(var(--row-target-min) - 2 * var(--button-inset, 0rem));
		}

		/*
		 * A screen's or a section's own action fills a narrow container
		 * and keeps its own width in a wide one (#2145), GOV.UK's Button
		 * behavior read as the space the button is given rather than the
		 * window (ADR-0024 rule 2). In a narrow column a button as wide as
		 * its words is a short target at one edge with an empty run beside
		 * it; full width it is the whole column's width to press, and a
		 * primary and a secondary in one `cluster-l` stack, each the
		 * column's width, in their source order -- primary first. In a
		 * wide column a full-width button is a bar that reads as a banner,
		 * not a control, so it keeps its own width there.
		 *
		 * The query reads the nearest containment context: a Template's
		 * `container-l` (the page's own frame), `RecordDetail`'s record, a
		 * `DataTable`'s own box, `Drawer`'s scope, the drag surface's
		 * frame, or `body` where none holds the button. None of those is a
		 * dialog panel or a rail card, so a dialog or a card on a wide
		 * screen is answered by the page it sits on (seen 2026-10-27: the
		 * confirm dialog's two buttons keep their own width at 1440).
		 *
		 * 37rem is a content floor, not a device: the width at which the
		 * column holds a reading measure (`--measure`, 65ch, 585px at the
		 * type the column is set in there). Prose is capped at a measure,
		 * so in a narrower column every paragraph runs the column's full
		 * width, and the action under it fills the same edge to edge; once
		 * the column is wider than a measure, prose stops short of its
		 * edge and a full-width button would run past the text it acts on.
		 * The floor check measures it (`floor.svelte.spec.ts`, the measure
		 * criterion on the Width demo of `/style-guide/button`): measured on
		 * Linux, as CI is, where the fixed point is 585px (36.6rem)
		 * and 37rem is the next whole rem, not a figure chosen first. GOV.UK's
		 * own breakpoint (40.0625em of window) is a device number and is
		 * not used. `Link.svelte`'s start button holds the same floor.
		 *
		 * No intrinsic mechanism picks between the two widths: the
		 * query-free form (Every Layout's Switcher arithmetic on
		 * `min-inline-size`) leaves an explicit minimum on the button in
		 * a wide column, which replaces a flex item's own (its longest
		 * word) -- the #1851 defect the `.label` floor exists to avoid --
		 * and `calc-size()`, which could say "own width", is Chromium's
		 * alone. So `inline-size`, set wide by the query below. A table
		 * cell keeps its buttons at their own width at every width -- a
		 * row's action is beside the row.
		 */
		button.fills-narrow {
			inline-size: 100%;
		}

		@container (min-width: 37rem) {
			button.fills-narrow {
				inline-size: auto;
			}
		}

		:global(td) button.fills-narrow {
			inline-size: auto;
		}

		/* Chrome controls: the shell's hamburger, avatar and Practice
		   switcher. No border and no fill, because a top bar that draws a
		   box round each of its own controls stops reading as one surface --
		   but the hit area is still the 44px floor every variant takes from
		   the base rule, whatever the glyph inside measures (the avatar is
		   34px). */
		button.bare {
			--button-inset: var(--space-2);
			padding: var(--space-1) var(--button-inset);
			border-color: transparent;
			background-color: transparent;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-normal);
			/* The Practice switcher's name can wrap in the narrow bar, and it
			   reads from the start there (#2141), not centered under itself
			   as the UA's own `text-align: center` on a button would draw it. */
			text-align: start;
		}

		button.bare:not(:disabled, [aria-disabled='true']):hover {
			color: var(--color-primary);
		}

		/* A row in a menu button's panel (#2141): the width of the panel,
		   words at the start, no box of its own, and the 44px floor from the
		   base rule. It reads as one of the panel's items rather than as a
		   control set down inside it. The focus ring is drawn inside the
		   row, since the panel clips what is drawn outside it. */
		button.menu {
			--button-inset: var(--space-4);
			justify-content: flex-start;
			inline-size: 100%;
			padding: var(--space-2) var(--button-inset);
			border: 0;
			border-radius: 0;
			background-color: transparent;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-normal);
			text-align: start;
		}

		button.menu:not(:disabled, [aria-disabled='true']):hover {
			background-color: var(--color-surface-container);
		}

		button.menu:focus-visible {
			outline-offset: calc(-1 * var(--focus-ring-width));
		}

		/* The brief keeps this spinner rather than replacing it: a small
		   indicator inside the button a person just pressed is the most
		   conventional loading affordance there is, and Jakob's Law is the
		   brief's governing law. What the brief does not allow is movement a
		   person cannot switch off, so the rotation is gated below and
		   `reduce` gets the ring standing still. The feedback survives that:
		   the ring is still drawn, and `[aria-disabled='true']` still dims the whole
		   control over its own fill (the outline rule above leaves a
		   loading button alone), so a pressed button still looks pressed.
		   See #418. */
		.spinner {
			/* tokens:ignore -- `1em` is the label's own size, so the ring
			   is as tall as the words it stands in for. */
			inline-size: 1em;
			block-size: 1em;
			/* tokens:ignore -- the ring's own stroke. Not --border-active,
			   which means "this control is the current one"; this is the
			   width of a drawn circle and moves with nothing else. */
			border: 2px solid currentColor;
			border-inline-end-color: transparent;
			border-radius: 50%;
		}

		@media (prefers-reduced-motion: no-preference) {
			/* motion:ignore -- a continuous indeterminate rotation is not a
			   state change, an entrance, or a navigation, so none of the three
			   motion tokens describes it; and `--ease-out` on an infinite loop
			   would make it lurch once per turn. This is the only place in the
			   app that legitimately needs a raw duration and `linear`. */
			.spinner {
				animation: spin 700ms linear infinite;
			}
		}

		/* motion:ignore -- the app's only keyframe animation that moves
		   anything (Skeleton's changes opacity once). It is the in-button
		   loading indicator above, already gated on
		   prefers-reduced-motion, and it moves nothing else on the page. */
		@keyframes spin {
			to {
				transform: rotate(360deg);
			}
		}
	}
</style>
