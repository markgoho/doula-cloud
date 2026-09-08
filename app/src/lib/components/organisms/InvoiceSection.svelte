<script lang="ts">
	/**
	 * The Engagement view's Invoices section (#81): the list of Invoices
	 * billed against the Contract so far, and either an amount input to
	 * create a new one or a Notice naming what is missing (#270). onCreate
	 * owns the actual API call; this component only reports the amount
	 * the Staff member entered and displays whatever error it throws.
	 *
	 * Three preconditions, three separate blocks -- each has its own
	 * fixer, so the reader has to be told which (#270, following #255's
	 * own three-separate-fields rule on this DTO):
	 *
	 * - contractStatus isn't billableContractStatus (#275) -- Staff
	 *   issues a new Contract, or the Client signs the one that exists.
	 * - !clientsCanPay -- a Practice Owner has to connect Stripe. isOwner
	 *   also gets a link to the Payments settings screen rather than the
	 *   old direct "Connect Stripe" button here: PostConnectHandler can
	 *   itself refuse (an unanswered website question, an unpublished
	 *   page), and this component has no way to show that refusal's own
	 *   words.
	 * - !hasClientEmail -- Staff adds an email to the Client's record.
	 *   Moved ahead of the old post-submit discovery, per #255's own
	 *   principle: state it as standing information rather than only as
	 *   feedback after a Send.
	 *
	 * Every one of the three renders Notice's info variant (role="status",
	 * a polite live region), never a disabled form: a Staff member who
	 * just hit one of these, or a screen reader user, hears why billing is
	 * unavailable rather than finding the control silently gone.
	 */
	import {
		billableContractStatus,
		clientHasNoEmailMessage,
		clientsCannotPayMessage,
		formatAmount,
		invoiceStatusLabel,
		unbillableContractMessage,
		type Invoice
	} from '#lib/invoice.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';

	let {
		invoices,
		contractStatus,
		clientsCanPay,
		hasClientEmail,
		isOwner,
		paymentsSettingsHref,
		onCreate
	}: {
		invoices: Invoice[];
		contractStatus: string;
		clientsCanPay: boolean;
		hasClientEmail: boolean;
		isOwner: boolean;
		paymentsSettingsHref: string;
		onCreate: (amountCents: number) => Promise<void>;
	} = $props();

	const isBillable = $derived(contractStatus === billableContractStatus);

	let amountDollars = $state('');
	let isCreating = $state(false);
	let createError = $state('');

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
{:else if !clientsCanPay}
	<Notice variant="info" message={clientsCannotPayMessage} />
	{#if isOwner}
		<Link href={paymentsSettingsHref} label="Go to Payments settings" />
	{/if}
{:else if !hasClientEmail}
	<Notice variant="info" message={clientHasNoEmailMessage} />
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
