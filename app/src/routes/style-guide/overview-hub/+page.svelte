<script module lang="ts">
	import type { DemoVariant } from '../drag-surface/dragSurface.js';

	interface Properties {
		isEmpty?: boolean;
	}

	/*
	 * The other state this page renders (#1638, ADR-0025). The empty hub
	 * replaces the populated one, so one mount cannot hold the two: it is
	 * its own subject, swept by the continuum check and offered on the drag
	 * surface under this name.
	 */
	export const variants: readonly DemoVariant<Properties>[] = [
		{ name: 'Overview hub, empty', props: { isEmpty: true } }
	];
</script>

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

	/*
	 * The longest realistic value, not a representative one (ADR-0025): the
	 * hub's title is a Practice's registered name, and the two regions carry
	 * Client names and activity lines in full -- which is what decides
	 * whether the secondary region still fits beside the primary one.
	 */
	const offers: Offer[] = [
		{
			client: 'Anne-Marie Ochieng-Whitfield',
			received: '2 March 2027',
			status: 'Awaiting reply'
		},
		{
			client: 'Persephone Adeyemi-Wollstonecraft',
			received: '28 February 2027',
			status: 'Awaiting reply'
		},
		{ client: 'Renata Chiamaka Okonkwo-Adeyemi', received: '26 February 2027', status: 'Accepted' }
	];

	const rosterItems = [
		{ label: 'Doulas', value: '14' },
		{ label: 'Taking new Clients', value: '9' },
		{ label: 'Invitations still open', value: '2' }
	];

	const noop = () => {};

	let { isEmpty = false }: Properties = $props();
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
				text="Persephone Adeyemi-Wollstonecraft sent an invoice for $4,250.00 to Anne-Marie Ochieng-Whitfield — 1 March 2027, 9:14am."
				step="body-sm"
				tone="variant"
			/>
			<Text
				text="Renata Chiamaka Okonkwo-Adeyemi accepted an Offer — 28 February 2027, 4:02pm."
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
			<Badge label="Payouts enabled, next on 15 March 2027" variant="success" />
			<Text text="42 credits remaining." step="body-sm" tone="variant" />
		</stack-l>
	</section>
{/snippet}

{#snippet empty()}
	<stack-l space="var(--space-4)">
		<Text
			text="No Clients yet. Invite your first Doula, or add a Client yourself and the hub fills in."
		/>
		<cluster-l space="var(--space-3)">
			<Button label="Invite a doula" onClick={noop} />
			<Button label="Add a client" variant="secondary" onClick={noop} />
		</cluster-l>
	</stack-l>
{/snippet}

<OverviewHub title="Highland Midwifery &amp; Birth Support Collective of Western New York" {primary} {secondary} {isEmpty} {empty} />
