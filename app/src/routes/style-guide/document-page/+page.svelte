<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		shown?: 'content' | 'empty' | 'loading' | 'loadError';
	}

	/*
	 * The other states this page renders (#1638, ADR-0025), each its own
	 * subject for the continuum check and the drag surface. Each one
	 * replaces the content, so one mount cannot hold two of them.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Document page, empty', props: { shown: 'empty' } },
		{ name: 'Document page, loading', props: { shown: 'loading' } },
		{ name: 'Document page, with a load error', props: { shown: 'loadError' } }
	];
</script>

<script lang="ts">
	import DocumentPage from '#lib/components/templates/DocumentPage.svelte';
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

	let { shown = 'content' }: Properties = $props();
</script>

{#snippet content()}
	<DescriptionList items={facts} />
	<Text
		text="Pay this Invoice by check or bank transfer to Riverside Doulas & Postpartum Care Collective, and quote A4B2-0011-PERSEPHONE."
	/>
{/snippet}

<!-- The title is the Invoices heading until the Invoice "arrives", the way
     the portal Invoice route passes it. An Invoice has no empty state, so
     the empty variant shows the Contract's instead. -->
<DocumentPage
	title={{ content: title, empty: 'Contract', loading: 'Invoices', loadError: 'Invoices' }[shown]}
	backHref="#"
	backLabel="Back to Invoices"
	{content}
	loading={shown === 'loading' ? 'Loading Invoice' : undefined}
	loadError={shown === 'loadError' ? 'Failed to load Invoice' : undefined}
	empty={shown === 'empty' ? 'No Contract has been sent for your care yet.' : undefined}
/>
