<script lang="ts">
	import ListPage from '#lib/components/templates/ListPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
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

	let state = $state<'content' | 'loading' | 'loadError'>('content');
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

<div class="controls">
	<Button label="Content" variant="secondary" size="sm" onClick={() => (state = 'content')} />
	<Button label="Loading" variant="secondary" size="sm" onClick={() => (state = 'loading')} />
	<Button label="Load error" variant="secondary" size="sm" onClick={() => (state = 'loadError')} />
</div>

{#if state === 'loading'}
	<ListPage title="Staff" {intro} {actions} {content} loading="Loading Staff" />
{:else if state === 'loadError'}
	<ListPage title="Staff" {intro} {actions} {content} loadError="Failed to load Staff" />
{:else}
	<ListPage title="Staff" {intro} {actions} {content} />
{/if}

<style>
	@layer components {
		/* Not part of the Template -- a switch so all three states of the
		   page can be seen without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
