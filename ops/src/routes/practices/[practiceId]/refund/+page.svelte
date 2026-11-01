<script lang="ts">
	import LabeledField from '$ui/molecules/LabeledField.svelte';
	import TextInput from '$ui/atoms/TextInput.svelte';
	import Button from '$ui/atoms/Button.svelte';
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import type { FormError } from '#lib/authRefusal.js';
	import { freshIdToken } from '#lib/firebase.js';
	import { readSession, type Operator } from '#lib/session.js';
	import { stepUp } from '#lib/stepUp.js';
	import {
		answerErrors,
		CREDITS_FIELD,
		longDate,
		money,
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


	const practiceId = $derived(page.params.practiceId ?? '');

	let operator = $state<Operator | undefined>();
	let check = $state<RefundCheck | undefined>();
	let step = $state<'answers' | 'check' | 'done'>('answers');
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);

	let credits = $state('');
	let requestedBy = $state('');
	let requestDate = $state<Pick<RefundAnswers, 'month' | 'day' | 'year'>>({ month: '', day: '', year: '' });
	let quote = $state<RefundQuote | undefined>();
	let result = $state<RefundResult | undefined>();
	let password = $state('');
	let code = $state('');
	let requestKey = '';

	const ownerTarget = $derived(
		check?.owners[0] ? radioFieldId(REQUESTED_BY_FIELD, check.owners[0].staffId) : REQUESTED_BY_FIELD
	);
	const located = (list: FormError[]) =>
		list.map((error) => (error.targetId === REQUESTED_BY_FIELD ? { ...error, targetId: ownerTarget } : error));
	const errorFor = (field: string) => errors.find((error) => error.targetId === field)?.message;
	const ownerName = $derived(check?.owners.find((owner) => owner.staffId === requestedBy)?.name ?? '');

	async function signedOut() {
		await goto(resolve('/sign-in'));
	}

	onMount(async () => {
		operator = await readSession();
		if (!operator) return signedOut();
		const outcome = await readRefundCheck(practiceId);
		if (outcome.kind === 'signed-out') return signedOut();
		if (outcome.kind === 'ok') {
			check = outcome.value;
			if (check.quote) credits = String(check.quote.maxCredits);
		} else {
			errors = outcome.kind === 'refused' ? outcome.errors : [];
		}
	});

	async function show(next: typeof step, focus?: string) {
		step = next;
		await tick();
		if (focus) document.querySelector<HTMLElement>(`#${focus}`)?.focus();
		else document.querySelector('h1')?.focus();
	}

	async function continueToCheck(event: SubmitEvent) {
		event.preventDefault();
		errors = located(answerErrors({ credits, requestedBy, ...requestDate }));
		if (errors.length > 0) return;
		isSending = true;
		try {
			const outcome = await readRefundCheck(practiceId, Number(credits));
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind !== 'ok') {
				errors = outcome.kind === 'refused' ? located(outcome.errors) : [];
				return;
			}
			quote = outcome.value.quote ?? undefined;
			requestKey = crypto.randomUUID();
			password = '';
			code = '';
			await show('check');
		} finally {
			isSending = false;
		}
	}

	async function refund(event: SubmitEvent) {
		event.preventDefault();
		isSending = true;
		try {
			errors = await stepUp({ email: operator!.email, password, code }, { freshIdToken });
			if (errors.length > 0) return;
			const outcome = await sendRefund(
				practiceId,
				{ credits: Number(credits), requestedBy, requestedOn: requestedOn(requestDate)! },
				requestKey
			);
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind === 'step-up') {
				errors = [{ message: 'Enter the code your authenticator app shows now to do this.', targetId: 'step-up-code' }];
				return;
			}
			if (outcome.kind === 'refused') {
				errors = located(outcome.errors);
				return;
			}
			result = outcome.value;
			await show('done');
		} finally {
			isSending = false;
		}
	}
</script>

<svelte:head><title>Refund Credits – DoulaCloud operations</title></svelte:head>

<ErrorSummary {errors} />

{#if step === 'done' && result}
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
{:else if check && step === 'check' && quote}
	<p class="caption">{check.practiceName}</p>
	<h1 tabindex="-1">Check the refund before you make it</h1>
	<SummaryList
		rows={[
			{ label: 'Practice', value: check.practiceName },
			{ label: 'Credits', value: String(quote.credits), changes: 'credits', onchange: () => show('answers', CREDITS_FIELD) },
			{
				label: 'Amount back',
				value: `${money(quote.amountCents)}, including ${money(quote.taxCents)} sales tax, to the card it was paid with`
			},
			{ label: 'Owner who asked', value: ownerName, changes: 'owner who asked', onchange: () => show('answers', ownerTarget) },
			{
				label: 'Date the request arrived',
				value: longDate(requestedOn(requestDate)!),
				changes: 'date the request arrived', onchange: () => show('answers', REQUESTED_ON_TARGET)
			}
		]}>
		{#snippet action(row, describedBy)}
			<Button variant="link" label="Change" {describedBy} onClick={row.onchange} />
		{/snippet}
	</SummaryList>
	<form novalidate onsubmit={refund}>
		<stack-l>
			<StepUpFields {errors} bind:password bind:code />
			<p>Stripe moves the money at once. A refund cannot be undone.</p>
			<Button label="Refund {money(quote.amountCents)}" type="submit" loading={isSending} />
		</stack-l>
	</form>
{:else if check}
	<p class="caption">{check.practiceName}</p>
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
				<Button label="Continue" type="submit" loading={isSending} />
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
