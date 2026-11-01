<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { FormError } from '#lib/authRefusal.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { openedOn, readPilotGate, sendPilotGateOpening, type PilotGate } from '#lib/pilotGate.js';
	import { readSession, type Operator } from '#lib/session.js';
	import { stepUp } from '#lib/stepUp.js';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';

	let operator = $state<Operator | undefined>();
	let gate = $state<PilotGate | undefined>();
	let result = $state<PilotGate | undefined>();
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);
	let password = $state('');
	let code = $state('');
	const requestKey = crypto.randomUUID();

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	onMount(async () => {
		operator = await readSession();
		if (!operator) return signedOut();
		const outcome = await readPilotGate();
		if (outcome.kind === 'signed-out') return signedOut();
		if (outcome.kind === 'ok') gate = outcome.value;
		else if (outcome.kind === 'refused') errors = outcome.errors;
	});

	async function open(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			errors = await stepUp({ email: operator!.email, password, code }, { freshIdToken });
			if (errors.length > 0) return;
			const outcome = await sendPilotGateOpening(requestKey);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'step-up') {
				errors = [{ message: 'Enter the code your authenticator app shows now to do this.', field: 'step-up-code' }];
				return;
			}
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				return;
			}
			result = outcome.value;
			await tick();
			document.querySelector('h1')?.focus();
		} finally {
			isSending = false;
		}
	}
</script>

<svelte:head><title>Pilot gate – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if result}
	<stack-l role="status">
		<h1 tabindex="-1">Pilot gate open</h1>
		<p>Anyone who signs up can now create a Practice. The Operator log shows the opening under your name.</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if gate?.openedAt}
	<stack-l>
		<h1 tabindex="-1">The pilot gate is open</h1>
		<p>It was opened on {openedOn(gate.openedAt)}. Anyone who signs up can create a Practice, and the gate cannot be closed.</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if gate}
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
			<StepUpFields {errors} bind:password bind:code />
			<button class="ops-button" type="submit" disabled={isSending}>Open the pilot gate</button>
		</stack-l>
	</form>
{/if}

<style>
	h1:focus {
		outline: none;
	}
</style>
