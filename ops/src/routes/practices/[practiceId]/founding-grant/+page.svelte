<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { readSession } from '#lib/session.js';
	import {
		credits,
		readFoundingGrantCheck,
		sendFoundingGrant,
		type FoundingGrantCheck,
		type FoundingGrantResult
	} from '#lib/foundingGrant.js';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
	import SummaryList from '#lib/components/SummaryList.svelte';

	const practiceId = $derived(page.params.practiceId ?? '');

	let check = $state<FoundingGrantCheck | undefined>();
	let result = $state<FoundingGrantResult | undefined>();
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);
	const requestKey = crypto.randomUUID();

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	onMount(async () => {
		if (!(await readSession())) return signedOut();
		const outcome = await readFoundingGrantCheck(practiceId);
		if (outcome.kind === 'signed-out') return signedOut();
		if (outcome.kind === 'ok') check = outcome.value;
		else if (outcome.kind === 'refused') errors = outcome.errors;
	});

	async function give(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			const outcome = await sendFoundingGrant(practiceId, requestKey);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			if (outcome.kind === 'ok') {
				errors = [];
				result = outcome.value;
				await tick();
				document.querySelector('h1')?.focus();
			}
		} finally {
			isSending = false;
		}
	}
</script>

<svelte:head><title>Founding grant – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if result}
	<stack-l role="status">
		<h1 tabindex="-1">Founding grant given</h1>
		<p>
			{credits(result.credits)} given to {result.practiceName}, 3 for each of the {result.staffCount} Staff. The Practice's
			Activity shows the grant under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if check}
	<p class="caption">{check.practiceName}</p>
	{#if check.grantedCredits !== null}
		<h1 tabindex="-1">Founding grant</h1>
		<p>
			This Practice already holds its founding grant of {credits(check.grantedCredits)}. A founding grant is given once.
		</p>
	{:else if check.staffCount === 0}
		<h1 tabindex="-1">Founding grant</h1>
		<p>This Practice has no Staff, so there is nothing to size a founding grant from.</p>
	{:else}
		<h1 tabindex="-1">Check the founding grant before you give it</h1>
		<SummaryList
			rows={[
				{ key: 'Practice', value: check.practiceName },
				{ key: 'Staff', value: String(check.staffCount) },
				{ key: 'Credits', value: `${credits(check.credits)}, 3 for each Staff member` }
			]}
		/>
		<form novalidate onsubmit={give}>
			<stack-l>
				<p>
					The Credits go on the Practice's ledger at once, and are spent before any it buys. A founding grant is given
					once and cannot be undone.
				</p>
				<button class="ops-button" type="submit" disabled={isSending}>Give {credits(check.credits)}</button>
			</stack-l>
		</form>
	{/if}
{/if}

<style>
	.caption {
		color: var(--color-on-surface-variant);
	}

	h1:focus {
		outline: none;
	}
</style>
