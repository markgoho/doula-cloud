<script lang="ts">
	/*
	 * Getting paid's payment terms, as its own question (#2046, #768): how
	 * many days after an Invoice is raised it falls due. One thing per
	 * page (GOV.UK Question pages), at the width of a three-digit answer
	 * (#1216). Its Save returns to Getting paid, which says what was saved
	 * in a status `Notice` before its <h1> (#1710).
	 *
	 * An Owner or an Admin sets it; the BFF is the authority. Anyone else
	 * who reaches this address reads the terms and who may change them,
	 * never a Save that exists only to be refused -- the Practice name
	 * screen's shape.
	 *
	 * ## Why the <form> is outside the Template
	 *
	 * `QuestionPage` renders the control and the button as two regions of
	 * one column, so no <form> inside it could hold both. Round the outside,
	 * Enter in the box saves.
	 */
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import { isOwnerOrAdmin } from '#lib/roles.js';
	import {
		loadPaymentTerms,
		setPaymentTerms,
		MIN_PAYMENT_TERMS_DAYS,
		MAX_PAYMENT_TERMS_DAYS,
		netDaysOutOfRangeMessage
	} from '#lib/invoice.js';
	import { gotoWithOutcome } from '#lib/outcome.js';
	import { FormSubmission, orThrownMessage } from '#lib/formSubmission.svelte.js';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import FormPage from '#lib/components/templates/FormPage.svelte';
	import QuestionPage from '#lib/components/templates/QuestionPage.svelte';
	import { paymentTermsSavedMessage, paymentTermsValue } from '../gettingPaid.js';
	import type { PracticeSession } from '../../../+layout.js';

	// Fixed, so the error summary's `<a href="#id">` reaches the box.
	const NET_DAYS_ID = 'payment-terms-net-days';
	const QUESTION = 'How many days does a Client have to pay an Invoice?';

	const session = $derived((page.data as { session: PracticeSession }).session);
	const canChange = $derived(isOwnerOrAdmin(session));
	const gettingPaidHref = $derived(
		resolve('/practices/[practiceId]/settings/payments', { practiceId: page.params.practiceId! })
	);

	let typedNetDays = $state('');
	let currentValue = $state('');
	let loadError = $state('');
	const submission = new FormSubmission();

	onMount(async () => {
		try {
			const terms = await loadPaymentTerms(apiFetchWithSession, page.params.practiceId!);
			typedNetDays = String(terms.netDays);
			currentValue = paymentTermsValue(terms);
		} catch (error_) {
			loadError = error_ instanceof Error ? error_.message : 'Failed to load payment terms';
		}
	});

	async function save(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			// Refused here before the boundary refuses it, in the same words
			// the BFF uses (`netDaysOutOfRangeMessage`).
			const netDays = Number(typedNetDays);
			if (!Number.isSafeInteger(netDays) || netDays < MIN_PAYMENT_TERMS_DAYS || netDays > MAX_PAYMENT_TERMS_DAYS) {
				return [{ message: netDaysOutOfRangeMessage, targetId: NET_DAYS_ID }];
			}
			const saved = await setPaymentTerms(apiFetchWithSession, page.params.practiceId!, netDays);
			await gotoWithOutcome(gettingPaidHref, paymentTermsSavedMessage(saved));
		}, orThrownMessage);
	}
</script>

<!-- Passed only while there is a refusal: a declared snippet is always
     truthy, so the Template would title a first visit "Error: " (#1705). -->
{#snippet errorSummary()}
	<ErrorSummary errors={submission.errors} />
{/snippet}

{#snippet readOnly()}
	<SummaryList rows={[{ label: 'Payment terms', value: currentValue }]} />
	<Notice variant="status" message="Only a Practice Owner or Admin can change the payment terms." />
{/snippet}

{#snippet noActions()}{/snippet}

{#if !canChange || loadError}
	<FormPage
		title="Payment terms"
		fieldsets={[{ content: readOnly }]}
		actions={noActions}
		loading={currentValue === '' && !loadError ? 'Loading the payment terms' : undefined}
		loadError={loadError || undefined}
	/>
{:else}
	<!-- stacked-form:ignore: #1108 -- this form wraps a Template. `QuestionPage` renders the control and the button as two separate regions of one column and stacks each itself, so the `<form>` here exists to put the submit button inside the form that owns the input, not to arrange anything. -->
	<form onsubmit={save} novalidate>
		<QuestionPage
			journey="Getting paid"
			steps={[]}
			backHref={gettingPaidHref}
			caption="Getting paid"
			question={{ as: 'label', text: QUESTION, for: NET_DAYS_ID }}
			hint="Applies to the next Invoice raised. An Invoice already raised keeps the terms it was billed under."
			errorSummary={submission.errors.length > 0 ? errorSummary : undefined}
		>
			{#snippet content({ describedBy })}
				{@const error = submission.errorFor(NET_DAYS_ID)}
				<!-- GOV.UK's error message, above the box, as `LabeledField`
				     draws one; here the label is the page's <h1>, so the
				     Template owns it and the box is drawn bare. -->
				{#if error}
					<p class="error" id="{NET_DAYS_ID}-error" role="alert">{error}</p>
				{/if}
				<!-- Up to 365 days, so three characters (#1216); the question
				     says "days" for a screen reader, which the suffix is not. -->
				<TextInput
					id={NET_DAYS_ID}
					describedBy={[describedBy, error && `${NET_DAYS_ID}-error`].filter(Boolean).join(' ') || undefined}
					invalid={Boolean(error)}
					width={3}
					suffix="days"
					type="number"
					min={MIN_PAYMENT_TERMS_DAYS}
					inputmode="numeric"
					value={typedNetDays}
					onInput={(value) => (typedNetDays = value)}
				/>
			{/snippet}
			{#snippet actions()}
				<Button type="submit" label="Save payment terms" loading={submission.isSubmitting} />
			{/snippet}
		</QuestionPage>
	</form>
{/if}

<style>
	@layer components {
		.error {
			margin: 0 0 var(--space-2);
			color: var(--color-error);
			font-size: var(--text-body-sm-size);
		}
	}
</style>
