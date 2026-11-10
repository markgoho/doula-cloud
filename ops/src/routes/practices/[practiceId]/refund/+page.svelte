<script lang="ts">
	import { longDate } from '#lib/dates.js';
	import { money } from '#lib/money.js';
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { CheckAct } from '#lib/checkAct.svelte.js';
	import { CheckRead } from '#lib/checkRead.svelte.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { type Operator } from '#lib/session.js';
	import {
		answerErrors,
		CREDITS_FIELD,
		readRefundCheck,
		REQUESTED_BY_FIELD,
		REQUESTED_ON_FIELD,
		REQUESTED_ON_TARGET,
		requestedOn,
		sendRefund,
		type RefundAnswers,
		type RefundCheck,
		type RefundQuote,
		type RefundResult
	} from '#lib/refund.js';
	import DateFields from '$ui/molecules/DateFields.svelte';
	import ErrorSummary from '$ui/molecules/ErrorSummary.svelte';
	import RadioGroup, { radioFieldId } from '$ui/molecules/RadioGroup.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';
	import SummaryList from '$ui/molecules/SummaryList.svelte';

	// Each step carries what it draws: the check its quote, the done screen its result.
	type Screen = { step: 'answers' } | { step: 'check'; quote: RefundQuote } | { step: 'done'; result: RefundResult };

	const practiceId = $derived(page.params.practiceId ?? '');

	const operator = $derived(page.data.operator as Operator);
	let screen = $state<Screen>({ step: 'answers' });

	let credits = $state('');
	let requestedBy = $state('');
	let requestDate = $state<Pick<RefundAnswers, 'month' | 'day' | 'year'>>({ month: '', day: '', year: '' });

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	const read = new CheckRead<RefundCheck>(signedOut);
	const ownerTarget = $derived(
		read.value?.owners[0] ? radioFieldId(REQUESTED_BY_FIELD, read.value.owners[0].staffId) : REQUESTED_BY_FIELD
	);
	const located = (list: FormError[]) =>
		list.map((error) => (error.targetId === REQUESTED_BY_FIELD ? { ...error, targetId: ownerTarget } : error));
	// Continue re-reads the check for the Credits asked for; the refund itself asks for a fresh code.
	const quoteAct = new CheckAct<RefundCheck>({ onsignedout: signedOut, locate: located });
	const refundAct = new CheckAct<RefundResult>({
		onsignedout: signedOut,
		locate: located,
		stepUp: { email: () => operator.email, freshIdToken }
	});
	const errors = $derived(read.state.status === 'failed' ? read.errors : [...quoteAct.errors, ...refundAct.errors]);
	const errorFor = (field: string) => errors.find((error) => error.targetId === field)?.message;
	const ownerName = $derived(read.value?.owners.find((owner) => owner.staffId === requestedBy)?.name ?? '');

	onMount(async () => {
		await read.load(() => readRefundCheck(practiceId));
		if (read.value?.quote) credits = String(read.value.quote.maxCredits);
	});

	async function show(next: Screen, focus?: string) {
		screen = next;
		await tick();
		if (focus) document.querySelector<HTMLElement>(`#${focus}`)?.focus();
		else document.querySelector('h1')?.focus();
	}

	async function continueToCheck(event: SubmitEvent) {
		event.preventDefault();
		// New answers are a new refund: its own key, and no code or refusal from the last one.
		refundAct.renew();
		const local = answerErrors({ credits, requestedBy, ...requestDate });
		if (local.length > 0) return quoteAct.refuse(local);
		const check = await quoteAct.run(() => readRefundCheck(practiceId, Number(credits)));
		if (!check) return;
		read.set(check);
		// A quote gone since the page loaded (its purchase spent or too old) draws the answers screen's own reason.
		await show(check.quote ? { step: 'check', quote: check.quote } : { step: 'answers' });
	}

	async function refund(event: SubmitEvent) {
		event.preventDefault();
		const result = await refundAct.run((key) =>
			sendRefund(practiceId, { credits: Number(credits), requestedBy, requestedOn: requestedOn(requestDate)! }, key)
		);
		if (result) await show({ step: 'done', result });
	}
</script>

<svelte:head><title>Refund Credits – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if screen.step === 'done'}
	{@const result = screen.result}
	<stack-l role="status">
		<h1 tabindex="-1">Refund made</h1>
		<p>
			{result.credits === 1 ? '1 Credit' : `${result.credits} Credits`} refunded to {result.practiceName}.
			{money(result.amountCents)} goes back to the card it was paid with, including {money(result.taxCents)} sales tax.
		</p>
		<p>
			Asked for by {result.requestedByName} on {longDate(result.requestedOn)}. The Practice's Activity shows the refund
			under your name.
		</p>
	</stack-l>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'failed'}
	<h1 tabindex="-1">Refund Credits</h1>
	<p><a href={resolve('/')}>Back to DoulaCloud operations</a></p>
{:else if read.state.status === 'loaded'}
	{@const check = read.state.value}
	<p class="caption">{check.practiceName}</p>
	{#if screen.step === 'check'}
		{@const quote = screen.quote}
		<h1 tabindex="-1">Check the refund before you make it</h1>
		<SummaryList
			rows={[
				{ label: 'Practice', value: check.practiceName },
				{ label: 'Credits', value: String(quote.credits), changes: 'credits', onchange: () => show({ step: 'answers' }, CREDITS_FIELD) },
				{
					label: 'Amount back',
					value: `${money(quote.amountCents)}, including ${money(quote.taxCents)} sales tax, to the card it was paid with`
				},
				{ label: 'Owner who asked', value: ownerName, changes: 'owner who asked', onchange: () => show({ step: 'answers' }, ownerTarget) },
				{
					label: 'Date the request arrived',
					value: longDate(requestedOn(requestDate)!),
					changes: 'date the request arrived', onchange: () => show({ step: 'answers' }, REQUESTED_ON_TARGET)
				}
			]}>
			{#snippet action(row, describedBy)}
				<Button variant="link" label="Change" {describedBy} onClick={row.onchange} />
			{/snippet}
		</SummaryList>
		<form novalidate onsubmit={refund}>
			<stack-l>
				<StepUpFields {errors} bind:password={refundAct.password} bind:code={refundAct.code} />
				<p>Stripe moves the money at once. A refund cannot be undone.</p>
				<Button label="Refund {money(quote.amountCents)}" type="submit" loading={refundAct.isSending} />
			</stack-l>
		</form>
	{:else}
		<h1 tabindex="-1">Refund Credits</h1>
		{#if !check.quote}
			<p>
				This Practice holds no purchased Credits that can be refunded. Credits given free, already spent or bought more
				than three years ago are not refundable.
			</p>
		{:else if check.owners.length === 0}
			<p>This Practice has no Owner, so no one can be named as asking for a refund.</p>
		{:else}
			<form novalidate onsubmit={continueToCheck}>
				<stack-l>
					<LabeledField id={CREDITS_FIELD} label="Number of Credits" hint={`Up to ${check.quote.maxCredits}. One refund gives back Credits from one purchase.`}
						error={errorFor(CREDITS_FIELD)}
					>
						{#snippet children({ id, describedBy, invalid })}
							<TextInput
								{id}
								{describedBy}
								{invalid}
								inputmode="numeric"
								value={credits}
								onInput={(next) => (credits = next)}
							/>
						{/snippet}
					</LabeledField>
					<RadioGroup
						name={REQUESTED_BY_FIELD}
						legend="Owner who asked for the refund"
						options={check.owners.map((owner) => ({ value: owner.staffId, label: owner.name }))}
						value={requestedBy || undefined}
						onChange={(next) => (requestedBy = next)}
						error={errorFor(ownerTarget)}
					/>
					<DateFields
						name={REQUESTED_ON_FIELD}
						legend="Date the request arrived"
						hint="For example, 10 30 2026."
						parts={requestDate}
						onChange={(next) => (requestDate = next)}
						error={errorFor(REQUESTED_ON_TARGET)}
					/>
					<Button label="Continue" type="submit" loading={quoteAct.isSending} />
				</stack-l>
			</form>
		{/if}
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
