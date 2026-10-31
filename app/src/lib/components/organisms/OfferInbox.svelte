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
	import { formatCalendarDay, formatInstant } from '#lib/dates.js';
	import { feePaidLine } from '#lib/doulaFee.js';
	import { formatFee, isOpen, offerStateLabels, offerStateVariants, type Offer } from '#lib/offer.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';

	let {
		offers,
		onDecide
	}: {
		offers: Offer[];
		onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<void>;
	} = $props();

	/*
	 * An Offer's facts, through `SummaryList` (#2021). The Client's own
	 * fields stop being served once an Offer goes terminal (#230), so a
	 * past Offer reads as the fact of the asking rather than as three
	 * blank rows. `terms` is a Practice's own free text and may be a
	 * pasted URL (#595); the list's value column breaks it.
	 */
	function offerFacts(offer: Offer): SummaryListRow[] {
		const rows: SummaryListRow[] = [];
		if (isOpen(offer)) {
			rows.push(
				{ label: 'Client', value: offer.clientFirstInitial },
				{ label: 'Area', value: offer.clientArea },
				{ label: 'Due date', value: formatCalendarDay(offer.dueDate) }
			);
		}
		if (offer.state === 'accepted' && offer.decidedAt) {
			rows.push({ label: 'Accepted on', value: formatInstant(offer.decidedAt) });
		}
		rows.push({ label: 'Fee', value: formatFee(offer.amountCents) });
		// #2271: what the Practice recorded paying her, on the one screen
		// she keeps after the job ends. Only an accepted fee is ever paid.
		if (offer.state === 'accepted' && offer.amountCents !== undefined) {
			rows.push({ label: 'Paid', value: offer.feePaid ? feePaidLine(offer.feePaid) : 'Not recorded yet' });
		}
		if (offer.terms) {
			rows.push({ label: 'Terms', value: offer.terms });
		}
		return rows;
	}

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
	<EmptyState message="You have no offers." />
{:else}
	<ul>
		{#each offers as offer (offer.offerId)}
			<li>
				<Badge label={offerStateLabels[offer.state]} variant={offerStateVariants[offer.state]} />
				<SummaryList rows={offerFacts(offer)} />
				{#if isOpen(offer)}
					<!-- v8 ignore start: Svelte-compiled attribute-diffing branches for
					     the dynamic id/describedBy strings below aren't reachable from
					     app-level interaction tests (no test changes an Offer's own id
					     mid-test), only from Svelte's own reactivity internals. -->
					<!-- One group, Accept the one primary, so the two answers keep a
					     gap between them and wrap together at a narrow width (#2019). -->
					<cluster-l space="var(--space-3)">
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
					</cluster-l>
					<span class="visually-hidden" id="offer-{offer.offerId}-decide-name"
						>{offer.clientArea}, due {formatCalendarDay(offer.dueDate)}</span
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
