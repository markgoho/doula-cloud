<script lang="ts">
	/**
	 * A Staff member's own Offers (#230, #317): what she has been offered
	 * and not yet taken, and what became of the ones she has already
	 * decided. The four decidable facts plus the terms are everything
	 * shown, because they are everything an Offer carries -- no Client
	 * name, no Engagement to click through to, until she accepts.
	 *
	 * onDecide owns the API call and the resulting state change; this
	 * component reports which Offer and which answer.
	 */
	import { formatFee, isOpen, offerStateLabels, type Offer } from './offer.js';
	import Badge from './components/atoms/Badge.svelte';
	import Button from './components/atoms/Button.svelte';
	import Notice from './components/atoms/Notice.svelte';

	let {
		offers,
		onDecide
	}: {
		offers: Offer[];
		onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<void>;
	} = $props();

	let decidingId = $state('');
	let decideError = $state('');

	const badgeVariants = {
		offered: 'info',
		accepted: 'success',
		declined: 'neutral',
		withdrawn: 'neutral',
		superseded: 'neutral',
		expired: 'warning'
	} as const;

	async function handleDecide(offerId: string, action: 'accept' | 'decline') {
		decideError = '';
		decidingId = offerId;
		try {
			await onDecide(offerId, action);
		} catch (error_) {
			decideError = error_ instanceof Error ? error_.message : 'Failed to answer this offer';
		} finally {
			decidingId = '';
		}
	}
</script>

{#if offers.length === 0}
	<p>You have no offers.</p>
{:else}
	<ul>
		{#each offers as offer (offer.offerId)}
			<li>
				<Badge label={offerStateLabels[offer.state]} variant={badgeVariants[offer.state]} />
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
				</dl>
				{#if isOpen(offer)}
					<Button
						label="Accept"
						loading={decidingId === offer.offerId}
						onClick={() => handleDecide(offer.offerId, 'accept')}
					/>
					<Button
						label="Decline"
						variant="secondary"
						loading={decidingId === offer.offerId}
						onClick={() => handleDecide(offer.offerId, 'decline')}
					/>
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if decideError}
	<Notice message={decideError} variant="error" />
{/if}
