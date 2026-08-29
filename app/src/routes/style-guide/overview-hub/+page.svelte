<script lang="ts">
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Button from '#lib/components/atoms/Button.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	interface Offer {
		client: string;
		received: string;
		status: string;
	}

	const offerColumns = [
		{ label: 'Client', accessor: (offer: Offer) => offer.client },
		{ label: 'Received', accessor: (offer: Offer) => offer.received },
		{ label: 'Status', accessor: (offer: Offer) => offer.status }
	];

	const offers: Offer[] = [
		{ client: 'Ada Lovelace', received: '2 March', status: 'Awaiting reply' },
		{ client: 'Grace Hopper', received: '28 February', status: 'Awaiting reply' },
		{ client: 'Katherine Johnson', received: '26 February', status: 'Accepted' }
	];

	const rosterItems = [
		{ label: 'Doulas', value: '14' },
		{ label: 'Taking clients', value: '9' },
		{ label: 'Invitations open', value: '2' }
	];

	const noop = () => {};

	let isEmpty = $state(false);
</script>

{#snippet primary()}
	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="Offers waiting on you" />
			<DataTable columns={offerColumns} rows={offers} emptyMessage="No Offers waiting." />
		</stack-l>
	</section>

	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="section" text="Recent activity" />
			<Text
				text="Priya Raman sent an invoice to Grace Hopper — 1 March, 9:14am."
				step="body-sm"
				tone="variant"
			/>
			<Text
				text="Ada Lovelace accepted an Offer — 28 February, 4:02pm."
				step="body-sm"
				tone="variant"
			/>
		</stack-l>
	</section>
{/snippet}

{#snippet secondary()}
	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="card" text="Roster" />
			<DescriptionList items={rosterItems} />
		</stack-l>
	</section>

	<section>
		<stack-l space="var(--space-4)">
			<Heading level={2} variant="card" text="Payments" />
			<Badge label="Payouts enabled" variant="success" />
			<Text text="42 credits remaining." step="body-sm" tone="variant" />
		</stack-l>
	</section>
{/snippet}

{#snippet empty()}
	<stack-l space="var(--space-4)">
		<Text text="No clients yet. Invite your first doula, or add a client yourself." />
		<cluster-l space="var(--space-3)">
			<Button label="Invite a doula" onClick={noop} />
			<Button label="Add a client" variant="secondary" onClick={noop} />
		</cluster-l>
	</stack-l>
{/snippet}

<div class="controls">
	<Button
		label={isEmpty ? 'Show the populated hub' : 'Show the empty hub'}
		variant="secondary"
		size="sm"
		onClick={() => (isEmpty = !isEmpty)}
	/>
</div>

<OverviewHub title="Willow Birth Collective" {primary} {secondary} {isEmpty} {empty} />

<style>
	@layer components {
		/* Not part of the Template -- a switch so both required states of the
		   page can be seen without editing this file. */
		.controls {
			padding: var(--space-3) var(--space-4);
			border-block-end: var(--border-thin) solid var(--color-outline-variant);
			background-color: var(--color-surface-container);
		}
	}
</style>
