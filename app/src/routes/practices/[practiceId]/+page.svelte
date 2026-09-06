<script lang="ts">
	/*
	 * The Practice landing page -- archetype B, on `OverviewHub` (#423,
	 * ADR-0018).
	 *
	 * It used to be an `<h1>` and eight bare links, which is the abandon
	 * point `docs/journeys/evaluator-doula.md` names: "Six of the seven are
	 * administration. The words 'birth plan' and 'visit' do not appear."
	 * Those links are chrome, and chrome now lives in
	 * `practices/+layout.svelte` until #452 builds the real shell, so this
	 * page is free to answer "what needs me today" instead.
	 */
	import { onMount } from 'svelte';
	import { page } from '#lib/appState.svelte.js';
	import { resolve } from '$app/paths';
	import { apiFetch, apiFetchWithSession } from '#lib/api.js';
	import {
		practicePushSubscriptionsPath,
		registerPushSubscription
	} from '#lib/pushRegistration.js';
	import { decideOffer } from '#lib/offer.js';
	import {
		hasSecondary,
		loadPracticeLanding,
		loadWaitingOnReplyPage,
		type PracticeLanding,
		type WaitingOnReply
	} from '#lib/practiceLanding.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import { activityLedgerColumns, loadPracticeActivityPage, type ActivityEntry } from '#lib/activityLedger.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import DescriptionList from '#lib/components/molecules/DescriptionList.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	let landing = $state<PracticeLanding | undefined>();
	let error = $state('');

	// #486's Recent-activity feed: cursor-paginated on its own, unlike the
	// one-shot blocks loadPracticeLanding already merges into `landing`, so
	// it gets its own PaginatedList the same way the Engagement page's own
	// Visits section does.
	const activity = new PaginatedList<ActivityEntry>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more activity'
	});
	let activityError = $state('');

	// #455's roll-up: cursor-paginated on its own, unlike the one-shot
	// blocks loadPracticeLanding already merges into `landing` -- the same
	// reasoning as `activity` above.
	const waitingOnReply = new PaginatedList<WaitingOnReply>({
		first: { items: [], hasMore: false },
		loadPage: (cursor) => loadWaitingOnReplyPage(apiFetchWithSession, page.params.practiceId!, cursor),
		failureMessage: 'Failed to load more Engagements waiting on a reply'
	});
	let waitingOnReplyError = $state('');

	async function load() {
		try {
			landing = await loadPracticeLanding(apiFetchWithSession, page.params.practiceId!);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load your Practice';
		}
	}

	async function loadActivity() {
		try {
			activity.reset(await loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, ''));
		} catch (error_) {
			activityError = error_ instanceof Error ? error_.message : 'Failed to load activity';
		}
	}

	async function loadWaitingOnReply() {
		try {
			waitingOnReply.reset(await loadWaitingOnReplyPage(apiFetchWithSession, page.params.practiceId!, ''));
		} catch (error_) {
			waitingOnReplyError =
				error_ instanceof Error ? error_.message : 'Failed to load Engagements waiting on a reply';
		}
	}

	onMount(async () => {
		await load();
		await loadActivity();
		await loadWaitingOnReply();

		// Fire-and-forget: #61's "once per device after login" push
		// registration is best-effort and must never block landing on the
		// Practice page, or navigate the page away, on any failure (see
		// pushRegistration.ts's doc comment) -- apiFetch, not
		// apiFetchWithSession, since that helper's own 401 handling would
		// sign the person out and redirect on a failure this call is
		// supposed to swallow silently.
		void registerPushSubscription(
			practicePushSubscriptionsPath(page.params.practiceId!),
			apiFetch
		);
	});

	// The whole hub is reloaded rather than the one Offer patched out:
	// accepting an Offer supersedes every other open Offer on the same
	// Engagement, and it can spend a credit, so the rail is stale too.
	async function handleDecide(offerId: string, action: 'accept' | 'decline') {
		await decideOffer(apiFetchWithSession, page.params.practiceId!, offerId, action);
		await load();
	}

	// #455: the roll-up of Engagements whose thread's latest Message came
	// from the Client. rowHref makes the Client's own name (accessor's
	// first column) the link through to that Engagement's detail page,
	// where the thread already renders -- DataTable's own treatment,
	// matching invoices/+page.svelte's engagementHref.
	function waitingOnReplyEngagementHref(item: WaitingOnReply): string {
		return resolve('/practices/[practiceId]/engagements/[engagementId]', {
			practiceId: page.params.practiceId!,
			engagementId: item.engagementId
		});
	}

	const waitingOnReplyColumns = [
		{ label: 'Client', accessor: (item: WaitingOnReply) => item.clientName },
		{
			label: 'Last message',
			accessor: (item: WaitingOnReply) => formatActivityTimestamp(item.lastMessageAt),
			variant: 'meta' as const,
			datetimeAccessor: (item: WaitingOnReply) => item.lastMessageAt
		}
	];

	const connectLabels: Record<string, { label: string; variant: 'neutral' | 'warning' | 'success' }> = {
		not_connected: { label: 'Not connected', variant: 'neutral' },
		onboarding_incomplete: { label: 'Onboarding incomplete', variant: 'warning' },
		pending: { label: 'Awaiting Stripe review', variant: 'warning' },
		payouts_restricted: { label: 'Payouts on hold', variant: 'warning' },
		active: { label: 'Taking payments', variant: 'success' }
	};
</script>

{#snippet primary()}
	<Heading level={2} variant="section" text="Offers awaiting your answer" />
	<Text
		text="Work you have been offered. Accepting puts you on the birth; declining is final for that offer."
		tone="variant"
	/>
	<OfferInbox offers={landing!.openOffers} onDecide={handleDecide} />

	<!--
		#455: computed from thread authorship (the latest Message on the
		thread came from the Client), not read state -- ADR-0028 (#454)
		settled that there is no notification bell, and read state is
		inherently per-person while this roll-up is Practice-scoped. "Who is
		waiting on me" belongs beside Offers: both answer "what needs me
		today", for any Staff role.
	-->
	<Heading level={2} variant="section" text="Clients waiting on a reply" />
	{#if waitingOnReplyError}
		<Notice variant="error" message={waitingOnReplyError} />
	{/if}
	<DataTable
		columns={waitingOnReplyColumns}
		rows={waitingOnReply.items}
		rowHref={waitingOnReplyEngagementHref}
		hasMore={waitingOnReply.hasMore}
		onLoadMore={() => waitingOnReply.loadMore()}
		isLoadingMore={waitingOnReply.isLoadingMore}
		loadMoreError={waitingOnReply.loadMoreError}
		emptyMessage="Nobody is waiting on a reply."
	/>
{/snippet}

{#snippet rosterBlock()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Your people" />
			{#if landing!.roster === 'unavailable'}
				<Text text="Could not load the roster just now." tone="muted" />
			{:else if landing!.roster}
				<DescriptionList
					items={[
						{ label: 'Members', value: String(landing!.roster.members) },
						{ label: 'Invitations pending', value: String(landing!.roster.pendingInvitations) }
					]}
				/>
				<!-- A cluster, not two stack children: `stack-l` spaces with
				     `margin-block-start`, which an inline Badge and an inline
				     Link ignore, so they would share a line with a single space
				     between them. -->
				<cluster-l space="var(--space-3)" align="center">
					{#if landing!.roster.expiredInvitations > 0}
						<Badge
							label={`${landing!.roster.expiredInvitations} invitation${landing!.roster.expiredInvitations === 1 ? '' : 's'} expired`}
							variant="warning"
						/>
					{/if}
					<Link
						href={resolve('/practices/[practiceId]/staff', { practiceId: page.params.practiceId! })}
						label="Manage staff"
						variant="secondary"
					/>
				</cluster-l>
			{/if}
		</stack-l>
	</section>
{/snippet}

<!--
	Where a stopped Doula becomes visible (#503). ADR-0017: a pending
	Request stops her from doing any work at all, so the hub whose question
	is "what needs me today" has to answer with it. The block counts and
	hands off -- the decision itself is made on the inbox, one click on.
-->
{#snippet requestBlock()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Requests awaiting approval" />
			{#if landing!.requests === 'unavailable'}
				<Text text="Could not load pending requests just now." tone="muted" />
			{:else if landing!.requests}
				<cluster-l space="var(--space-3)" align="center">
					{#if landing!.requests.count > 0}
						<Badge
							label={`${landing!.requests.count}${landing!.requests.hasMore ? '+' : ''} waiting`}
							variant="warning"
						/>
					{:else}
						<Text text="Nobody is waiting on you." tone="muted" />
					{/if}
					<Link
						href={resolve('/practices/[practiceId]/engagement-requests', {
							practiceId: page.params.practiceId!
						})}
						label="Review requests"
						variant="secondary"
					/>
				</cluster-l>
			{/if}
		</stack-l>
	</section>
{/snippet}

{#snippet creditBlock()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Credits" />
			{#if landing!.credit === 'unavailable'}
				<Text text="Could not load your credit balance just now." tone="muted" />
			{:else if landing!.credit}
				<DescriptionList items={[{ label: 'Balance', value: String(landing!.credit.balance) }]} />
				<Link
					href={resolve('/practices/[practiceId]/billing', { practiceId: page.params.practiceId! })}
					label="Buy credits"
					variant="secondary"
				/>
			{/if}
		</stack-l>
	</section>
{/snippet}

{#snippet connectBlock()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Getting paid" />
			{#if landing!.connect === 'unavailable'}
				<Text text="Could not load your Stripe status just now." tone="muted" />
			{:else if landing!.connect}
				{#if landing!.connect.requirementsDue.length > 0}
					<Text
						text={`Stripe is waiting on ${landing!.connect.requirementsDue.length} more detail${landing!.connect.requirementsDue.length === 1 ? '' : 's'}.`}
						tone="variant"
					/>
				{/if}
				<cluster-l space="var(--space-3)" align="center">
					<Badge
						label={connectLabels[landing!.connect.status].label}
						variant={connectLabels[landing!.connect.status].variant}
					/>
					<Link
						href={resolve('/practices/[practiceId]/settings/payments', {
							practiceId: page.params.practiceId!
						})}
						label="Payment settings"
						variant="secondary"
					/>
				</cluster-l>
			{/if}
		</stack-l>
	</section>
{/snippet}

{#snippet secondary()}
	{#if landing!.requests !== undefined}{@render requestBlock()}{/if}
	{#if landing!.roster !== undefined}{@render rosterBlock()}{/if}
	{#if landing!.credit !== undefined}{@render creditBlock()}{/if}
	{#if landing!.connect !== undefined}{@render connectBlock()}{/if}
{/snippet}

<!--
	#486 AC1/AC3: the practice-wide feed, gated per row by #485's
	activitygate and rendered here with the brief's own ledger treatment --
	a meta date column, the event in body text, the actor muted, hairline
	rows, no vertical column rule. Low prominence per the design brief's
	own #433 amendment, which is why it renders through OverviewHub's
	`feed` slot rather than inside `primary` or `secondary`.
-->
{#snippet activityFeed()}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="card" text="Recent activity" />
			{#if activityError}
				<Notice variant="error" message={activityError} />
			{/if}
			<DataTable
				columns={activityLedgerColumns()}
				rows={activity.items}
				hasMore={activity.hasMore}
				onLoadMore={() => activity.loadMore()}
				isLoadingMore={activity.isLoadingMore}
				loadMoreError={activity.loadMoreError}
				emptyMessage="Nothing has happened yet."
			/>
		</stack-l>
	</section>
{/snippet}

{#snippet empty()}
	<!--
		The zero-Client state is the reason ADR-0018 makes `empty` a required
		prop. It names the work rather than the filing cabinet, and it offers
		one action, not a menu -- a menu is what made this page the abandon
		point in the first place.
	-->
	<stack-l space="var(--space-4)">
		<Text
			text="Nothing is here yet, because no Client is. Add one and this becomes the Client's birth plan, your visits to the Client, and the contract and invoices between you."
		/>
		<!--
			Through search, not straight into intake (#466). ADR-0017 makes
			search the first screen of intake and the only door to it, so a
			link that jumped the queue here would be the top-level "Add a
			Client" action the ADR rules out. The search's own empty state
			carries whatever was typed into the new record, so nothing is
			asked twice.
		-->
		<Link
			href={resolve('/practices/[practiceId]/clients/search', {
				practiceId: page.params.practiceId!
			})}
			label="Add your first Client"
		/>
	</stack-l>
{/snippet}

<OverviewHub
	title={landing ? `Welcome to ${landing.practiceName}` : ''}
	isEmpty={landing ? !landing.hasClients : false}
	{primary}
	secondary={landing && hasSecondary(landing) ? secondary : undefined}
	feed={landing ? activityFeed : undefined}
	{empty}
	loading={landing || error ? undefined : 'Loading your Practice'}
	loadError={error || undefined}
/>
