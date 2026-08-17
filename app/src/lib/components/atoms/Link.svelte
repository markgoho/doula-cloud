<script lang="ts">
	import Icon from './Icon.svelte';

	interface Properties {
		href: string;
		label: string;
		variant?: 'primary' | 'secondary';
	}

	let { href, label, variant = 'primary' }: Properties = $props();

	// Absolute (http(s)) or protocol-relative hrefs leave the app; every
	// internal route in this codebase is a relative or root-relative path
	// (see `resolve()` call sites), so this alone tells external apart from
	// internal without a redundant `external` prop for callers to keep in sync.
	const isExternal = $derived(/^(https?:)?\/\//.test(href));
</script>

<a
	{href}
	class={variant}
	target={isExternal ? '_blank' : undefined}
	rel={isExternal ? 'noopener noreferrer' : undefined}
>
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

		a:focus-visible {
			outline: 2px solid var(--color-accent);
			outline-offset: 2px;
		}

		a.primary {
			color: var(--color-accent);
		}

		a.primary:hover {
			color: var(--color-accent-strong);
		}

		a.secondary {
			color: var(--color-text);
		}

		a.secondary:hover {
			color: var(--color-accent);
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
