<script lang="ts">
	import { page } from '$app/state';
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import Button from '$ui/atoms/Button.svelte';
	import { tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { ADDRESS_FIELD, addressErrors, sendPilotAddress, type PilotAddressResult } from '#lib/pilotAllowlist.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	// One step at a time, and the answer only once there is one to show.
	type Screen = { step: 'answers' } | { step: 'check' } | { step: 'done'; result: PilotAddressResult };

	let screen = $state<Screen>({ step: 'answers' });
	let address = $state('');

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const act = new CheckAct<PilotAddressResult>({ onsignedout: signedOut });

	const stored = $derived(address.trim().toLowerCase());
	const errorFor = (field: string) => act.errors.find((error) => error.targetId === field)?.message;

	async function show(next: Screen, focus?: string) {
		screen = next;
		await tick();
		if (focus) document.querySelector<HTMLElement>(`#${focus}`)?.focus();
		else document.querySelector('h1')?.focus();
	}

	async function continueToCheck(event: SubmitEvent) {
		event.preventDefault();
		const errors = addressErrors(address);
		if (errors.length > 0) return act.refuse(errors);
		act.renew();
		await show({ step: 'check' });
	}

	async function add(event: SubmitEvent) {
		event.preventDefault();
		const result = await act.run((key) => sendPilotAddress(address, key));
		if (result) return show({ step: 'done', result });
		if (act.errors.some((error) => error.targetId === ADDRESS_FIELD)) await show({ step: 'answers' }, ADDRESS_FIELD);
	}

	async function addAnother(event: MouseEvent) {
		event.preventDefault();
		address = '';
		act.renew();
		await show({ step: 'answers' }, ADDRESS_FIELD);
	}
</script>

<svelte:head><title>Pilot allowlist – DoulaCloud operations</title></svelte:head>

<ErrorSummary errors={act.errors} />

{#if screen.step === 'done'}
	<stack-l role="status">
		<h1 tabindex="-1">Address added</h1>
		<p>
			{screen.result.address} is on the pilot allowlist. A doula who signs up with this address can now create a Practice.
			The Operator log shows the add under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/pilot-allowlist')} onclick={addAnother}>Add another address</a></p>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if screen.step === 'check'}
	<h1 tabindex="-1">Check the address before you add it</h1>
	<SummaryList rows={[{ label: 'Email address', value: stored, changes: 'email address', onchange: () => show({ step: 'answers' }, ADDRESS_FIELD) }]}>
		{#snippet action(row, describedBy)}
			<Button variant="link" label="Change" {describedBy} onClick={row.onchange} />
		{/snippet}
	</SummaryList>
	<form novalidate onsubmit={add}>
		<stack-l>
			<p>
				A doula who signs up with this address can create a Practice before January. An address cannot be taken off
				the list once it is added.
			</p>
			<Button label="Add the address" type="submit" loading={act.isSending} />
		</stack-l>
	</form>
{:else if page.data.operator}
	<h1 tabindex="-1">Add an address to the pilot allowlist</h1>
	<form novalidate onsubmit={continueToCheck}>
		<stack-l>
			<LabeledField id={ADDRESS_FIELD} label="Email address" hint="The address the doula will sign up with. Add it once you have spoken with the doula." error={errorFor(ADDRESS_FIELD)}>
				{#snippet children({ id, describedBy, invalid })}
					<TextInput
						{id}
						{describedBy}
						{invalid}
						type="email"
						autocomplete="off"
						value={address}
						onInput={(next) => (address = next)}
					/>
				{/snippet}
			</LabeledField>
			<Button label="Continue" type="submit" />
		</stack-l>
	</form>
{/if}

<style>
	h1:focus {
		outline: none;
	}
</style>
