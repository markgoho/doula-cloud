<script lang="ts">
	/**
	 * The Engagement view's Invoices section (#81): the list of Invoices
	 * billed against the Contract so far, and either an amount input to
	 * create a new one, or -- once a creation attempt discovers the
	 * Practice hasn't connected Stripe yet -- the #79 connect-gate state
	 * instead of the form (an Owner gets a Connect Stripe button, a
	 * non-Owner gets a static message), per #78's lazy-connect-prompt rule.
	 * onCreate/onConnect own the actual API calls and any resulting state
	 * change (e.g. flipping connectGate); this component only reports the
	 * amount the Staff member entered and displays whatever error either
	 * callback throws.
	 *
	 * #275: contractStatus gates all of the above. A Contract that isn't
	 * billableContractStatus (Signed) never reaches the connect gate or the
	 * form at all -- Create Invoice is not offered, and Notice's info
	 * variant (role="status", a polite live region) says why, so a Staff
	 * member who just voided the Contract, or a screen reader user, hears
	 * that billing is deliberately unavailable rather than finding the
	 * control silently gone.
	 */
	import {
		billableContractStatus,
		formatAmount,
		invoiceStatusLabel,
		unbillableContractMessage,
		type Invoice
	} from '#lib/invoice.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let {
		invoices,
		contractStatus,
		connectGate,
		onCreate,
		onConnect
	}: {
		invoices: Invoice[];
		contractStatus: string;
		connectGate?: { isOwner: boolean };
		onCreate: (amountCents: number) => Promise<void>;
		onConnect: () => Promise<void>;
	} = $props();

	const isBillable = $derived(contractStatus === billableContractStatus);

	let amountDollars = $state('');
	let isCreating = $state(false);
	let createError = $state('');
	let isConnecting = $state(false);
	let connectError = $state('');

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';

		const dollars = Number(amountDollars);
		if (!Number.isFinite(dollars) || dollars <= 0) {
			createError = 'Enter an amount greater than zero';
			return;
		}

		isCreating = true;
		try {
			await onCreate(Math.round(dollars * 100));
			amountDollars = '';
		} catch (error_) {
			createError = error_ instanceof Error ? error_.message : 'Failed to create invoice';
		} finally {
			isCreating = false;
		}
	}

	async function handleConnect() {
		connectError = '';
		isConnecting = true;
		try {
			await onConnect();
		} catch (error_) {
			connectError = error_ instanceof Error ? error_.message : 'Failed to start Stripe Connect onboarding';
		} finally {
			isConnecting = false;
		}
	}
</script>

{#if invoices.length === 0}
	<p>No Invoices yet.</p>
{:else}
	<ul>
		{#each invoices as invoice (invoice.id)}
			<li>
				<!-- v8 ignore start: Svelte's compiled null-guard on these text nodes is unreachable -- formatAmount and invoiceStatusLabel always return a string -->
				{formatAmount(invoice.amountCents)} — {invoiceStatusLabel(invoice.status)}
				<!-- v8 ignore stop -->
				{#if invoice.paidAt}
					<!-- v8 ignore start: Svelte's compiled null-guard on this text node is unreachable -- toLocaleDateString always returns a string -->
					(paid {new Date(invoice.paidAt).toLocaleDateString()})
					<!-- v8 ignore stop -->
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if !isBillable}
	<Notice variant="info" message={unbillableContractMessage(contractStatus)} />
{:else if connectGate?.isOwner}
	<p>Connect Stripe to create an Invoice.</p>
	<Button label="Connect Stripe" onClick={handleConnect} loading={isConnecting} />
	{#if connectError}
		<p role="alert">{connectError}</p>
	{/if}
{:else if connectGate}
	<p>Ask a Practice Owner to connect Stripe.</p>
{:else}
	<form onsubmit={handleCreate}>
		<LabeledField label="Amount (USD)">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="number"
					step={0.01}
					value={amountDollars}
					onInput={(value) => (amountDollars = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<Button label="Create Invoice" type="submit" loading={isCreating} />
	</form>
	{#if createError}
		<p role="alert">{createError}</p>
	{/if}
{/if}
