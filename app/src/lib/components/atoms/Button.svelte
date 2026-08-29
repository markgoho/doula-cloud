<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		label: string;
		variant?: 'primary' | 'secondary' | 'destructive' | 'bare';
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
		onClick
	}: Properties = $props();

	const isDisabled = $derived(disabled || loading);
	const iconSize = $derived(size === 'sm' ? 16 : (size === 'lg' ? 24 : 20));
	const buttonClass = $derived(`${variant} size-${size}`);
</script>

<button
	{type}
	class={buttonClass}
	disabled={isDisabled}
	aria-busy={loading}
	aria-expanded={expanded}
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
			opacity: 0.6;
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

		/* WCAG-standard clip technique: stays in the accessibility tree and
		   readable by AT/voice-control/translation tools, unlike aria-label
		   which strips real DOM text out of those paths. */
		/* tokens:ignore -- the WCAG clip technique's own geometry, not a
		   design value. The 1px box and the -1px pull are what the
		   technique is; a token would imply somebody may retune them. */
		.visually-hidden {
			position: absolute;
			inline-size: 1px;
			block-size: 1px;
			margin: -1px;
			padding: 0;
			overflow: hidden;
			clip: rect(0, 0, 0, 0);
			white-space: nowrap;
			border: 0;
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
