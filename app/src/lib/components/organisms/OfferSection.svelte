<script lang="ts">
	/**
	 * The Engagement view's Offers section (#317): who has been offered
	 * this work and what each of them said, plus the form that makes a new
	 * Offer -- to a Doula who is already at the Practice, or to an email
	 * address, which invites her and puts the job in front of her at once.
	 *
	 * The four decidable facts are typed here, not derived: an Offer is a
	 * copy taken at send time, and the Client's first initial is pre-filled
	 * from the Engagement only as a convenience -- what is sent is what the
	 * sender saw.
	 *
	 * onCreate and onWithdraw own the API calls and the resulting state
	 * change; this component reports what was typed and shows what either
	 * callback throws.
	 */
	import { untrack } from 'svelte';
	import { formatFee, isOpen, offerStateLabels, offerStateVariants, type NewOffer, type Offer } from '#lib/offer.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup from '#lib/components/molecules/RadioGroup.svelte';

	let {
		offers,
		doulas,
		clientFirstInitial = '',
		onCreate,
		onWithdraw
	}: {
		offers: Offer[];
		/** The Practice's Doula memberships, for the "someone already here"
		 * target. employmentType decides whether a fee is required. */
		doulas: { staffId: string; name: string; employmentType: string }[];
		clientFirstInitial?: string;
		onCreate: (offer: NewOffer) => Promise<void>;
		onWithdraw: (offerId: string) => Promise<void>;
	} = $props();

	let target = $state<'staff' | 'email'>('staff');
	let staffId = $state('');
	let email = $state('');
	let feeDollars = $state('');
	let terms = $state('');
	// Pre-filled from the Client's name once, then hers to change -- hence
	// untrack: the row holds what was actually sent, so this is a
	// convenience at first render, not a binding to the Engagement.
	let initial = $state(untrack(() => clientFirstInitial).slice(0, 1));
	let clientArea = $state('');
	let dueDate = $state('');
	let isSending = $state(false);
	let createError = $state('');
	let withdrawError = $state('');

	// Which employment type the fee rule is read against: her own
	// Membership for a Doula already here, and always contractor for an
	// email address, since that is what the Invitation joins her as.
	const selectedType = $derived(
		target === 'email' ? 'contractor' : (doulas.find((d) => d.staffId === staffId)?.employmentType ?? '')
	);
	const isFeeRequired = $derived(selectedType === 'contractor');

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		createError = '';

		const offer: NewOffer = {
			clientFirstInitial: initial,
			clientArea,
			dueDate,
			terms: terms || undefined
		};
		if (target === 'email') {
			offer.email = email;
		} else {
			offer.staffId = staffId;
		}
		if (isFeeRequired) {
			const dollars = Number(feeDollars);
			if (!Number.isFinite(dollars) || dollars <= 0) {
				createError = 'Enter a fee greater than zero';
				return;
			}
			offer.amountCents = Math.round(dollars * 100);
		}

		isSending = true;
		try {
			await onCreate(offer);
			email = '';
			feeDollars = '';
			terms = '';
			clientArea = '';
			dueDate = '';
		} catch (error_) {
			createError = error_ instanceof Error ? error_.message : 'Failed to send offer';
		} finally {
			isSending = false;
		}
	}

	async function handleWithdraw(offerId: string) {
		withdrawError = '';
		try {
			await onWithdraw(offerId);
		} catch (error_) {
			withdrawError = error_ instanceof Error ? error_.message : 'Failed to withdraw offer';
		}
	}
</script>

{#if offers.length === 0}
	<p>Nobody has been offered this work yet.</p>
{:else}
	<ul>
		{#each offers as offer (offer.offerId)}
			<li>
				<span>{offer.targetName || offer.targetAddress}</span>
				<Badge label={offerStateLabels[offer.state]} variant={offerStateVariants[offer.state]} />
				<span>{formatFee(offer.amountCents)}</span>
				{#if isOpen(offer)}
					<Button label="Withdraw" variant="secondary" size="sm" onClick={() => handleWithdraw(offer.offerId)} />
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if withdrawError}
	<Notice message={withdrawError} variant="error" />
{/if}

<form onsubmit={handleCreate}>
	<RadioGroup
		legend="Offer this work to"
		options={[
			{ value: 'staff', label: 'Someone already at this practice' },
			{ value: 'email', label: 'Someone new, by email' }
		]}
		value={target}
		onChange={(value) => (target = value)}
	/>

	{#if target === 'staff'}
		<RadioGroup
			legend="Doula"
			options={doulas.map((doula) => ({ value: doula.staffId, label: doula.name }))}
			value={staffId}
			onChange={(value) => (staffId = value)}
		/>
	{:else}
		<LabeledField label="Email address">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="email"
					value={email}
					onInput={(value) => (email = value)}
					required
				/>
			{/snippet}
		</LabeledField>
		<p>She joins the practice as a contractor doula, so this offer carries a fee.</p>
	{/if}

	{#if isFeeRequired}
		<LabeledField label="Fee (USD)">
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					type="number"
					step={0.01}
					value={feeDollars}
					onInput={(value) => (feeDollars = value)}
					required
				/>
			{/snippet}
		</LabeledField>
	{/if}

	<LabeledField label="Client's first initial">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				maxlength={1}
				value={initial}
				onInput={(value) => (initial = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="General area">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				value={clientArea}
				onInput={(value) => (clientArea = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField label="Due date">
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="date" value={dueDate} onInput={(value) => (dueDate = value)} required />
		{/snippet}
	</LabeledField>
	<LabeledField label="Terms">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={terms} onInput={(next) => (terms = next)} />
		{/snippet}
	</LabeledField>

	<Button label="Send Offer" type="submit" loading={isSending} />
</form>

{#if createError}
	<Notice message={createError} variant="error" />
{/if}
