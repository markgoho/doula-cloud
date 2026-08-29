<script lang="ts">
	/*
	 * The portal hub -- "Your care". What the Client's Engagement is, and
	 * the way to the two documents that belong to it.
	 *
	 * The message thread used to render here and now has its own route
	 * (#452): Messages is a destination in the portal's nav, and a nav item
	 * that scrolls to a section of another page is a nav item that lies.
	 * Push registration moved with it, up to `portal/(authenticated)/
	 * +layout.svelte`, so a Client who lands straight on her Contract is
	 * still registered for #61's alerts.
	 */
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Skeleton from '#lib/components/atoms/Skeleton.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	type Detail = {
		engagementId: string;
		practiceName: string;
		clientName: string;
		status: string;
		createdAt: string;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	onMount(async () => {
		const response = await apiFetchWithSession(
			`/api/portal/engagements/${page.params.engagementId}`
		);
		if (!response.ok) {
			error = await response.text();
			return;
		}

		detail = await response.json();
	});
</script>

{#snippet summary()}
	<DescriptionList
		items={[
			{ label: 'Status', value: detail!.status },
			{ label: 'Created', value: new Date(detail!.createdAt).toLocaleDateString() }
		]}
	/>
{/snippet}

{#snippet actions()}
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', {
			engagementId: page.params.engagementId!
		})}
		label="Birth plan"
	/>
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/contract', {
			engagementId: page.params.engagementId!
		})}
		label="Contract"
	/>
{/snippet}

{#if error}
	<Notice variant="error" message={error} />
{:else if detail}
	<!--
		Archetype D, ADR-0018 -- the same Template the staff Engagement page
		uses, which is the point of putting both on it. No sections and no
		contents region: what this page holds is the record's own summary
		and the way to its documents.
	-->
	<RecordDetail title={`Welcome to ${detail.practiceName}`} {summary} {actions} sections={[]} />
{:else}
	<Skeleton variant="text" lines={5} label="Loading your Engagement" />
{/if}
