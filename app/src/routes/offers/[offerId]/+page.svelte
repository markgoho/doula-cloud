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
	import { page } from '$app/state';
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

	const token = $derived(page.url.searchParams.get('token') ?? '');
	const offerId = $derived(page.params.offerId!);

	let code = $state('');
	let offer = $state<PreAccountOffer | undefined>();
	let error = $state('');
	let isOpening = $state(false);
	let isDeclining = $state(false);

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
		isDeclining = true;
		try {
			const decided = await declinePreAccountOffer(apiFetch, offerId, token, code);
			offer &&= { ...offer, state: decided.state };
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Could not decline this offer';
		} finally {
			isDeclining = false;
		}
	}
</script>

<Heading level={1} text="An offer of work" />

{#if !token}
	<Notice message="This link is missing its token. Open the offer from the email you were sent." variant="error" />
{:else if !offer}
	<Text text="Enter the six-digit code from the email to open this offer." muted />
	<form onsubmit={handleOpen}>
		<label>
			Access code
			<input type="text" inputmode="numeric" maxlength="6" bind:value={code} required />
		</label>
		<Button label="Open offer" type="submit" loading={isOpening} />
	</form>
{:else}
	<dl>
		<dt>Client</dt>
		<dd>{offer.clientFirstInitial}</dd>
		<dt>Area</dt>
		<dd>{offer.clientArea}</dd>
		<dt>Due date</dt>
		<dd>{offer.dueDate}</dd>
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
		<Text text="Accepting this work means joining the practice, so that the offer can be recorded in your name." muted />
		<Link href={`${resolve('/accept-invite')}?token=${encodeURIComponent(token)}`} label="Join and accept" />
		<Button label="Decline" variant="secondary" onClick={handleDecline} loading={isDeclining} />
	{/if}
{/if}

{#if error}
	<Notice message={error} variant="error" />
{/if}
