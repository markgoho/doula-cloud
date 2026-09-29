<script lang="ts">
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	/*
	 * The longest realistic values, not representative ones (ADR-0025): a
	 * hyphenated reference in the title and a Practice's full name in the
	 * payment sentence are what decide whether the frame holds at 320px.
	 */
	const title = 'Invoice A4B2-0011-PERSEPHONE';
	const facts = [
		{ label: 'Amount', value: '$12,345.67' },
		{ label: 'Status', value: 'Open' },
		{ label: 'Due', value: 'Mar 1, 2027' },
		{ label: 'Billed to', value: 'Persephone Adeyemi-Wollstonecraft' }
	];

	let state = $state<'content' | 'empty' | 'loading' | 'loadError'>('content');
</script>

{#snippet content()}
	<DescriptionList items={facts} />
	<Text
		text="Pay this Invoice by check or bank transfer to Riverside Doulas & Postpartum Care Collective, and quote A4B2-0011-PERSEPHONE."
	/>
{/snippet}

<div class="controls">
	<Button label="Content" variant="secondary" size="sm" onClick={() => (state = 'content')} />
	<Button label="Empty" variant="secondary" size="sm" onClick={() => (state = 'empty')} />
	<Button label="Loading" variant="secondary" size="sm" onClick={() => (state = 'loading')} />
	<Button label="Load error" variant="secondary" size="sm" onClick={() => (state = 'loadError')} />
</div>

{#if state === 'empty'}
	<DocumentPage
		title="Contract"
		backHref="#"
		{content}
		empty="No Contract has been sent for your care yet."
	/>
{:else if state === 'loading'}
	<DocumentPage title="Invoices" backHref="#" backLabel="Back to Invoices" {content} loading="Loading Invoice" />
{:else if state === 'loadError'}
	<DocumentPage title="Invoices" backHref="#" backLabel="Back to Invoices" {content} loadError="Failed to load Invoice" />
{:else}
	<DocumentPage {title} backHref="#" backLabel="Back to Invoices" {content} />
{/if}

<style>
	@layer components {
		/* Not part of the Template -- a switch so all four states of the
		   page can be seen without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
