<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { apiFetchWithSession } from '#lib/api.js';
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import { resolve } from '$app/paths';
	import type { PracticeSession } from '../+layout.js';
	import { decideOffer, loadInbox, type DecisionOutcome, type Offer } from '#lib/offer.js';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';

	let offers = $state<Offer[]>([]);
	let error = $state('');
	let isLoaded = $state(false);

	async function refresh() {
		error = '';
		try {
			offers = await loadInbox(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load your offers';
		} finally {
			isLoaded = true;
		}
	}

	// The list is reloaded rather than patched in place: accepting one
	// Offer supersedes every other open Offer on the same Engagement, and
	// those are rows this list may well be showing.
	async function handleDecide(offerId: string, action: 'accept' | 'decline') {
		const decision = await decideOffer(apiFetchWithSession, page.params.practiceId!, offerId, action);
		await refresh();
		return decision;
	}

	let outcome = $state<DecisionOutcome>();

	const engagementHref = (engagementId: string) =>
		resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId
		});

	onMount(refresh);
</script>

{#snippet intro()}
	<Text
		text="Work you have been offered. Accepting puts you on the birth; declining is final for that Offer, and the Practice may ask you again."
		tone="variant"
	/>
{/snippet}

{#snippet content()}
	{#if error}
		<Notice message={error} variant="error" />
	{/if}

	{#if isLoaded}
		<OfferInbox
			{offers}
			onDecide={handleDecide}
			practiceName={(page.data as { session: PracticeSession }).session.practiceName}
			{engagementHref}
			bind:outcome
		/>
	{:else}
		<Skeleton variant="text" lines={3} label="Loading your offers" />
	{/if}
{/snippet}

<ListPage title="Your offers" {intro} {content} />
