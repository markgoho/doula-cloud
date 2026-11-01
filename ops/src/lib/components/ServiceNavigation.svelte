<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import { resolve } from '$app/paths';
	import type { FormError } from '../authRefusal.ts';
	import { endSession, SERVICE_PROBLEM, type Operator } from '../session.ts';

	let {
		operator,
		pathname,
		onsignedout,
		signOut = endSession
	}: {
		operator: Operator | undefined;
		pathname: string;
		onsignedout: () => void | Promise<void>;
		signOut?: () => Promise<void>;
	} = $props();

	const sections = [
		{ label: 'Practices', href: resolve('/practices') },
		{ label: 'Monthly Credits', href: resolve('/credits') },
		{ label: 'Feedback', href: resolve('/feedback') },
		{ label: 'Reset two-factor authentication', href: resolve('/staff') },
		{ label: 'Pilot allowlist', href: resolve('/pilot-allowlist') },
		{ label: 'Pilot gate', href: resolve('/pilot-gate') },
		{ label: 'Legal versions', href: resolve('/legal-versions') },
		{ label: 'Operator log', href: resolve('/operator-log') }
	];

	let errors = $state<FormError[]>([]);
	let isSigningOut = $state(false);

	function isCurrent(href: string): boolean {
		return pathname === href || pathname.startsWith(`${href}/`);
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		isSigningOut = true;
		try {
			await signOut();
			errors = [];
			await onsignedout();
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSigningOut = false;
		}
	}
</script>

<header>
	{#if operator}
		<a class="service" href={resolve('/')}>DoulaCloud operations</a>
	{:else}
		<p class="service">DoulaCloud operations</p>
	{/if}
</header>
{#if operator}
	<div class="bar">
		<nav aria-label="Operations">
			<ul role="list">
				{#each sections as section (section.href)}
					<li>
						<a href={section.href} aria-current={isCurrent(section.href) ? 'page' : undefined}>{section.label}</a>
					</li>
				{/each}
			</ul>
		</nav>
		<form onsubmit={submit}>
			<Button label="Sign out" type="submit" variant="secondary" size="sm" loading={isSigningOut} />
		</form>
	</div>
	{#if errors.length > 0}
		<div class="errors"><ErrorSummary {errors} /></div>
	{/if}
{/if}

<style>
	header {
		padding: var(--space-3) var(--space-4);
		color: var(--color-on-primary);
		background-color: var(--color-primary);
	}

	.service {
		margin: 0;
		font-weight: bold;
		color: var(--color-on-primary);
	}

	a.service:hover {
		color: var(--color-on-primary);
	}

	.bar {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2) var(--space-4);
		padding: var(--space-2) var(--space-4);
		background-color: var(--color-surface-container);
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	nav {
		flex: 1 1 auto;
	}

	ul {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-4);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	nav a {
		display: inline-block;
		padding-block: var(--space-2);
		border-block-end: var(--border-active) solid transparent;
	}

	nav a[aria-current='page'] {
		border-block-end-color: currentcolor;
		font-weight: bold;
	}

	.errors {
		padding: var(--space-4);
	}

	a:focus-visible {
		outline: var(--focus-ring-width) solid var(--color-primary);
		outline-offset: var(--focus-ring-offset);
	}

	a.service:focus-visible {
		outline-color: var(--color-on-primary);
	}
</style>
