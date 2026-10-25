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
		 */
		variant?: 'primary' | 'secondary' | 'destructive' | 'bare' | 'link';
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
		 * per-row Button -- DataTable's rowActions, DynamicFieldEditor's
		 * "Move up"/"Move down"/"Remove" -- reads the same bare word on
		 * every row to a screen-reader user tabbing through or scanning a
		 * rotor's controls list. The caller joins this to a visually-hidden
		 * sibling naming the row, the same way CheckAnswers's Change links
		 * do; the atom stays closed over children so the fix is one prop
		 * rather than a second way to pass content in.
		 */
		describedBy?: string;
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
		onClick
	}: Properties = $props();

	const isDisabled = $derived(disabled || loading);
	const iconSize = $derived(size === 'sm' ? 16 : (size === 'lg' ? 24 : 20));
	// `link` carries no size scale of its own (see the prop comment above),
	// so the `size-*` class -- and the padding/min-height it sets -- is
	// left off rather than fought with a second, higher-specificity rule.
	const buttonClass = $derived(variant === 'link' ? 'link' : `${variant} size-${size}`);
</script>

<button
	{type}
	class={buttonClass}
	class:icon-only={iconOnly}
	disabled={isDisabled}
	aria-disabled={ariaDisabled || undefined}
	aria-busy={loading}
	aria-expanded={expanded}
	aria-controls={ariaControls}
	aria-describedby={describedBy}
	popovertarget={popoverTarget}
	onclick={ariaDisabled ? undefined : onClick}
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
		 * row-direction parent (`grid-l`, `sidebar-l`, `switcher-l`) and
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
			min-block-size: 3rem;
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
		}

		button.bare:not(:disabled, [aria-disabled='true']):hover {
			color: var(--color-primary);
		}

		/* The brief keeps this spinner rather than replacing it: a small
		   indicator inside the button a person just pressed is the most
		   conventional loading affordance there is, and Jakob's Law is the
		   brief's governing law. What the brief does not allow is movement a
		   person cannot switch off, so the rotation is gated below and
		   `reduce` gets the ring standing still. The feedback survives that:
		   the ring is still drawn, and `button:disabled` still dims the whole
		   control, so a pressed button still looks pressed. See #418. */
		.spinner {
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

		/* motion:ignore -- the app's only keyframe animation. It is the
		   in-button loading indicator above, already gated on
		   prefers-reduced-motion, and it moves nothing else on the page. */
		@keyframes spin {
			to {
				transform: rotate(360deg);
			}
		}
	}
</style>
