<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { endSession, readSession, SERVICE_PROBLEM, type Operator } from '#lib/session.js';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
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
	<form onsubmit={signOut}>
		<button class="ops-button" type="submit" disabled={isSigningOut}>Sign out</button>
	</form>
{/if}
