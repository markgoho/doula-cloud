<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		shown?: 'content' | 'loading' | 'loadError';
	}

	/*
	 * The other states this page renders (#1638, ADR-0025), each its own
	 * subject for the continuum check and the drag surface. Each one
	 * replaces the content, so one mount cannot hold two of them.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'List page, loading', props: { shown: 'loading' } },
		{ name: 'List page, with a load error', props: { shown: 'loadError' } }
	];
</script>

<script lang="ts">
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface StaffRow {
		name: string;
		email: string;
		roles: string;
	}

	const columns = [
		{ label: 'Name', accessor: (row: StaffRow) => row.name },
		{ label: 'Email', accessor: (row: StaffRow) => row.email },
		{ label: 'Roles', accessor: (row: StaffRow) => row.roles }
	];

	/*
	 * The longest realistic values, not representative ones (ADR-0025): the
	 * list's rows carry names and emails in full, which is what decides
	 * whether a column stretches past what it needs once the frame is
	 * uncapped.
	 */
	const rows: StaffRow[] = [
		{
			name: 'Anne-Marie Ochieng-Whitfield',
			email: 'anne-marie.ochieng-whitfield@example.com',
			roles: 'owner, admin'
		},
		{
			name: 'Persephone Adeyemi-Wollstonecraft',
			email: 'persephone.adeyemi-wollstonecraft@example.com',
			roles: 'doula'
		}
	];

	let { shown = 'content' }: Properties = $props();
</script>

{#snippet intro()}
	<Text
		text="Work states are self-reported by each person and are not verified. They set how much sales tax your practice pays on credits."
	/>
{/snippet}

{#snippet actions()}
	<Link href="#" label="Invite a Staff member" />
{/snippet}

{#snippet content()}
	<DataTable {columns} {rows} emptyMessage="No Staff yet." />
{/snippet}

{#if shown === 'loading'}
	<ListPage title="Staff" {intro} {actions} {content} loading="Loading Staff" />
{:else if shown === 'loadError'}
	<ListPage title="Staff" {intro} {actions} {content} loadError="Failed to load Staff" />
{:else}
	<ListPage title="Staff" {intro} {actions} {content} />
{/if}
