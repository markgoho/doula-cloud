<script lang="ts">
	/**
	 * The pre-account Offer read (#230, ADR-0008). The link in the email
	 * carries the Invitation token; the six-digit code is typed here. Both
	 * together open exactly one Offer, and what it serves is the four
	 * decidable facts plus the terms -- enough to decide, and nothing about
	 * the Client or the practice.
	 *
	 * Declining works from here, without an account. Accepting cannot: an
	 * attachment to an Engagement names a person, so accepting means
	 * joining the practice first, through the same token this page already
	 * holds.
	 */
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetch } from '#lib/api.js';
	import {
		declinePreAccountOffer,
		formatFee,
		offerStateLabels,
		loadPreAccountOffer,
		type PreAccountOffer
	} from '#lib/offer.js';
	import { FormSubmission, orThrownMessage, type FormError } from '#lib/formSubmission.svelte.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import ErrorSummary from '#lib/components/molecules/ErrorSummary.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import StackedForm from '#lib/components/molecules/StackedForm.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	const codeId = 'offer-access-code';
	const CODE_PATTERN = /^\d{6}$/;

	const token = $derived(page.url.searchParams.get('token') ?? '');
	const offerId = $derived(page.params.offerId!);

	let code = $state('');
	let offer = $state<PreAccountOffer | undefined>();
	/*
	 * Declining's own failure, kept apart from the access-code form's
	 * (#804): a rejected `onConfirm` leaves the dialog open over an
	 * `inert` page, so it is reported as a `Notice` inside the dialog
	 * rather than as an entry in the summary behind it, whose fragment
	 * link could not focus anything anyway.
	 */
	let declineError = $state('');
	let isDeclineDialogOpen = $state(false);
	const submission = new FormSubmission();

	/*
	 * The page's own refusal, in our words rather than the browser's
	 * (#1107, ADR-0021's Recover from validation errors pattern). Both
	 * messages name the next action and what a code looks like, because a
	 * person reaching this screen is often unsure why she is being asked
	 * for a code at all.
	 */
	function refusedCode(entered: string): FormError[] | undefined {
		if (entered === '') {
			return [{ message: 'Enter the six-digit code from your email', targetId: codeId }];
		}
		if (!CODE_PATTERN.test(entered)) {
			return [
				{ message: 'The code from your email is six digits, like 123456', targetId: codeId }
			];
		}
		return undefined;
	}

	async function handleOpen(event: SubmitEvent) {
		event.preventDefault();
		await submission.run(async () => {
			const entered = code.trim();
			const refused = refusedCode(entered);
			if (refused) return refused;

			/*
			 * A wrong code and an expired token come back as one sentence
			 * the BFF wrote, and it deliberately does not say which of the
			 * two credentials was the wrong one -- so the refusal it throws
			 * reports untargeted, the same way `recovery-code`'s does.
			 */
			offer = await loadPreAccountOffer(apiFetch, offerId, token, entered);
		}, orThrownMessage);
	}

	async function handleDecline() {
		declineError = '';
		try {
			const decided = await declinePreAccountOffer(apiFetch, offerId, token, code.trim());
			offer &&= { ...offer, state: decided.state };
		} catch (error_) {
			// Rethrown so ConfirmDialog stays open and renders this inside
			// itself (#804), rather than closing over a failure with no
			// account to sign back into and try again from.
			declineError = error_ instanceof Error ? error_.message : 'Could not decline this offer';
			throw error_;
		}
	}
</script>

<PageTitle page="An offer of work" isError={submission.errors.length > 0} />

<!--
	Above the `<h1>`, which is GOV.UK's own markup for the summary and the
	position every other signed-out screen puts it in (`forgot-password`
	and `reset-password` render it inline the same way, having no
	`EntryPage` to position it for them).
-->
<ErrorSummary errors={submission.errors} />

<Heading level={1} text="An offer of work" />

{#if !token}
	<Notice message="This link is missing its token. Open the offer from the email you were sent." variant="error" />
{:else if !offer}
	<Text text="Enter the six-digit code from the email to open this offer." tone="variant" />
	<StackedForm onSubmit={handleOpen}>
		<LabeledField id={codeId} label="Access code" error={submission.errorFor(codeId)}>
			{#snippet children({ id, describedBy, invalid })}
				<TextInput
					{id}
					{describedBy}
					{invalid}
					inputmode="numeric"
					maxlength={6}
					value={code}
					onInput={(value) => (code = value)}
					required
					autocomplete="one-time-code"
				/>
			{/snippet}
		</LabeledField>
		<Button label="Open offer" type="submit" loading={submission.isSubmitting} />
	</StackedForm>
{:else}
	<dl>
		<!-- #230: she opens the link in March and gets a closed offer, not
		     the Client's due date. The BFF stops serving these once the
		     Offer is terminal; the page stops asking for their row. -->
		{#if offer.state === 'offered'}
			<dt>Client</dt>
			<dd>{offer.clientFirstInitial}</dd>
			<dt>Area</dt>
			<dd>{offer.clientArea}</dd>
			<dt>Due date</dt>
			<dd>{offer.dueDate}</dd>
		{/if}
		<dt>Fee</dt>
		<dd>{formatFee(offer.amountCents)}</dd>
		{#if offer.terms}
			<dt>Terms</dt>
			<dd>{offer.terms}</dd>
		{/if}
		<dt>Status</dt>
		<dd>{offerStateLabels[offer.state]}</dd>
	</dl>

	{#if offer.state === 'offered'}
		<Text text="Accepting this work means joining the practice, so that the offer can be recorded in your name." tone="variant" />
		<Link href={`${resolve('/(signed-out)/accept-invite')}?token=${encodeURIComponent(token)}`} label="Join and accept" />
		<Button label="Decline" variant="destructive" onClick={() => (isDeclineDialogOpen = true)} />
		<ConfirmDialog
			bind:open={isDeclineDialogOpen}
			title="Decline this offer"
			consequence="Declining this offer cannot be undone."
			confirmLabel="Decline this offer"
			error={declineError}
			onConfirm={handleDecline}
		/>
	{/if}
{/if}
