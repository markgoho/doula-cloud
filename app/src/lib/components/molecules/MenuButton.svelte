<script lang="ts">
	import type { Snippet } from 'svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import type { IconName } from '#lib/components/atoms/Icon/manifest.js';

	/*
	 * A control that reveals a small panel beside itself: the shell's
	 * Practice switcher and its avatar menu (#452).
	 *
	 * Native `popover` does the work. The browser puts the panel in the top
	 * layer, dismisses it on a click outside or on Escape, and returns focus
	 * to the trigger -- three behaviours that are otherwise a document
	 * listener, a key handler and a stored element reference each. The only
	 * script here mirrors the open state onto `aria-expanded`, which not
	 * every engine yet derives from `popovertarget` on its own.
	 *
	 * Deliberately not `role="menu"`. That role is a promise of arrow-key
	 * navigation between items, and these panels hold ordinary links and
	 * buttons that Tab already reaches. `aria-haspopup` is left off for the
	 * same reason.
	 */
	interface Properties {
		/*
		 * The trigger's accessible name, always real DOM text.
		 */
		label: string;
		icon?: IconName;
		iconPosition?: 'start' | 'end';
		/*
		 * Hide the trigger's label visually; the visual or icon carries it.
		 */
		iconOnly?: boolean;
		visual?: Snippet;
		/*
		 * Which edge of the trigger the panel lines up with.
		 */
		align?: 'start' | 'end';
		children: Snippet;
	}

	let {
		label,
		icon,
		iconPosition = 'end',
		iconOnly = false,
		visual,
		align = 'end',
		children
	}: Properties = $props();

	// Per instance, so a component rendered twice in one document -- the
	// avatar menu is in the desktop bar and again inside the narrow sheet --
	// does not collide with itself over one id.
	const instanceId = $props.id();
	const panelId = `menu-panel-${instanceId}`;

	// See BrandLockup: an interpolated class attribute compiles to a nullish
	// check the gate can never satisfy for a prop that has a default.
	const panelClasses = $derived(`panel align-${align}`);

	let isOpen = $state(false);

	function handleToggle(event: ToggleEvent) {
		isOpen = event.newState === 'open';
	}
</script>

<div class="menu">
	<Button
		{label}
		{icon}
		{iconPosition}
		{iconOnly}
		{visual}
		variant="bare"
		popoverTarget={panelId}
		expanded={isOpen}
	/>
	<div id={panelId} popover="auto" class={panelClasses} ontoggle={handleToggle}>
		{@render children()}
	</div>
</div>

<style>
	@layer components {
		.menu {
			display: inline-flex;
		}

		.panel {
			inline-size: max-content;
			min-inline-size: var(--menu-panel-min);
			max-inline-size: min(var(--menu-panel-max), calc(100vw - var(--space-4) * 2));
			margin: 0;
			padding: var(--space-2) 0;
			overflow: hidden;
			border: var(--border-thin) solid var(--color-surface-container-highest);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);

			/* Without anchor positioning a popover is centred in the viewport,
			   which for a top-bar menu reads as a modal that never opened.
			   Pin it under the bar at the inline end instead: not tethered to
			   the trigger, but in the place a person is already looking. */
			position: fixed;
			inset: auto;
			inset-block-start: var(--top-bar-height);
			inset-inline-end: var(--page-gutter);
		}

		@supports (position-area: block-end span-inline-start) {
			.panel {
				/* Implicit anchor: the button that invoked it. */
				position: absolute;
				inset: auto;
				margin-block-start: var(--space-1);
				/* Flip toward the viewport rather than off it -- a switcher
				   near the inline end has no room to span inline-end. */
				position-try-fallbacks: flip-inline;
			}

			.align-start {
				position-area: block-end span-inline-end;
			}

			.align-end {
				position-area: block-end span-inline-start;
			}
		}
	}
</style>
