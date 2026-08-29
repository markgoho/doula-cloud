<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		href: string;
		label: string;
		variant?: 'primary' | 'secondary' | 'card';
		icon?: IconName;
		current?: boolean;
	}

	let { href, label, variant = 'primary', icon, current = false }: Properties = $props();

	// Absolute (http(s)) or protocol-relative hrefs leave the app; every
	// internal route in this codebase is a relative or root-relative path
	// (see `resolve()` call sites), so this alone tells external apart from
	// internal without a redundant `external` prop for callers to keep in sync.
	const isExternal = $derived(/^(https?:)?\/\//.test(href));
</script>

<a
	{href}
	class={variant}
	class:has-icon={Boolean(icon)}
	aria-current={current ? 'page' : undefined}
	target={isExternal ? '_blank' : undefined}
	rel={isExternal ? 'noopener noreferrer' : undefined}
>
	{#if icon}
		<Icon name={icon} size={16} />
	{/if}
	{label}
	{#if isExternal}
		<Icon name="arrow-square-out" size={16} />
		<span class="visually-hidden">(opens in new tab)</span>
	{/if}
</a>

<style>
	@layer components {
		a {
			display: inline-flex;
			align-items: center;
			gap: var(--space-1);
			font-family: var(--font-family-base);
			text-decoration: underline;
		}

		/* A link carrying its own icon (a nav item, a card) reads as a
		   control, not inline prose -- the underline that marks prose links
		   would be visual noise here. */
		a.has-icon {
			text-decoration: none;
		}

		a:focus-visible {
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
		}

		a[aria-current='page'] {
			color: var(--color-primary);
			font-weight: var(--font-weight-semibold);
		}

		a.primary {
			color: var(--color-primary);
		}

		a.primary:hover {
			color: var(--color-primary-hover);
		}

		a.secondary {
			color: var(--color-on-surface);
		}

		a.secondary:hover {
			color: var(--color-primary);
		}

		/* A block-level tile: icon over label, its own chrome. tokens.css has
		   no --color-surface-bright / --space-5 yet -- both are asked for by name
		   with a light fallback so this picks them up unchanged once they
		   land (see docs/design/brief.md). */
		a.card {
			flex-direction: column;
			align-items: flex-start;
			gap: 14px;
			inline-size: 100%;
			padding: var(--space-5, 1.25rem);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright, oklch(99% 0.004 320));
			color: var(--color-on-surface);
			font-size: var(--text-body-size);
			font-weight: var(--font-weight-medium);
			line-height: 1.2;
		}

		a.card:hover {
			border-color: var(--color-primary);
		}

		a.card :global(svg) {
			color: var(--color-on-surface-variant);
		}

		a.card:hover :global(svg) {
			color: var(--color-primary);
		}

		/* WCAG-standard clip technique: stays in the accessibility tree and
		   readable by AT/voice-control/translation tools, unlike aria-label
		   which strips real DOM text out of those paths (see Button.svelte). */
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
	}
</style>
