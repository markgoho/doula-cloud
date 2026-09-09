<script lang="ts">
	/*
	 * The portal hub -- "Your care". What the Client's Engagement is, and
	 * the way to the two documents that belong to it.
	 *
	 * The message thread used to render here and now has its own route
	 * (#452): Messages is a destination in the portal's nav, and a nav item
	 * that scrolls to a section of another page is a nav item that lies.
	 * Push registration moved with it, up to `portal/(authenticated)/
	 * +layout.svelte`, so a Client who lands straight on her Contract is
	 * still registered for #61's alerts.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiErrorMessage, apiFetchWithSession } from '#lib/api.js';
	import { formatCalendarDay } from '#lib/dates.js';
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
	import { engagementLabel, engagementStatusLabel } from '#lib/clientRegister.js';
	import Link from '#lib/components/atoms/Link.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import RecordDetail from '#lib/components/templates/RecordDetail.svelte';

	type Detail = {
		engagementId: string;
		practiceName: string;
		clientName: string;
		status: string;
		dueDate?: string;
		offersBirthPlan: boolean;
	};

	let detail = $state<Detail | undefined>();
	let error = $state('');

	// #486 AC5: the same record-scoped ledger the staff Engagement page
	// gets, behind a closed disclosure -- the design brief's own placement
	// decision for the Client portal.
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPortalActivityPage(apiFetchWithSession, page.params.engagementId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	let activityError = $state('');

	// #478: her own Visits, scheduled and past, in one cursor-paginated
	// stream ordered furthest-future first -- so the two groups need no
	// second request. `inReadingOrder` turns the scheduled half around for
	// display; its own doc comment says why the two orders differ.
	const visits = new PaginatedList<PortalVisit>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPortalVisitsPage(apiFetchWithSession, page.params.engagementId!, cursor),
		failureMessage: 'Failed to load more visits'
	});
	let visitsError = $state('');

	onMount(async () => {
		const response = await apiFetchWithSession(
			`/api/portal/engagements/${page.params.engagementId}`
		);
		if (!response.ok) {
			error = await apiErrorMessage(response);
			return;
		}

		detail = await response.json();

		// Both sections' first page at once, not one behind the other: the
		// section a Client came for is the open one at the top, and
		// awaiting the ledger's request first would make it wait on a
		// table that is closed when the page loads. Neither read depends
		// on the other's answer, and each keeps its own failure.
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

	/** The summary row's own facts (#505). `dueDate` is left out of the
	 * array entirely, rather than shown with a placeholder, when null --
	 * ADR-0017's postpartum-only Engagement genuinely has none, and a
	 * "Due date" row reading "Not given" would tell a Client something is
	 * missing from her own record rather than that nothing was ever due. */
	function summaryItems(d: Detail): { label: string; value: string }[] {
		const items = [{ label: 'Status', value: engagementStatusLabel(d.status) }];
		if (d.dueDate) {
			items.push({ label: 'Due date', value: formatCalendarDay(d.dueDate) });
		}
		return items;
	}
</script>

{#snippet summary()}
	<DescriptionList items={summaryItems(detail!)} />
{/snippet}

{#snippet actions()}
	<!--
		The same one answer the nav item reads -- #311's kind half and
		#294's living-or-expected-baby half, derived server-side and never
		recomputed here.
	-->
	{#if detail?.offersBirthPlan}
		<Link
			href={resolve('/portal/(authenticated)/engagements/[engagementId]/birth-plan', {
				engagementId: page.params.engagementId!
			})}
			label="Birth plan"
		/>
	{/if}
	<Link
		href={resolve('/portal/(authenticated)/engagements/[engagementId]/contract', {
			engagementId: page.params.engagementId!
		})}
		label="Contract"
	/>
{/snippet}

<!--
	#478: CONTEXT.md's Visit entry gives this section its heading -- the
	Client register's own "Your visits", her phrasing ("when she comes
	over", "when Maya came"), never the team's "Visits".

	It is the FIRST section on the page, not the second. The drawing puts
	it directly under "What happens next", and that section does not
	exist on this route -- nothing above the sections here but the
	record's own summary and the two document links -- so directly under
	it means first. Ahead of the Activity ledger either way: both
	journeys put "when is someone coming" among the first things she
	wants, and the ledger is what already happened.

	Open, unlike the ledger's closed disclosure: the ledger sits behind
	one because it is a record to consult, and this is the answer she came
	for.

	No Visit type is rendered, in any wording -- see portalVisits.ts, and
	CONTEXT.md's Visit entry, for why.
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

<!--
	#486 AC5: CONTEXT.md's own words for this to a Client -- "Everything
	that has happened" -- as the section heading. GOV.UK's Details guidance
	is that a summary names what it reveals rather than a bare "Show" with
	no subject, so the disclosure's own toggle text repeats "what has
	happened" instead of the heading's exact words. Behind a closed
	disclosure, per the design brief's #433 amendment for the Client
	portal.

	#708: the Client's own column set, not the staff one -- every row's
	event text is the register's fixed phrase for that action, never the
	raw `activity.EngagementAction` the two staff surfaces show.
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
	Archetype D, ADR-0018 -- the same Template the staff Engagement page
	uses, which is the point of putting both on it. #486 gives this page
	its first section: everything else here is the record's own summary
	and the way to its documents.
-->
<!--
	#310: `serviceName` is what `PageTitle` folds into `<title>`, and
	SvelteKit's own client-side navigation announcer reads that title
	aloud after every route change -- it is how "a change of Engagement
	is announced" gets met without a bespoke live region. `engagementLabel`
	rather than the bare Practice name so switching between two
	Engagements at the same Practice still announces something that
	differs; the `<h1>` stays the same words on every Engagement, since a
	distinguishing fact belongs in what is spoken, not necessarily in what
	is read.
-->
<!--
	#296: the heading is CONTEXT.md's own Client word for an Engagement --
	the Engagement entry's `_Client says_:` line reads `my care ("Your
	care" as a heading)`. It was `Welcome to {practiceName}`, a first-visit
	greeting rendered on every visit for the life of the Engagement, which
	on the loss journey is what the screen said to a woman coming back
	three weeks after her pregnancy ended.

	A literal, not an expression, and that is the whole fix: the heading
	consults nothing -- not the record, not a visit count, not an outcome
	-- because "Your care" is true on the first visit and the four
	hundredth, for an `intake`, `active` or `completed` Engagement, and
	for a Client after a loss. The register's own rule is that a word which
	cannot be kind to every Client is the wrong word, never a word wanting
	a condition, so there is no branch here to keep true.

	The Practice's name has not gone anywhere: `serviceName` below is what
	`PageTitle` folds into `<title>`, and the portal shell above this route
	-- `portal/(authenticated)/+layout.svelte`'s `PortalTopBar` -- renders
	it on the page itself, which is why losing it from the `<h1>` loses
	nothing a Client needs to know about whose portal she is in.
-->
<RecordDetail
	title="Your care"
	serviceName={engagementLabel({ practiceName: page.data.practiceName, createdAt: page.data.createdAt })}
	{summary}
	{actions}
	sections={detail
		? [
				{ heading: 'Your visits', content: visitsSection },
				{ heading: 'Everything that has happened', content: activitySection }
			]
		: []}
	loading={detail || error ? undefined : 'Loading your care'}
	loadError={error || undefined}
/>
