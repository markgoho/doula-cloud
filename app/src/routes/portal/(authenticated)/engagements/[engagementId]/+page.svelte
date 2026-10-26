<script lang="ts">
	/*
	 * The portal hub -- "Your care" (#2007). Her care, in the drawing's
	 * order: what happens next, her Visits, her Birth Plan, her Contract,
	 * her Messages, what she still owes, and everything that has happened.
	 * No care phase, at any width: the drawings state none, and #293 found
	 * none true after a loss, so the BFF no longer sends one.
	 *
	 * The message thread used to render here and now has its own route
	 * (#452). Push registration lives in `portal/(authenticated)/
	 * +layout.svelte`, so a Client who lands straight on her Contract is
	 * still registered for #61's alerts.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetchWithSession } from '#lib/api.js';
	import { apiErrorMessage } from '#lib/apiErrorMessage.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import {
		clientActivityLedgerColumns,
		loadPortalActivityPage,
		type ActivityEntry
	} from '#lib/activityLedger.js';
	import {
		inReadingOrder,
		loadPortalVisitsPage,
		noVisitsMessage,
		portalVisitColumns,
		type PortalVisit
	} from '#lib/portalVisits.js';
	import { INVOICES_HEADING, OWED_HEADING, engagementLabel } from '#lib/clientRegister.js';
	import { totalToPayFact } from '#lib/clientInvoice.js';
	import {
		NEXT_HEADING,
		birthPlanCard,
		contractCard,
		messagesCard,
		nextVisit,
		whatHappensNext,
		type BirthPlanGlance,
		type CareCard,
		type ContractGlance,
		type LastMessage
	} from '#lib/yourCare.js';
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	type Detail = {
		engagementId: string;
		practiceName: string;
		clientName: string;
		/** Absent after a loss (#2003, as #293 decided), and on a
		 * postpartum-only Engagement, which never had one (#505). */
		dueDate?: string;
		offersBirthPlan: boolean;
		/** What she still owes on this care (#983); the Invoices route
		 * holds the rest. */
		totalToPayCents: number;
		contract?: ContractGlance;
		birthPlan?: BirthPlanGlance;
		lastMessage?: LastMessage;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	// #486 AC5: the same record-scoped ledger the staff Engagement page
	// gets, behind a closed disclosure.
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPortalActivityPage(apiFetchWithSession, page.params.engagementId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	let activityError = $state('');

	// #478: her own Visits, scheduled and past, in one cursor-paginated
	// stream ordered furthest-future first; `inReadingOrder` turns the
	// scheduled half around for display.
	const visits = new PaginatedList<PortalVisit>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPortalVisitsPage(apiFetchWithSession, page.params.engagementId!, cursor),
		failureMessage: 'Failed to load more visits'
	});
	let visitsError = $state('');

	onMount(async () => {
		const response = await apiFetchWithSession(`/api/portal/engagements/${page.params.engagementId}`);
		if (!response.ok) {
			error = await apiErrorMessage(response);
			return;
		}

		detail = await response.json();

		// Both lists' first page at once: neither read depends on the
		// other's answer, and each keeps its own failure.
		await Promise.all([
			(async () => {
				try {
					activity.reset(await loadPortalActivityPage(apiFetchWithSession, page.params.engagementId!, ''));
				} catch (error_) {
					activityError = error_ instanceof Error ? error_.message : 'Failed to load activity';
				}
			})(),
			(async () => {
				try {
					visits.reset(await loadPortalVisitsPage(apiFetchWithSession, page.params.engagementId!, ''));
				} catch (error_) {
					visitsError = error_ instanceof Error ? error_.message : 'Failed to load visits';
				}
			})()
		]);
	});

	const next = $derived(whatHappensNext(nextVisit(visits.items), detail?.dueDate));

	const routes = $derived.by(() => {
		const engagementId = page.params.engagementId!;
		return {
			birthPlan: resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', { engagementId }),
			contract: resolve('/portal/(authenticated)/engagements/[engagementId]/contract', { engagementId }),
			messages: resolve('/portal/(authenticated)/engagements/[engagementId]/messages', { engagementId }),
			invoices: resolve('/portal/(authenticated)/engagements/[engagementId]/invoices', { engagementId })
		};
	});
</script>

<!--
	Under the heading: whose care this is, then "What happens next" -- the
	next Visit as a sentence, and the due date where there is one. A
	labeled region of its own, so a screen reader can reach it by name;
	its heading is the drawing's small label, not a section heading.
-->
{#snippet summary()}
	<div class="summary">
		<div class="practice"><Text text={page.data.practiceName ?? ''} tone="variant" /></div>
		<section class="next" aria-labelledby="what-happens-next"><!-- spelling:ignore: aria-labelledby is an ARIA attribute name -->
			<Heading level={2} variant="card" text={NEXT_HEADING} id="what-happens-next" />
			<Text text={next.lead} step="label" />
			{#if next.detail}
				<Text text={next.detail} tone="variant" />
			{/if}
		</section>
	</div>
{/snippet}

<!-- One card per document: the line that says where it stands, under
     the way to it (DoorLink, #1645). No link where there is nothing to
     open yet. -->
{#snippet card(content: CareCard, href: string)}
	<div class="column">
		{#if content.label}
			<DoorLink {href} label={content.label} description={content.description} />
		{:else}
			<Text text={content.description} />
		{/if}
	</div>
{/snippet}

<!--
	#478: "Your visits" is the Client register's own heading (GLOSSARY.md's
	Visit entry). Open, unlike the ledger: this is the answer she came for.
	No Visit type is rendered, in any wording -- see portalVisits.ts.
-->
{#snippet visitsSection()}
	{#if visitsError}
		<Notice variant="error" message={visitsError} />
	{/if}
	<DataTable
		columns={portalVisitColumns()}
		rows={inReadingOrder(visits.items)}
		hasMore={visits.hasMore}
		onLoadMore={() => visits.loadMore()}
		isLoadingMore={visits.isLoadingMore}
		loadMoreError={visits.loadMoreError}
		emptyMessage={noVisitsMessage}
	/>
{/snippet}

{#snippet birthPlanSection()}
	{@render card(birthPlanCard(detail?.birthPlan), routes.birthPlan)}
{/snippet}

{#snippet contractSection()}
	{@render card(contractCard(detail?.contract, page.data.practiceName ?? ''), routes.contract)}
{/snippet}

{#snippet messagesSection()}
	{@render card(messagesCard(detail?.lastMessage), routes.messages)}
{/snippet}

<!--
	#983, #2038: the "Total to pay" row is the Invoices page's own, so the
	figure reads one way in both places, $0.00 included (#982). Paying
	happens on an Invoice's own page (#983); how a Client pays is #297's.
-->
{#snippet owedSection()}
	<div class="column">
		<SummaryList rows={[totalToPayFact(detail?.totalToPayCents ?? 0, 'usd')]} />
		<DoorLink
			href={routes.invoices}
			label={INVOICES_HEADING}
			description="Each Invoice for your care, and the way to pay one that is due."
		/>
	</div>
{/snippet}

<!--
	#486 AC5: GLOSSARY.md's own words for this to a Client, behind a
	closed disclosure (the design brief's #433 amendment). #708: every
	row's event text is the register's fixed phrase for that action.
-->
{#snippet activitySection()}
	{#if activityError}
		<Notice variant="error" message={activityError} />
	{/if}
	<DataTable
		disclosure="Show what has happened"
		columns={clientActivityLedgerColumns()}
		rows={activity.items}
		hasMore={activity.hasMore}
		onLoadMore={() => activity.loadMore()}
		isLoadingMore={activity.isLoadingMore}
		loadMoreError={activity.loadMoreError}
		emptyMessage="Nothing has happened yet."
	/>
{/snippet}

<!--
	Archetype D, ADR-0018. #296: the heading is GLOSSARY.md's own Client
	word for an Engagement, a literal that consults nothing, so it is true
	for every Client, after a loss included. #310: `serviceName` is what
	`PageTitle` folds into `<title>`, which SvelteKit's navigation
	announcer reads aloud, so two Engagements at one Practice announce
	different words.

	The Birth Plan section follows `offersBirthPlan`, the same one answer
	the nav item reads (#311, #294): where none is offered she meets no
	mention of it.
-->
<RecordDetail
	title="Your care"
	serviceName={engagementLabel({ practiceName: page.data.practiceName, createdAt: page.data.createdAt })}
	{summary}
	sections={detail
		? [
				{ heading: 'Your visits', content: visitsSection },
				...(detail.offersBirthPlan ? [{ heading: 'Your birth plan', content: birthPlanSection }] : []),
				{ heading: 'Your contract', content: contractSection },
				{ heading: 'Messages', content: messagesSection },
				{ heading: OWED_HEADING, content: owedSection },
				{ heading: 'Everything that has happened', content: activitySection }
			]
		: []}
	loading={detail || error ? undefined : 'Loading your care'}
	loadError={error || undefined}
/>

<style>
	@layer components {
		.summary {
			display: flex;
			flex-direction: column;
			gap: var(--space-8);
			margin-block-start: var(--space-2);
		}

		/* The Practice's name is her Practice's own data, of any length. */
		.practice {
			overflow-wrap: anywhere;
		}

		/* The drawing's card: one bordered block, as wide as the cards
		   below it. */
		.next {
			display: flex;
			flex-direction: column;
			gap: var(--space-3);
			max-inline-size: var(--measure);
			padding: var(--space-5);
			border: var(--border-thin) solid var(--color-outline-variant);
			border-radius: var(--radius);
			background-color: var(--color-surface-bright);
		}

		/* One column for every section's card, the drawing's width: a
		   card that ran the full 1440px would put its one line a long way
		   from its edge. */
		.column {
			display: flex;
			flex-direction: column;
			gap: var(--space-4);
			max-inline-size: var(--measure);
		}
	}
</style>
