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
	 * to the trigger -- three behaviors that are otherwise a document
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

	/*
	 * The box the panel opens under (#1573). It is the wrapper rather than
	 * the button because a top bar stretches the wrapper to its own height,
	 * so the wrapper's bottom edge is the bar's -- and a bar grows when a
	 * long Practice name wraps. Named per instance, for the same reason as
	 * the panel id above.
	 */
	const anchorName = `--menu-${instanceId}`;
	let menu = $state<HTMLDivElement>();

	let isOpen = $state(false);

	/*
	 * Where the fallback below pins the panel, read as it opens: the bar's
	 * height is no longer a constant a stylesheet can name. Set whether or
	 * not the engine has anchor positioning -- where it has, the rule that
	 * reads this is overridden and the number is simply unused.
	 */
	let fallbackTop = $state<string>();

	function handleBeforeToggle(event: ToggleEvent) {
		if (menu && event.newState === 'open') {
			fallbackTop = `${menu.getBoundingClientRect().bottom}px`;
		}
	}

	function handleToggle(event: ToggleEvent) {
		isOpen = event.newState === 'open';
	}
</script>

<div class="menu" bind:this={menu} style:anchor-name={anchorName}>
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
	<div
		id={panelId}
		popover="auto"
		class={panelClasses}
		style:position-anchor={anchorName}
		style:--menu-fallback-top={fallbackTop}
		onbeforetoggle={handleBeforeToggle}
		ontoggle={handleToggle}
	>
		{@render children()}
	</div>
</div>

<style>
	@layer components {
		/* Stretched to whatever row holds it, with the button centered
		   inside, so in a top bar this box's bottom edge is the bar's own
		   and the panel opens below the bar however tall it grew (#1573). */
		.menu {
			display: inline-flex;
			align-self: stretch;
			align-items: center;
		}

		.panel {
			inline-size: max-content;
			min-inline-size: var(--menu-panel-min);
			max-inline-size: min(var(--menu-panel-max), calc(100dvw - var(--space-4) * 2));
			margin: 0;
			padding: var(--space-2) 0;
			overflow: hidden;
			border: var(--border-thin) solid var(--color-surface-container-highest);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
			color: var(--color-on-surface);
			font-family: var(--font-family-base);

			/* Without anchor positioning a popover is centered in the viewport,
			   which for a top-bar menu reads as a modal that never opened.
			   Pin it under the bar at the inline end instead: not tethered to
			   the trigger, but in the place a person is already looking.
			   Under the bar's real bottom edge, read as the panel opens, since
			   a wrapped Practice name makes the bar taller than
			   `--top-bar-height` (#1573). */
			position: fixed;
			inset: auto;
			inset-block-start: var(--menu-fallback-top, var(--top-bar-height));
			inset-inline-end: var(--page-gutter);
		}

		@supports (position-area: block-end span-inline-start) {
			.panel {
				/* Anchored to the wrapper named above rather than to the button
				   that invoked it: the wrapper spans the bar's height. */
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
