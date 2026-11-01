<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { endSession, readSession, SERVICE_PROBLEM, type Operator } from '#lib/session.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import type { FormError } from '#lib/authRefusal.js';

	let operator = $state<Operator | undefined>();
	let errors = $state<FormError[]>([]);
	let isSigningOut = $state(false);

	onMount(async () => {
		operator = await readSession();
		if (!operator) await goto(resolve('/sign-in'));
	});

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
	<p><a href={resolve('/practices')}>Practices</a></p>
	<p><a href={resolve('/credits')}>Monthly Credits</a></p>
	<p><a href={resolve('/feedback')}>Feedback</a></p>
	<p><a href={resolve('/pilot-allowlist')}>Pilot allowlist</a></p>
	<p><a href={resolve('/pilot-gate')}>Pilot gate</a></p>
	<p><a href={resolve('/legal-versions')}>Legal versions</a></p>
	<p><a href={resolve('/operator-log')}>Operator log</a></p>
	<form onsubmit={signOut}>
		<Button label="Sign out" type="submit" loading={isSigningOut} />
	</form>
{/if}
