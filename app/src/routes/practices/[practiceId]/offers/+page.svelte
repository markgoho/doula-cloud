<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import { resolve } from '$app/paths';
	import type { PracticeSession } from '../+layout.js';
	import { practiceTimeZone } from '#lib/practiceZone.js';
	import { decideOffer, loadInbox, type DecisionOutcome, type Offer } from '#lib/offer.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import { PageRead } from '#lib/pageRead.svelte.js';

	// The inbox's read. A failed first read is said on its own, never over
	// an empty inbox that would read as "nothing offered".
	const inbox = new PageRead<Offer[]>();
	const LOAD_FAILURE = 'Failed to load your offers';

	function readInbox(): Promise<Offer[]> {
		return loadInbox(apiFetchWithSession, page.params.practiceId!);
	}

	// The list is reloaded rather than patched in place: accepting one
	// Offer supersedes every other open Offer on the same Engagement, and
	// those are rows this list may well be showing.
	async function handleDecide(offerId: string, action: 'accept' | 'decline') {
		const decision = await decideOffer(apiFetchWithSession, page.params.practiceId!, offerId, action);
		// In place: the decision went through, so the inbox stays drawn and
		// a failed re-read is said above it.
		await inbox.refresh(readInbox, LOAD_FAILURE);
		return decision;
	}

	let outcome = $state<DecisionOutcome>();

	const engagementHref = (engagementId: string) =>
		resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId
		});

	onMount(() => inbox.load(readInbox, LOAD_FAILURE));
</script>

{#snippet intro()}
	<Text
		text="Work you have been offered. Accepting puts you on the birth; declining is final for that Offer, and the Practice may ask you again."
		tone="variant"
	/>
{/snippet}

{#snippet content()}
	{#if inbox.state.status === 'failed'}
		<Notice message={inbox.state.message} variant="error" />
	{:else if inbox.state.status === 'loaded'}
		{#if inbox.refreshError}
			<Notice message={inbox.refreshError} variant="error" />
		{/if}
		<OfferInbox
			offers={inbox.state.value}
			onDecide={handleDecide}
			practiceName={(page.data as { session: PracticeSession }).session.practiceName}
			timeZone={practiceTimeZone()}
			{engagementHref}
			bind:outcome
		/>
	{:else}
		<Skeleton variant="text" lines={3} label="Loading your offers" />
	{/if}
{/snippet}

<ListPage title="Your offers" {intro} {content} />
