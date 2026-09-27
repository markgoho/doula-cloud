<script lang="ts">
	/*
	 * PROTOTYPE -- #1502. Stand-in content for the screen a person is on
	 * when they decide to send feedback, so the banner is judged at real
	 * density. Not the real pages: those need a session and a live API.
	 */
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';

	interface Properties {
		screenKey: string;
		title: string;
	}

	let { screenKey, title }: Properties = $props();

	const clients = [
		['Alex Rivera', 'Due December 12, 2026', 'Birth and postpartum'],
		['Sam Okafor', 'Due October 3, 2026', 'Birth'],
		['Priya Natarajan', 'Born September 14, 2026', 'Postpartum'],
		['Morgan Lee', 'Due January 20, 2027', 'Birth and postpartum'],
		['Dana Whitfield', 'Due November 8, 2026', 'Birth']
	];
</script>

<div class="host">
	<Heading level={1} text={title} />
	{#if screenKey === 'clients'}
		<Text text="12 active Clients" tone="muted" />
		<ul class="rows">
			{#each clients as [name, due, kind] (name)}
				<li><strong>{name}</strong><span>{due}</span><span>{kind}</span></li>
			{/each}
		</ul>
	{:else if screenKey === 'client'}
		<DescriptionList
			items={[
				{ label: 'Due date', value: 'December 12, 2026' },
				{ label: 'Doula', value: 'Jordan Blake' },
				{ label: 'Care', value: 'Birth and postpartum' },
				{ label: 'Contract', value: 'Signed September 2, 2026' },
				{ label: 'Invoices', value: '1 paid, 1 not yet paid' }
			]}
		/>
	{:else if screenKey === 'care'}
		<Text text="Jordan Blake is your doula." />
		<DescriptionList
			items={[
				{ label: 'Due date', value: 'December 12, 2026' },
				{ label: 'Next visit', value: 'October 4, 2026 at 10:00 AM' },
				{ label: 'Contract', value: 'Signed' },
				{ label: 'Birth plan', value: 'Started' }
			]}
		/>
	{:else}
		<Text text="You signed this Contract on September 2, 2026." />
		<Text
			text="1. Services. Your doula will provide continuous support during labor and birth, two prenatal visits, and two postpartum visits…"
			measure
		/>
	{/if}
</div>

<style>
	.host {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	.rows {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.rows li {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-1) var(--space-4);
		padding-block: var(--space-3);
		border-block-end: var(--border-thin) solid var(--color-outline-variant);
	}

	.rows strong {
		flex: 1 1 12rem;
	}

	.rows span {
		color: var(--color-on-surface-variant);
		font-size: var(--text-body-sm-size);
	}
</style>
