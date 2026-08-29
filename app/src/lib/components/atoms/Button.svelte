<script lang="ts">
	import Icon from './Icon.svelte';
	import type { IconName } from './Icon/manifest.js';

	interface Properties {
		label: string;
		variant?: 'primary' | 'secondary' | 'destructive';
		size?: 'sm' | 'md' | 'lg';
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		loading?: boolean;
		icon?: IconName;
		iconOnly?: boolean;
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
		iconOnly = false,
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
	onclick={onClick}
>
	{#if loading}
		<span class="spinner" aria-hidden="true"></span>
	{:else if icon}
		<Icon name={icon} size={iconSize} weight="light" />
	{/if}
	<span class:visually-hidden={iconOnly}>{label}</span>
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
			outline: 2px solid var(--color-primary);
			outline-offset: 2px;
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

		/* WCAG-standard clip technique: stays in the accessibility tree and
		   readable by AT/voice-control/translation tools, unlike aria-label
		   which strips real DOM text out of those paths. */
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

		.spinner {
			inline-size: 1em;
			block-size: 1em;
			border: 2px solid currentColor;
			border-inline-end-color: transparent;
			border-radius: 50%;
			animation: spin 700ms linear infinite;
		}

		@keyframes spin {
			to {
				transform: rotate(360deg);
			}
		}
	}
</style>
