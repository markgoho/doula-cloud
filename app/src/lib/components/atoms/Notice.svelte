<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		message: string;
		variant: 'error' | 'status' | 'info';
		/**
		 * Whether the Notice takes focus when it appears, and again when
		 * its message changes. For a Notice that reports the result of a
		 * control which is gone once it has been pressed: focus would
		 * otherwise fall to the document, and a keyboard user would start
		 * again from the top of the page. It is focusable from a script
		 * only (tabindex -1), so it adds no stop to the tab order.
		 *
		 * First used by the Engagement hub's "Put me on this Engagement"
		 * (#1598). The two Feedback organisms (#1527, #1528) keep a
		 * focusable wrapper of their own: their Notice appears while a
		 * Drawer closes, the closing dialog hands focus back to the control
		 * that opened it, and their own effect must run after that.
		 */
		isFocusedOnAppear?: boolean;
	}

	let { message, variant, isFocusedOnAppear = false }: Properties = $props();

	let element = $state<HTMLElement>();
	// tabindex -1 and only where asked: reachable from the effect below,
	// never from the Tab key, and absent on every other Notice.
	const focusTarget = $derived(isFocusedOnAppear ? { tabindex: -1 } : {});

	// The shape ErrorSummary's own effect has, for the same reason: a
	// programmatic focus that follows the message, not a keyboard one.
	$effect(() => {
		void message;
		if (isFocusedOnAppear) element?.focus();
	});

	// Matches the role="alert" (assertive) vs role="status" (polite) split
	// already in use ad hoc across the app -- errors interrupt, status/info don't.
	const role = $derived(variant === 'error' ? 'alert' : 'status');

	const iconByVariant: Record<Properties['variant'], IconName> = {
		error: 'warning',
		status: 'check',
		info: 'info'
	};
</script>

<p {role} class={variant} bind:this={element} {...focusTarget}>
	<Icon name={iconByVariant[variant]} size={20} />
	{message}
</p>

<style>
	@layer components {
		p {
			display: flex;
			align-items: flex-start;
			gap: var(--space-2);
			margin: 0;
			padding: var(--space-3) var(--space-4);
			border: var(--border-thin) solid;
			border-radius: var(--radius);
			font-family: var(--font-family-base);
			font-size: var(--text-body-sm-size);
		}

		/* Mixed in oklab, not oklch -- #434. oklch interpolates hue along the
		   shorter polar arc, so a 12% mix drags every variant toward
		   --color-surface's own hue instead of its own. oklab has no hue
		   angle to spiral on, so each variant's tint stays distinct. */
		p.error {
			color: var(--color-error);
			border-color: var(--color-error);
			background-color: color-mix(in oklab, var(--color-error) 12%, var(--color-surface));
		}

		p.status {
			color: var(--color-status);
			border-color: var(--color-status);
			background-color: color-mix(in oklab, var(--color-status) 12%, var(--color-surface));
		}

		/* Focused from a script on appear (isFocusedOnAppear), not from the
		   keyboard, so the ring is on :focus and does not wait on
		   :focus-visible -- the reason ErrorSummary's own ring is
		   unconditional. currentColor is the variant's own color. */
		p:focus {
			outline: var(--focus-ring-width) solid currentColor;
			outline-offset: var(--focus-ring-offset);
		}

		p.info {
			color: var(--color-info);
			border-color: var(--color-info);
			background-color: color-mix(in oklab, var(--color-info) 12%, var(--color-surface));
		}
	}
</style>
