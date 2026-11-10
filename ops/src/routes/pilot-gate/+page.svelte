<script lang="ts">
	import { page } from '$app/state';
	import { newYorkDay } from '#lib/dates.js';
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { readPilotGate, sendPilotGateOpening, type PilotGate } from '#lib/pilotGate.js';
	import { type Operator } from '#lib/session.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';

	const operator = $derived(page.data.operator as Operator);

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const read = new CheckRead<PilotGate>(signedOut);
	const act = new CheckAct<PilotGate>({ onsignedout: signedOut, stepUp: { email: () => operator.email, freshIdToken } });
	const errors = $derived(read.state.status === 'failed' ? read.errors : act.errors);

	onMount(() => read.load(() => readPilotGate()));

	async function open(event: SubmitEvent) {
		event.preventDefault();
		if (!(await act.run((key) => sendPilotGateOpening(key)))) return;
		await tick();
		document.querySelector('h1')?.focus();
	}
</script>

<svelte:head><title>Pilot gate – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if act.state.status === 'done'}
	<stack-l role="status">
		<h1 tabindex="-1">Pilot gate open</h1>
		<p>Anyone who signs up can now create a Practice. The Operator log shows the opening under your name.</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'failed'}
	<h1 tabindex="-1">Pilot gate</h1>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'loaded'}
	{@const gate = read.state.value}
	{#if gate.openedAt}
		<stack-l>
			<h1 tabindex="-1">The pilot gate is open</h1>
			<p>
				It was opened on {newYorkDay(gate.openedAt)}. Anyone who signs up can create a Practice, and the gate cannot be
				closed.
			</p>
		</stack-l>
		<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
	{:else}
		<h1 tabindex="-1">Check before you open the pilot gate</h1>
		<stack-l>
			<p>
				Anyone who signs up can create a Practice, not only an address on the pilot allowlist. Do this when DoulaCloud
				opens to everyone in January.
			</p>
			<p>The gate stays open for good. Nothing closes it again.</p>
		</stack-l>
		<form novalidate onsubmit={open}>
			<stack-l>
				<StepUpFields {errors} bind:password={act.password} bind:code={act.code} />
				<Button label="Open the pilot gate" type="submit" loading={act.isSending} />
			</stack-l>
		</form>
	{/if}
{/if}

<style>
	h1:focus {
		outline: none;
	}
</style>
