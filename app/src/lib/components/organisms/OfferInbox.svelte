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
	import { formatFee, isOpen, offerStateLabels, offerStateVariants, type Offer } from '#lib/offer.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

	let {
		offers,
		onDecide
	}: {
		offers: Offer[];
		onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<void>;
	} = $props();

	let decidingId = $state('');
	let decideError = $state('');

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
				<Badge label={offerStateLabels[offer.state]} variant={offerStateVariants[offer.state]} />
				<dl>
					<!-- The Client's own fields stop being served once an Offer
					     goes terminal (#230), so a past Offer reads as the fact
					     of the asking rather than as three blank rows. -->
					{#if isOpen(offer)}
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
				</dl>
				{#if isOpen(offer)}
					<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for
					     the dynamic id/describedBy strings below aren't reachable from
					     app-level interaction tests (no test changes an Offer's own id
					     mid-test), only from Svelte's own reactivity internals. -->
					<Button
						label="Accept"
						describedBy="offer-{offer.offerId}-decide-name"
						loading={decidingId === offer.offerId}
						onClick={() => handleDecide(offer.offerId, 'accept')}
					/>
					<Button
						label="Decline"
						variant="secondary"
						describedBy="offer-{offer.offerId}-decide-name"
						loading={decidingId === offer.offerId}
						onClick={() => handleDecide(offer.offerId, 'decline')}
					/>
					<span class="visually-hidden" id="offer-{offer.offerId}-decide-name"
						>{offer.clientArea}, due {offer.dueDate}</span
					>
					<!-- v8 ignore stop -->
				{/if}
			</li>
		{/each}
	</ul>
{/if}

{#if decideError}
	<Notice message={decideError} variant="error" />
{/if}

<style>
	@layer components {
		/*
		 * #595: `terms` is a Practice's own free text about an Offer, and
		 * #530 already found a bare pasted URL -- no space or hyphen to
		 * break on -- pushing an unconstrained `dd` past its frame.
		 * `DescriptionList`'s own value column carries this exact pairing
		 * for the same reason (its own comment); this `dl` predates that
		 * molecule and had never been measured with hostile content.
		 */
		dd {
			max-inline-size: var(--measure);
			overflow-wrap: anywhere;
		}
	}
</style>
