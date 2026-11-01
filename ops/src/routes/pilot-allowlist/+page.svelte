<script lang="ts">
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import type { FormError } from '#lib/authRefusal.js';
	import { readSession } from '#lib/session.js';
	import { ADDRESS_FIELD, addressErrors, sendPilotAddress, type PilotAddressResult } from '#lib/pilotAllowlist.js';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	let isSignedIn = $state(false);
	let step = $state<'answers' | 'check' | 'done'>('answers');
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);
	let address = $state('');
	let result = $state<PilotAddressResult | undefined>();
	let requestKey = '';

	const stored = $derived(address.trim().toLowerCase());
	const errorFor = (field: string) => errors.find((error) => error.targetId === field)?.message;

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	onMount(async () => {
		if (!(await readSession())) return signedOut();
		isSignedIn = true;
	});

	async function show(next: typeof step, focus?: string) {
		step = next;
		await tick();
		if (focus) document.querySelector<HTMLElement>(`#${focus}`)?.focus();
		else document.querySelector('h1')?.focus();
	}

	async function continueToCheck(event: SubmitEvent) {
		event.preventDefault();
		errors = addressErrors(address);
		if (errors.length > 0) return;
		requestKey = crypto.randomUUID();
		await show('check');
	}

	async function add(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			const outcome = await sendPilotAddress(address, requestKey);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'refused') {
				errors = outcome.errors;
				if (errors.some((error) => error.targetId === ADDRESS_FIELD)) await show('answers', ADDRESS_FIELD);
				return;
			}
			if (outcome.kind === 'ok') {
				errors = [];
				result = outcome.value;
				await show('done');
			}
		} finally {
			isSending = false;
		}
	}

	async function addAnother(event: MouseEvent) {
		event.preventDefault();
		address = '';
		result = undefined;
		errors = [];
		await show('answers', ADDRESS_FIELD);
	}
</script>

<svelte:head><title>Pilot allowlist – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if step === 'done' && result}
	<stack-l role="status">
		<h1 tabindex="-1">Address added</h1>
		<p>
			{result.address} is on the pilot allowlist. A doula who signs up with this address can now create a Practice. The
			Operator log shows the add under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/pilot-allowlist')} onclick={addAnother}>Add another address</a></p>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if step === 'check'}
	<h1 tabindex="-1">Check the address before you add it</h1>
	<SummaryList rows={[{ label: 'Email address', value: stored, changes: 'email address', onchange: () => show('answers', ADDRESS_FIELD) }]}>
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
			<Button label="Add the address" type="submit" loading={isSending} />
		</stack-l>
	</form>
{:else if isSignedIn}
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
