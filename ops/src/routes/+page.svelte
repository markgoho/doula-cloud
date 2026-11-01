<script lang="ts">
	import { page } from '$app/state';
	import Button from '$ui/atoms/Button.svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { endSession, SERVICE_PROBLEM, type Operator } from '#lib/session.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/authRefusal.js';

	const operator = $derived(page.data.operator as Operator);
	let errors = $state<FormError[]>([]);
	let isSigningOut = $state(false);


	async function signOut(event: SubmitEvent) {
		event.preventDefault();
		isSigningOut = true;
		try {
			await endSession();
			await goto(resolve('/signed-out'));
		} catch {
			errors = [{ message: SERVICE_PROBLEM }];
		} finally {
			isSigningOut = false;
		}
	}
</script>

<svelte:head><title>DoulaCloud operations</title></svelte:head>

{#if operator}
	<ErrorSummary {errors} />
	<h1>DoulaCloud operations</h1>
	<p>You are signed in as {operator.name} ({operator.email}).</p>
	<nav aria-label="Operations">
		<ul>
			<li><a href={resolve('/practices')}>Practices</a></li>
			<li><a href={resolve('/credits')}>Monthly Credits</a></li>
			<li><a href={resolve('/feedback')}>Feedback</a></li>
			<li><a href={resolve('/staff')}>Reset two-factor authentication</a></li>
			<li><a href={resolve('/pilot-allowlist')}>Pilot allowlist</a></li>
			<li><a href={resolve('/pilot-gate')}>Pilot gate</a></li>
			<li><a href={resolve('/legal-versions')}>Legal versions</a></li>
			<li><a href={resolve('/operator-log')}>Operator log</a></li>
		</ul>
	</nav>
	<form onsubmit={signOut}>
		<Button label="Sign out" type="submit" loading={isSigningOut} />
	</form>
{/if}

<style>
	ul {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		padding-inline-start: var(--space-4);
	}
</style>
