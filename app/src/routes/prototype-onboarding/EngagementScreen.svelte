<script lang="ts">
	/*
	 * PROTOTYPE -- #1496, screen 5: the Engagement's page, where the flow
	 * ends (#1516 position 4, #1611). First Value. The status message says
	 * that the work started. Her other details are added from here or from
	 * her record with "Add (name)'s details" (#1610). The sections are the
	 * real page's headings with their empty states; the real page is 2,000
	 * lines and is not rebuilt.
	 */
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { NO_DOULA, SELF, nowhere, prototype } from './model.svelte.js';

	const name = $derived(prototype.clientName);
	const engagement = $derived(prototype.engagement!);
	const doula = $derived(prototype.doulaLabel(engagement.doula));
	const isBirth = $derived(engagement.kind === 'birth');
	const dueDate = $derived(
		engagement.dueDate
			? new Date(`${engagement.dueDate}T12:00:00`).toLocaleDateString('en-US', { dateStyle: 'long' })
			: ''
	);
	const attached = $derived(
		engagement.doula === SELF
			? `${prototype.ownerName} started this Engagement as its Doula`
			: `${prototype.ownerName} put ${doula} on this Engagement as the Doula`
	);
</script>

{#snippet summary()}
	<stack-l space="var(--space-4)">
		<Notice variant="status" message={`Work with ${name} started.`} />
		<DescriptionList
			items={[
				{ label: 'Kind of work', value: isBirth ? 'Birth' : 'Postpartum' },
				{ label: 'Due date', value: dueDate || 'Not given' },
				{ label: 'Doula', value: doula },
				{ label: 'Status', value: 'Intake' }
			]}
		/>
		<Link href={nowhere('details')} label={`Add ${name}'s details`} />
	</stack-l>
{/snippet}

{#snippet onCall()}
	{#if engagement.doula === NO_DOULA}
		<Text text="Nobody is on call for this birth, because no Doula is on the Engagement yet." tone="variant" />
	{:else if isBirth && engagement.dueDate}
		<Text text={`${doula} is on call for this birth, around the due date, ${dueDate}.`} />
	{:else}
		<Text text="No On-call window. Postpartum work has none." tone="variant" />
	{/if}
{/snippet}

{#snippet visits()}
	<Text text="No Visits yet." tone="variant" />
	<Link href={nowhere('visit')} label="Add a Visit" variant="secondary" />
{/snippet}

{#snippet birthPlan()}
	<Text text="No Birth Plan yet." tone="variant" />
	<Link href={nowhere('birth-plan')} label="Start the Birth Plan" variant="secondary" />
{/snippet}

{#snippet contract()}
	<Text text="No Contract yet." tone="variant" />
	<Link href={nowhere('contract')} label="Write a Contract" variant="secondary" />
{/snippet}

{#snippet messages()}
	<Notice
		variant="info"
		message="This Client has no email address on file. Add one before sending a portal invite."
	/>
{/snippet}

{#snippet activity()}
	<DescriptionList
		items={[
			{ label: 'Today', value: `${prototype.ownerName} added ${name} as a Client` },
			{ label: 'Today', value: `${prototype.ownerName} started this Engagement. It used 1 Credit.` },
			...(engagement.doula === NO_DOULA
				? []
				: [{ label: 'Today', value: attached }])
		]}
	/>
{/snippet}

<RecordDetail
	title={name}
	{summary}
	isContentsShown
	sections={[
		{ heading: 'On call', content: onCall },
		{ heading: 'Visits', content: visits },
		{ heading: 'Birth Plan', content: birthPlan },
		{ heading: 'Contract', content: contract },
		{ heading: 'Messages', content: messages },
		{ heading: 'Activity', content: activity }
	]}
/>
