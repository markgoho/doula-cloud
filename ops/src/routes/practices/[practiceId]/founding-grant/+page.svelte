<script lang="ts">
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import {
		credits,
		readFoundingGrantCheck,
		sendFoundingGrant,
		type FoundingGrantCheck,
		type FoundingGrantResult
	} from '#lib/foundingGrant.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	const practiceId = $derived(page.params.practiceId ?? '');

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const read = new CheckRead<FoundingGrantCheck>(signedOut);
	const act = new CheckAct<FoundingGrantResult>({ onsignedout: signedOut });
	const errors = $derived(read.state.status === 'failed' ? read.errors : act.errors);

	onMount(() => read.load(() => readFoundingGrantCheck(practiceId)));

	async function give(event: SubmitEvent) {
		event.preventDefault();
		if (!(await act.run((key) => sendFoundingGrant(practiceId, key)))) return;
		await tick();
		document.querySelector('h1')?.focus();
	}
</script>

<svelte:head><title>Founding grant – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if act.state.status === 'done'}
	{@const result = act.state.value}
	<stack-l role="status">
		<h1 tabindex="-1">Founding grant given</h1>
		<p>
			{credits(result.credits)} given to {result.practiceName}, 3 for each of the {result.staffCount} Staff. The Practice's
			Activity shows the grant under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'failed'}
	<h1 tabindex="-1">Founding grant</h1>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'loaded'}
	{@const check = read.state.value}
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
				{ label: 'Practice', value: check.practiceName },
				{ label: 'Staff', value: String(check.staffCount) },
				{ label: 'Credits', value: `${credits(check.credits)}, 3 for each Staff member` }
			]}
		/>
		<form novalidate onsubmit={give}>
			<stack-l>
				<p>
					The Credits go on the Practice's ledger at once, and are spent before any it buys. A founding grant is given
					once and cannot be undone.
				</p>
				<Button label="Give {credits(check.credits)}" type="submit" loading={act.isSending} />
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
