<script lang="ts">
	/*
	 * PROTOTYPE (#1926) -- the Practice overview: the one screen that reads
	 * `display`, with section and card headings and an Offers table below.
	 */
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import { offers, PRACTICE_NAME } from './fixtures.js';

	type Offer = (typeof offers)[number];

	const offerColumns = [
		{ label: 'Client', accessor: (offer: Offer) => offer.client },
		{ label: 'Received', accessor: (offer: Offer) => offer.received },
		{ label: 'Status', accessor: (offer: Offer) => offer.status }
	];

	const rosterItems = [
		{ label: 'Doulas', value: '14' },
		{ label: 'Taking new Clients', value: '9' },
		{ label: 'Invitations still open', value: '2' }
	];

	const noop = () => {};
</script>

{#snippet primary()}
	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="Offers waiting on you" />
			<DataTable hasMore={false} columns={offerColumns} rows={offers} emptyMessage="No Offers waiting." />
		</stack-l>
	</section>

	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="Recent activity" />
			<Text
				text="Persephone Adeyemi-Wollstonecraft sent an invoice for $4,250.00 to Anne-Marie Ochieng-Whitfield."
				step="body-sm"
				tone="variant"
			/>
			<Text text="1 March 2027, 9:14am" step="meta" tone="muted" />
			<Text text="Renata Okonkwo accepted an Offer." step="body-sm" tone="variant" />
			<Text text="28 February 2027, 4:02pm" step="meta" tone="muted" />
		</stack-l>
	</section>
{/snippet}

{#snippet secondary()}
	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="card" text="Roster" />
			<DescriptionList items={rosterItems} />
			<Button label="Invite a doula" variant="secondary" onClick={noop} />
		</stack-l>
	</section>

	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="card" text="Payments" />
			<Badge label="Payouts enabled, next on 15 March 2027" variant="success" />
			<Text text="42 credits remaining." step="body-sm" tone="variant" />
		</stack-l>
	</section>
{/snippet}

{#snippet empty()}
	<Text text="No Clients yet." />
{/snippet}

<OverviewHub title={PRACTICE_NAME} {primary} {secondary} isEmpty={false} {empty} />
