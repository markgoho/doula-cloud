<script lang="ts">
	/*
	 * PROTOTYPE -- #1496: the Client's record, where the second action on
	 * the Start work form lands (#1611). The Client is saved and nothing is
	 * started. A second Engagement also starts from here, as today.
	 */
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { nowhere, prototype, to } from './model.svelte.js';

	const name = $derived(prototype.clientName);
	const client = $derived(prototype.client!);
</script>

{#snippet summary()}
	<stack-l space="var(--space-4)">
		{#if prototype.engagement}
			<Link href={to('engagement')} label={`Open the Engagement with ${name}`} />
		{:else}
			<Text text={`${name} is saved. No work is started, and no Credit is used.`} />
			<Link href={to('start')} label={`Start new work with ${name}`} />
		{/if}
		<Link href={nowhere('details')} label={`Add ${name}'s details`} variant="secondary" />
	</stack-l>
{/snippet}

{#snippet who()}
	<DescriptionList
		items={[
			{ label: 'Given name', value: client.givenName },
			{ label: 'Family name', value: client.familyName },
			{ label: 'Preferred name', value: client.preferredName },
			{ label: 'Date of birth', value: '' }
		]}
	/>
{/snippet}

{#snippet reach()}
	<DescriptionList
		items={[
			{ label: 'Email', value: '' },
			{ label: 'Phone', value: '' }
		]}
	/>
{/snippet}

{#snippet engagements()}
	{#if prototype.engagement}
		<Link href={to('engagement')} label={prototype.engagement.kind === 'birth' ? 'Birth, Intake' : 'Postpartum, Intake'} />
	{:else}
		<Text text="No Engagements yet." tone="variant" />
	{/if}
{/snippet}

{#snippet history()}
	<DescriptionList items={[{ label: 'Today', value: `${prototype.ownerName} added ${name} as a Client` }]} />
{/snippet}

<RecordDetail
	title={name}
	{summary}
	isContentsShown
	sections={[
		{ heading: `Who ${name} is`, content: who },
		{ heading: `How to reach ${name}`, content: reach },
		{ heading: 'Engagements', content: engagements },
		{ heading: 'History', content: history }
	]}
/>
