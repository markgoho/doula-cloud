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
	import { emailFormatError, type FormError } from '#lib/formErrors.js';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Textarea from '#lib/components/atoms/Textarea.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import RadioGroup, { radioFieldId } from '#lib/components/molecules/RadioGroup.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';

	let {
		offers,
		doulas,
		clientName = '',
		onCreate,
		onWithdraw
	}: {
		offers: Offer[];
		/** The Doulas this Offer can go to, for the "someone already here"
		 * target. employmentType decides whether a fee is required. An
		 * empty list is a fact -- nobody is there to pick -- and `undefined`
		 * is the roster that could not be read, which is not the same
		 * thing and is not said in the same words (#1432). */
		doulas: { staffId: string; name: string; employmentType: string }[] | undefined;
		/** The Client's full name, from which the Offer's first-initial
		 * field is seeded. Not an initial: an Offer carries only an
		 * initial (ADR-0017), but the seed for it is the name the
		 * Engagement already holds, and a prop asking for an initial that
		 * is handed a whole name reads as a bug. */
		clientName?: string;
		onCreate: (offer: NewOffer) => Promise<void>;
		onWithdraw: (offerId: string) => Promise<void>;
	} = $props();

	// What was last chosen, not what the form acts on: `answer` below is,
	// because an empty roster leaves "email" as the only answer whatever
	// this holds (#1432). Nothing is chosen when the form opens (#1914,
	// ADR-0055): which follow-up question is asked first is the sender's
	// to decide, not the form's.
	let target = $state<'staff' | 'email' | undefined>();
	let staffId = $state('');
	let email = $state('');
	let feeDollars = $state('');
	let terms = $state('');
	// Pre-filled from the Client's name once, then hers to change -- hence
	// untrack: the row holds what was actually sent, so this is a
	// convenience at first render, not a binding to the Engagement.
	let initial = $state(untrack(() => clientName).slice(0, 1));
	let clientArea = $state('');
	let dueDate = $state('');
	let withdrawError = $state('');

	// #1228: five of this form's controls carried `required`, the only
	// refusal it had -- StackedForm's `novalidate` (ADR-0021) takes that
	// away, so every one of them (plus the "Doula" radio pick, which
	// carried no browser check of its own to lose) is checked here first.
	const targetGroupName = 'offer-target';
	const doulaGroupName = 'offer-doula';
	const emailFieldId = 'offer-email';
	const feeFieldId = 'offer-fee';
	const initialFieldId = 'offer-initial';
	const areaFieldId = 'offer-area';
	const dueDateFieldId = 'offer-due-date';

	// #1432: "Someone already at this practice" is a question only while
	// there is somebody to pick. With nobody -- a Practice with no Doula,
	// or (since #1598 took the sender out of the list) a solo Owner whose
	// only Doula is the person at the screen -- the form has one answer
	// left, so it does not ask: GOV.UK's radios are for a choice, and an
	// option with nothing behind it is a refusal waiting for a submit.
	const roster = $derived(doulas ?? []);
	const firstDoula = $derived(roster[0]);

	// What the form acts on: "email" whenever there is nobody to pick, and
	// otherwise the sender's own answer, which is `undefined` until she
	// gives one. Unanswered, neither follow-up question is drawn.
	const answer = $derived(firstDoula === undefined ? 'email' : target);

	// "Offer this work to"'s refusal points at its first option, GOV.UK's
	// rule for a group. Only reached while the question is on the page.
	const targetFieldId = radioFieldId(targetGroupName, 'staff');

	// The Doula question's own target for a refusal -- its first option's
	// id, the string RadioGroup itself builds -- and `undefined` whenever
	// that question is not on the page: nobody to pick, or "someone new"
	// chosen, or nothing chosen yet. One value answers both "is the
	// question asked" and "where does its refusal point", so a refusal
	// cannot be given an id that no element carries, which is what an id
	// built from an empty roster was.
	const doulaFieldId = $derived(
		firstDoula !== undefined && answer === 'staff' ? radioFieldId(doulaGroupName, firstDoula.staffId) : undefined
	);
	const isByEmail = $derived(answer === 'email');

	// Which employment type the fee rule is read against: her own
	// Membership for a Doula already here, and always contractor for an
	// email address, since that is what the Invitation joins her as. With
	// no answer yet there is no type, so no fee is asked.
	const selectedType = $derived(
		isByEmail ? 'contractor' : (roster.find((d) => d.staffId === staffId)?.employmentType ?? '')
	);
	const isFeeRequired = $derived(selectedType === 'contractor');

	const offerSubmission = new FormSubmission();

	async function handleCreate(event: SubmitEvent) {
		event.preventDefault();
		await offerSubmission.run(async () => {
			const errors: FormError[] = [];

			if (answer === undefined) {
				errors.push({ message: 'Select who to offer this work to', targetId: targetFieldId });
			} else if (doulaFieldId !== undefined) {
				if (!staffId) errors.push({ message: 'Select a Doula', targetId: doulaFieldId });
			} else if (email.trim() === '') {
				errors.push({ message: 'Enter an email address', targetId: emailFieldId });
			} else {
				const formatError = emailFormatError(email.trim());
				if (formatError) errors.push({ message: formatError, targetId: emailFieldId });
			}

			// Checked here, not after the three facts below: the summary
			// lists its entries in the order the fields are on the page
			// (GOV.UK's error summary), and the fee is the second field.
			let amountCents: number | undefined;
			if (isFeeRequired) {
				if (feeDollars.trim() === '') {
					errors.push({ message: 'Enter a fee', targetId: feeFieldId });
				} else {
					const dollars = Number(feeDollars);
					if (!Number.isFinite(dollars) || dollars <= 0) {
						errors.push({ message: 'Enter a fee greater than zero', targetId: feeFieldId });
					} else {
						amountCents = Math.round(dollars * 100);
					}
				}
			}

			if (initial.trim() === '') {
				errors.push({ message: "Enter the Client's first initial", targetId: initialFieldId });
			}
			if (clientArea.trim() === '') {
				errors.push({ message: 'Enter the general area', targetId: areaFieldId });
			}
			if (dueDate === '') {
				errors.push({ message: 'Enter the due date', targetId: dueDateFieldId });
			}

			if (errors.length > 0) return errors;

			const offer: NewOffer = {
				clientFirstInitial: initial,
				clientArea,
				dueDate,
				terms: terms || undefined
			};
			if (isByEmail) {
				offer.email = email;
			} else {
				offer.staffId = staffId;
			}
			if (amountCents !== undefined) offer.amountCents = amountCents;

			await onCreate(offer);
			// Blank again for the next Offer, with nothing chosen: not the
			// last answer, and not a default (#1914).
			target = undefined;
			staffId = '';
			email = '';
			feeDollars = '';
			terms = '';
			clientArea = '';
			dueDate = '';
		}, orThrownMessage);
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
	<EmptyState message="Nobody has been offered this work yet." />
{:else}
	<ul>
		{#each offers as offer (offer.offerId)}
			<li>
				<span>{offer.targetName || offer.targetAddress}</span>
				<Badge label={offerStateLabels[offer.state]} variant={offerStateVariants[offer.state]} />
				<span>{formatFee(offer.amountCents)}</span>
				{#if isOpen(offer)}
					<!-- v8 ignore start: Svelte-compiled attribute-diffing branch for
					     the dynamic id/describedBy string below isn't reachable from
					     app-level interaction tests (no test changes an Offer's own id
					     mid-test), only from Svelte's own reactivity internals. -->
					<Button
						label="Withdraw"
						variant="secondary"
						size="sm"
						describedBy="offer-{offer.offerId}-withdraw-name"
						onClick={() => handleWithdraw(offer.offerId)}
					/>
					<span class="visually-hidden" id="offer-{offer.offerId}-withdraw-name"
						>{offer.targetName || offer.targetAddress}</span
					>
					<!-- v8 ignore stop -->
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if withdrawError}
	<Notice message={withdrawError} variant="error" />
{/if}

<StackedForm onSubmit={handleCreate}>
	{#if offerSubmission.errors.length > 0}
		<ErrorSummary errors={offerSubmission.errors} />
	{/if}
	{#if doulas === undefined}
		<p>
			We could not load who is at this practice, so this work can only be offered to someone new, by email. To
			pick someone already here, reload the page.
		</p>
	{:else if firstDoula === undefined}
		<!-- True for a Practice with no Doula, and for a reader who is its
		     only Doula, whom the Engagement page leaves out (#1598): the words
		     say nothing about whether the person at the screen is a Doula. -->
		<p>There is no one at this practice to offer this work to, so it can only be offered to someone new, by email.</p>
	{:else}
		<RadioGroup
			legend="Offer this work to"
			name={targetGroupName}
			options={[
				{ value: 'staff', label: 'Someone already at this practice' },
				{ value: 'email', label: 'Someone new, by email' }
			]}
			value={target}
			onChange={(value) => (target = value)}
			error={offerSubmission.errorFor(targetFieldId)}
		/>
	{/if}

	<!-- With nothing chosen yet, neither follow-up question is asked. -->
	{#if doulaFieldId !== undefined}
		<RadioGroup
			legend="Doula"
			name={doulaGroupName}
			options={roster.map((doula) => ({ value: doula.staffId, label: doula.name }))}
			value={staffId}
			onChange={(value) => (staffId = value)}
			error={offerSubmission.errorFor(doulaFieldId)}
		/>
	{:else if isByEmail}
		<LabeledField id={emailFieldId} label="Email address" error={offerSubmission.errorFor(emailFieldId)}>
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
		<p>A doula invited by email joins the practice as a contractor, so this offer carries a fee.</p>
	{/if}

	{#if isFeeRequired}
		<!--
			Widths (#1216): a fee to the cent ("2500.00") is seven characters,
			so GOV.UK's 10-character step, after a "$" that the label's "(USD)"
			says again for a screen reader; one letter at the smallest step; an
			area is a town's name, so GOV.UK's 20; the due date sizes to its
			own control.
		-->
		<LabeledField id={feeFieldId} label="Fee (USD)" error={offerSubmission.errorFor(feeFieldId)} width={10}>
			{#snippet children({ id, describedBy, invalid, width })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					{width}
					prefix="$"
					type="number"
					step={0.01}
					value={feeDollars}
					onInput={(value) => (feeDollars = value)}
					required
				/>
			{/snippet}
		</LabeledField>
	{/if}

	<LabeledField
		id={initialFieldId}
		label="Client's first initial"
		error={offerSubmission.errorFor(initialFieldId)}
		width={2}
	>
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				maxlength={1}
				value={initial}
				onInput={(value) => (initial = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField id={areaFieldId} label="General area" error={offerSubmission.errorFor(areaFieldId)} width={20}>
		{#snippet children({ id, describedBy, invalid, width })}
			<TextInput
				{id}
				{describedBy}
				{invalid}
				{width}
				value={clientArea}
				onInput={(value) => (clientArea = value)}
				required
			/>
		{/snippet}
	</LabeledField>
	<LabeledField id={dueDateFieldId} label="Due date" error={offerSubmission.errorFor(dueDateFieldId)}>
		{#snippet children({ id, describedBy, invalid })}
			<TextInput {id} {describedBy} {invalid} type="date" value={dueDate} onInput={(value) => (dueDate = value)} required />
		{/snippet}
	</LabeledField>
	<LabeledField label="Terms">
		{#snippet children({ id, describedBy, invalid })}
			<Textarea {id} {describedBy} {invalid} value={terms} onInput={(next) => (terms = next)} />
		{/snippet}
	</LabeledField>

	<Button label="Send Offer" type="submit" loading={offerSubmission.isSubmitting} />
</StackedForm>
