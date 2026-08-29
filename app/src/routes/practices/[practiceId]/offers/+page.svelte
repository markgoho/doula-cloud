<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiFetchWithSession } from '#lib/api.js';
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import { decideOffer, loadInbox, type Offer } from '#lib/offer.js';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';

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
		await decideOffer(apiFetchWithSession, page.params.practiceId!, offerId, action);
		await refresh();
	}

	onMount(refresh);
</script>

<Heading level={1} text="Your offers" />
<Text
	text="Work you have been offered. Accepting puts you on the birth; declining is final for that offer, and the practice may ask you again."
	tone="variant"
/>

{#if error}
	<Notice message={error} variant="error" />
{/if}

{#if isLoaded}
	<OfferInbox {offers} onDecide={handleDecide} />
{/if}
