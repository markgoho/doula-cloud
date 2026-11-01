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
	import { tick } from 'svelte';
	import { formatCalendarDay, formatCalendarMonth } from '#lib/dates.js';
	import { feePaidLine } from '#lib/doulaFee.js';
	import {
		clientFactRows,
		decideDescription,
		decisionOutcome,
		engagementLinkLabel,
		formatFee,
		isOpen,
		offerStateLabels,
		offerStateVariants,
		type DecisionOutcome,
		type Offer,
		type OfferDecision
	} from '#lib/offer.js';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ConfirmDialog from '#lib/components/molecules/ConfirmDialog.svelte';
	import EmptyState from '#lib/components/molecules/EmptyState.svelte';
	import SummaryList, { type SummaryListRow } from '#lib/components/molecules/SummaryList.svelte';

	/*
	 * #2268: `outcome` says what an answer did. The card it answered is
	 * gone once the list reloads, so it is focused where it appears. It is
	 * bindable so it outlives this component on a parent that draws it in
	 * another place after the reload (the Overview, once her first
	 * Engagement arrives).
	 */
	let {
		offers,
		onDecide,
		practiceName,
		engagementHref,
		outcome = $bindable()
	}: {
		offers: Offer[];
		onDecide: (offerId: string, action: 'accept' | 'decline') => Promise<OfferDecision>;
		practiceName: string;
		engagementHref: (engagementId: string) => string;
		outcome?: DecisionOutcome;
	} = $props();

	/*
	 * An Offer's facts, through `SummaryList` (#2021). The Client's own
	 * fields stop being served once an Offer goes terminal (#230), so a
	 * past Offer reads as the fact of the asking rather than as three
	 * blank rows. An accepted one keeps its due month (#2383), so two
	 * births she worked read apart. `terms` is a Practice's own free text
	 * and may be a pasted URL (#595); the list's value column breaks it.
	 */
	function offerFacts(offer: Offer): SummaryListRow[] {
		const rows: SummaryListRow[] = [];
		if (isOpen(offer)) {
			rows.push(...clientFactRows(offer));
		}
		if (offer.state === 'accepted' && offer.dueMonth) {
			rows.push({ label: 'Due', value: formatCalendarMonth(offer.dueMonth) });
		}
		// #2513: the Practice's day, as Ended beside it is.
		if (offer.state === 'accepted' && offer.acceptedOn) {
			rows.push({ label: 'Accepted on', value: formatCalendarDay(offer.acceptedOn) });
		}
		// #2270: the day the work ended, beside the day it began.
		if (offer.state === 'accepted' && offer.endedOn) {
			rows.push({ label: 'Ended', value: formatCalendarDay(offer.endedOn) });
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
		outcome = undefined;
		decidingId = offerId;
		try {
			outcome = decisionOutcome(await onDecide(offerId, action), practiceName, engagementHref);
		} catch (error_) {
			decideError = error_ instanceof Error ? error_.message : 'Failed to answer this offer';
		} finally {
			decidingId = '';
		}
	}

	// #2268: declining is final, so it is confirmed first, by the house's
	// hard confirm (docs/design/govuk-alignment.md, the modal Dialog).
	let declining = $state('');
	let isConfirmOpen = $state(false);

	function askToDecline(offerId: string) {
		declining = offerId;
		isConfirmOpen = true;
	}

	// Closed before the answer is sent, so the dialog's own return of
	// focus to Decline comes before the outcome takes it.
	async function confirmDecline() {
		isConfirmOpen = false;
		await tick();
		await handleDecide(declining, 'decline');
	}
</script>

{#if outcome}
	<stack-l space="var(--space-2)">
		<Notice message={outcome.message} variant="status" isFocusedOnAppear />
		{#if outcome.link}
			<Link href={outcome.link.href} label={outcome.link.label} />
		{/if}
	</stack-l>
{/if}

{#if offers.length === 0}
	<EmptyState message="You have no offers." />
{:else}
	<ul>
		{#each offers as offer (offer.offerId)}
			<li>
				<Badge label={offerStateLabels[offer.state]} variant={offerStateVariants[offer.state]} />
				<SummaryList rows={offerFacts(offer)} />
				{#if offer.engagementId}
					<Link href={engagementHref(offer.engagementId)} label={engagementLinkLabel(offer)} />
				{/if}
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
							onClick={() => askToDecline(offer.offerId)}
						/>
					</cluster-l>
					<span class="visually-hidden" id="offer-{offer.offerId}-decide-name"
						>{decideDescription(offer)}</span
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

<ConfirmDialog
	bind:open={isConfirmOpen}
	title="Decline this offer?"
	consequence="You can't undo this. {practiceName} can send you a new offer later."
	confirmLabel="Decline offer"
	cancelLabel="Keep the offer"
	onConfirm={confirmDecline}
/>
