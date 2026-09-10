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
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import TextInput from '#lib/components/atoms/TextInput.svelte';
	import LabeledField from '#lib/components/molecules/LabeledField.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import PageTitle from '#lib/components/PageTitle.svelte';

	const token = $derived(page.url.searchParams.get('token') ?? '');
	const offerId = $derived(page.params.offerId!);

	let code = $state('');
	let offer = $state<PreAccountOffer | undefined>();
	let error = $state('');
	let isOpening = $state(false);
	let isDeclineDialogOpen = $state(false);

	async function handleOpen(event: SubmitEvent) {
		event.preventDefault();
		error = '';
		isOpening = true;
		try {
			offer = await loadPreAccountOffer(apiFetch, offerId, token, code);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Could not open this offer';
		} finally {
			isOpening = false;
		}
	}

	async function handleDecline() {
		error = '';
		try {
			const decided = await declinePreAccountOffer(apiFetch, offerId, token, code);
			offer &&= { ...offer, state: decided.state };
		} catch (error_) {
			// Rethrown so ConfirmDialog stays open and renders this inside
			// itself (#804), rather than closing over a failure with no
			// account to sign back into and try again from.
			error = error_ instanceof Error ? error_.message : 'Could not decline this offer';
			throw error_;
		}
	}
</script>

<PageTitle page="An offer of work" />
<Heading level={1} text="An offer of work" />

{#if !token}
	<Notice message="This link is missing its token. Open the offer from the email you were sent." variant="error" />
{:else if !offer}
	<Text text="Enter the six-digit code from the email to open this offer." tone="variant" />
	<!--
		#660: this is the one signed-out form that is not a `StackedForm`.
		That molecule sets `novalidate`, because ADR-0021's Recover from
		validation errors pattern is that the page refuses the submit and
		says so once at the top -- and this screen has no `ErrorSummary` and
		no refusal path to say it with, so it is still relying on the
		browser's own bubble to stop an empty access code. Adopting the
		molecule here would take that refusal away and put nothing in its
		place, so the wrapper is inline until this screen gets #467's error
		summary of its own (#1107).
	-->
	<form onsubmit={handleOpen}>
		<stack-l space="var(--space-5)">
			<LabeledField label="Access code">
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
					/>
				{/snippet}
			</LabeledField>
			<Button label="Open offer" type="submit" loading={isOpening} />
		</stack-l>
	</form>
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
			error={error}
			onConfirm={handleDecline}
		/>
	{/if}
{/if}

{#if error && !isDeclineDialogOpen}
	<!-- Decline's own failure renders inside ConfirmDialog while it is
	     open (#804); this is the access-code form's, which has no dialog
	     to gate it. -->
	<Notice message={error} variant="error" />
{/if}
