<script lang="ts">
	/*
	 * PROTOTYPE -- #1496, screen 2: the Practice overview on `OverviewHub`.
	 *
	 * Empty (no Client): #1515's words (#1599), the Credits sentence (#1612),
	 * and one link, which opens the name question and not the search (#1609).
	 * Nothing here names Stripe, email verification or a second factor
	 * (#1492, #1495).
	 *
	 * Three variants of ONE thing, the place of the Credits sentence:
	 *   A  its own paragraph, below the words and before the link (#1612)
	 *   B  in the same paragraph as the words
	 *   C  after the link, in the smaller text step
	 *
	 * After First Value the hub is not empty; the "Getting paid" card has
	 * #1495's words. That state is after the route and is here only so that
	 * the Overview link has a true target.
	 */
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import { emptyPracticeCredits, words } from './fixtures.js';
	import { nowhere, prototype, to } from './model.svelte.js';

	const credits = $derived(emptyPracticeCredits(prototype.balance));
</script>

{#snippet empty()}
	<stack-l space="var(--space-4)">
		{#if prototype.creditsPlace === 'B'}
			<Text text={`${words.emptyPractice} ${credits}`} measure />
		{:else}
			<Text text={words.emptyPractice} measure />
		{/if}
		{#if prototype.creditsPlace === 'A'}
			<Text text={credits} measure />
		{/if}
		<Link href={to('name')} label="Add your first Client" />
		{#if prototype.creditsPlace === 'C'}
			<Text text={credits} step="body-sm" tone="variant" measure />
		{/if}
	</stack-l>
{/snippet}

{#snippet primary()}
	<Heading level={2} variant="section" text="Offers awaiting your answer" />
	<Text text="No Offers are waiting for you." tone="muted" />
	<Heading level={2} variant="section" text="Clients waiting on a reply" />
	<Text text="Nobody is waiting on a reply." tone="muted" />
	<Link href={to('engagement')} label={`Open the Engagement with ${prototype.clientName}`} />
{/snippet}

{#snippet secondary()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Credits" />
			<DescriptionList items={[{ label: 'Balance', value: String(prototype.balance) }]} />
			<Link href={nowhere('credits')} label="Buy credits" variant="secondary" />
		</stack-l>
	</section>
	<!-- #1495: the first ask to connect Stripe, after First Value. -->
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Getting paid" />
			<Text
				text="Clients cannot pay you by card yet. To take card payments, connect Stripe. It takes about fifteen minutes, and then Stripe reviews your details. If you collect payment yourself, you do not need Stripe."
			/>
			<Link href={nowhere('payments')} label="Set up card payments" variant="secondary" />
		</stack-l>
	</section>
{/snippet}

<OverviewHub
	title={`Welcome to ${prototype.account?.practiceName ?? ''}`}
	isEmpty={prototype.client === undefined}
	{primary}
	secondary={prototype.client ? secondary : undefined}
	{empty}
/>
