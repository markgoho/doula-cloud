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
	disabled={isDisabled}
	aria-busy={loading}
	aria-expanded={expanded}
	aria-controls={ariaControls}
	aria-describedby={describedBy}
	popovertarget={popoverTarget}
	onclick={onClick}
>
	{#if loading}
		<span class="spinner" aria-hidden="true"></span>
	{:else if visual}
		{@render visual()}
	{:else if icon && iconPosition === 'start'}
		<Icon name={icon} size={iconSize} weight="light" />
	{/if}
	<span class:visually-hidden={iconOnly}>{label}</span>
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

		button {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			gap: var(--space-2);
			font-family: var(--font-family-base);
			font-weight: var(--font-weight-medium);
			border-radius: var(--radius);
			border: var(--border-thin) solid transparent;
		}

		button:disabled {
			cursor: not-allowed;
			opacity: var(--opacity-disabled);
		}

		button:focus-visible {
			outline: var(--focus-ring-width) solid var(--color-primary);
			outline-offset: var(--focus-ring-offset);
		}

		button.size-sm {
			min-height: 2rem;
			padding: var(--space-1) var(--space-3);
			font-size: var(--text-body-sm-size);
		}

		button.size-md {
			min-height: 2.5rem;
			padding: var(--space-2) var(--space-4);
			font-size: var(--text-body-size);
		}

		button.size-lg {
			min-height: 3rem;
			padding: var(--space-3) var(--space-6);
			font-size: var(--text-subheading-size);
		}

		button.primary {
			color: var(--color-on-primary);
			background-color: var(--color-primary);
		}

		button.primary:not(:disabled):hover {
			background-color: var(--color-primary-hover);
		}

		button.secondary {
			color: var(--color-on-surface);
			background-color: transparent;
			border-color: var(--color-outline);
		}

		button.secondary:not(:disabled):hover {
			background-color: var(--color-outline-variant);
		}

		/* --color-error's lightness tracks --color-primary's per theme (light:
		   dark-on-light, dark: light-on-dark), so --color-on-primary
		   stays legible here too -- no separate error-contrast token needed. */
		button.destructive {
			color: var(--color-on-primary);
			background-color: var(--color-error);
		}

		button.destructive:not(:disabled):hover {
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
			min-block-size: 0;
			padding: 0;
			border: 0;
			background-color: transparent;
			color: var(--color-primary);
			font-weight: var(--font-weight-normal);
			text-align: inherit;
			text-decoration: underline;
		}

		button.link:not(:disabled):hover {
			color: var(--color-primary-hover);
		}

		/* Chrome controls: the shell's hamburger, avatar and Practice
		   switcher. No border and no fill, because a top bar that draws a
		   box round each of its own controls stops reading as one surface --
		   but the hit area is still the 44px WCAG 2.5.5 target whatever the
		   glyph inside measures (the avatar is 34px). */
		button.bare {
			min-block-size: var(--hit-target-min);
			min-inline-size: var(--hit-target-min);
			padding: var(--space-1) var(--space-2);
			border-color: transparent;
			background-color: transparent;
			color: var(--color-on-surface);
			font-size: var(--text-body-sm-size);
			font-weight: var(--font-weight-normal);
		}

		button.bare:not(:disabled):hover {
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
