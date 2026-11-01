<script lang="ts">
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
		requestedOn,
		sendRefund,
		type RefundCheck,
		type RefundQuote,
		type RefundResult
	} from '#lib/refund.js';
	import DateFields from '#lib/components/DateFields.svelte';
	import ErrorSummary from '#lib/components/ErrorSummary.svelte';
	import RadioGroup from '#lib/components/RadioGroup.svelte';
	import StepUpFields from '#lib/components/StepUpFields.svelte';
	import SummaryList from '#lib/components/SummaryList.svelte';
	import TextField from '#lib/components/TextField.svelte';

	const practiceId = $derived(page.params.practiceId ?? '');

	let operator = $state<Operator | undefined>();
	let check = $state<RefundCheck | undefined>();
	let step = $state<'answers' | 'check' | 'done'>('answers');
	let errors = $state<FormError[]>([]);
	let isSending = $state(false);

	let credits = $state('');
	let requestedBy = $state('');
	let month = $state('');
	let day = $state('');
	let year = $state('');
	let quote = $state<RefundQuote | undefined>();
	let result = $state<RefundResult | undefined>();
	let password = $state('');
	let code = $state('');
	let requestKey = '';

	const errorFor = (field: string) => errors.find((error) => error.field === field)?.message;
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
		errors = answerErrors({ credits, requestedBy, day, month, year });
		if (errors.length > 0) return;
		isSending = true;
		try {
			const outcome = await readRefundCheck(practiceId, Number(credits));
			if (outcome.kind === 'signed-out') return signedOut();
			if (outcome.kind !== 'ok') {
				errors = outcome.kind === 'refused' ? outcome.errors : [];
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
				{ credits: Number(credits), requestedBy, requestedOn: requestedOn({ day, month, year })! },
				requestKey
			);
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
			{ key: 'Practice', value: check.practiceName },
			{ key: 'Credits', value: String(quote.credits), onchange: () => show('answers', CREDITS_FIELD) },
			{
				key: 'Amount back',
				value: `${money(quote.amountCents)}, including ${money(quote.taxCents)} sales tax, to the card it was paid with`
			},
			{ key: 'Owner who asked', value: ownerName, onchange: () => show('answers', REQUESTED_BY_FIELD) },
			{
				key: 'Date the request arrived',
				value: longDate(requestedOn({ day, month, year })!),
				onchange: () => show('answers', REQUESTED_ON_FIELD)
			}
		]}
	/>
	<form novalidate onsubmit={refund}>
		<stack-l>
			<StepUpFields {errors} bind:password bind:code />
			<p>Stripe moves the money at once. A refund cannot be undone.</p>
			<button class="ops-button" type="submit" disabled={isSending}>Refund {money(quote.amountCents)}</button>
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
				<TextField
					id={CREDITS_FIELD}
					label="Number of Credits"
					hint={`Up to ${check.quote.maxCredits}. One refund gives back Credits from one purchase.`}
					inputmode="numeric"
					bind:value={credits}
					error={errorFor(CREDITS_FIELD)}
				/>
				<RadioGroup
					id={REQUESTED_BY_FIELD}
					legend="Owner who asked for the refund"
					options={check.owners.map((owner) => ({ value: owner.staffId, label: owner.name }))}
					bind:value={requestedBy}
					error={errorFor(REQUESTED_BY_FIELD)}
				/>
				<DateFields
					id={REQUESTED_ON_FIELD}
					legend="Date the request arrived"
					hint="For example, 10 30 2026."
					bind:month
					bind:day
					bind:year
					error={errorFor(REQUESTED_ON_FIELD)}
				/>
				<button class="ops-button" type="submit" disabled={isSending}>Continue</button>
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
