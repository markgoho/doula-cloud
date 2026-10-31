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
		closeSecondFactorRecommendation,
		isConnectCardShown,
		isRequestBlockShown,
		isRosterBlockShown,
		loadPracticeLanding,
		loadWaitingOnReplyPage,
		type ConnectHealth,
		type PracticeLanding,
		type RequestHealth,
		type RosterHealth,
		type WaitingOnReply
	} from '#lib/practiceLanding.js';
	import { CONNECT_STATUS_BADGES } from '#lib/payments.js';
	import {
		EVERY_ROLE_SENTENCE,
		hasEveryRole,
		isAmbientContractor,
		isDoula,
		isOwner,
		isOwnerOrAdmin
	} from '#lib/roles.js';
	import { PaginatedList } from '#lib/paginatedList.svelte.js';
	import DoorLink from '#lib/components/atoms/DoorLink.svelte';
	import { startHref } from './clients/new/intake.js';
	import {
		activityLedgerColumns,
		loadPracticeActivityPage,
		RECENT_ACTIVITY_ROWS,
		type ActivityEntry
	} from '#lib/activityLedger.js';
	import { formatActivityTimestamp } from '#lib/dates.js';
	import { takeOutcome } from '#lib/outcome.js';
	import type { PracticeSession } from './+layout.js';
	import OfferInbox from '#lib/components/organisms/OfferInbox.svelte';
	import OverviewHub from '#lib/components/templates/OverviewHub.svelte';
	import SecondFactorRecommendation from '#lib/components/molecules/SecondFactorRecommendation.svelte';
	import DataTable from '#lib/components/organisms/DataTable.svelte';
	import SummaryList from '#lib/components/molecules/SummaryList.svelte';
	import Badge from '#lib/components/atoms/Badge.svelte';
	import Heading from '#lib/components/atoms/Heading.svelte';
	import Link from '#lib/components/atoms/Link.svelte';
	import Notice from '#lib/components/atoms/Notice.svelte';
	import Text from '#lib/components/atoms/Text.svelte';

	let landing = $state<PracticeLanding | undefined>();
	let error = $state('');

	// What the act that sent her here did (#1710): creating the Practice
	// at signup (#2182), or agreeing to a changed Terms of Service or
	// Privacy Policy (#1549). Read once,
	// while the page renders, so a reload says nothing.
	const outcome = takeOutcome(page.url);

	// #486's Recent-activity feed, bounded by #2006: the newest few rows of
	// the feed's first page, and a link to the whole ledger on its own
	// screen. At 390px the whole page of rows stacked to 6,250px, and the
	// brief puts the ledger low on every page it appears on -- it is not
	// what a person comes to the Overview for. Eight is the drawing's
	// (`RECENT_ACTIVITY_ROWS`, shared with the Engagement page, #2105).
	let recentActivity = $state<ActivityEntry[]>([]);
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

	// #2245, the Staff sibling of #2218: both lists start empty, so drawn
	// before their reads answer they said nothing had happened and nobody
	// was waiting. The page stays on its loading state until both have
	// answered, as Your care does (#2218), so neither empty line paints
	// before the data and no row arriving later moves the cards under it
	// (ADR-0020). The three reads run side by side, so the page waits for
	// the slowest, not their sum.
	let areListsAnswered = $state(false);

	// Resolved once by practices/[practiceId]/+layout.ts (#835), not a
	// fetch of loadPracticeLanding's own -- the Membership its roles are
	// read off.
	const session = $derived((page.data as { session: PracticeSession }).session);

	// The empty Practice's sentence about Credits (#1612): with the
	// balance where she may read it and the read worked, without it
	// otherwise.
	const creditSentence = $derived(
		landing?.credit && landing.credit !== 'unavailable'
			? `Adding a Client is free. Starting work with a Client uses 1 Credit, and this Practice has ${landing.credit.count}.`
			: "Adding a Client is free. Starting work with a Client uses 1 of the Practice's Credits."
	);

	async function load() {
		try {
			landing = await loadPracticeLanding(apiFetchWithSession, page.params.practiceId!, session);
		} catch (error_) {
			error = error_ instanceof Error ? error_.message : 'Failed to load your Practice';
		}
	}

	async function loadActivity() {
		try {
			const first = await loadPracticeActivityPage(apiFetchWithSession, page.params.practiceId!, '');
			recentActivity = first.items.slice(0, RECENT_ACTIVITY_ROWS);
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
		await Promise.all([load(), loadActivity(), loadWaitingOnReply()]);
		areListsAnswered = true;

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

	// #1533: the close is the BFF's to keep, so it holds on every device.
	// The control is gone once it works, so focus goes to the start of the
	// main content rather than falling to the document.
	async function closeRecommendation() {
		await closeSecondFactorRecommendation(apiFetchWithSession, page.params.practiceId!);
		landing!.secondFactorRecommended = false;
		document.querySelector<HTMLElement>('#main')?.focus();
	}

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

	// The words of the Getting paid card with no Stripe account (#1589).
	// An Owner can connect Stripe, so hers asks; an Admin cannot
	// (ADR-0035), so hers says who can. The last sentence is left out once
	// the Practice has chosen Stripe: it answered the question.
	const ownerAskStart =
		'Clients cannot pay you by card yet. To take card payments, connect Stripe. It takes about fifteen minutes, and then Stripe reviews your details.';
	const ownerAskEnd = 'If you collect payment yourself, you do not need Stripe.';
	const adminAsk = 'Clients cannot pay this Practice by card yet. A Practice Owner has to connect Stripe.';

	const isPracticeOwner = $derived(isOwner(session));

	interface QuickLinkCard {
		label: string;
		description: string;
		href: string;
	}

	// #2095: the drawing gives three cards a count in place of their line
	// ("14 active", "31 Credits"), so a person sees how much is there
	// before going in. Each count comes from a read this page already
	// makes; the active Clients from the one-row `/clients/count` that
	// also says whether the Practice has a Client at all. Until a count is
	// known the card keeps its own line.
	function countLine(count: number | undefined, words: string): string | undefined {
		if (count === undefined) return undefined;
		return count === 0 ? `None ${words}` : `${count} ${words}`;
	}

	const quickLinkCards = $derived.by((): QuickLinkCard[] => {
		const practiceId = page.params.practiceId!;
		const isForOwnerOrAdmin = isOwnerOrAdmin(session);
		return [
			{
				label: 'Clients',
				description: countLine(landing?.activeClients, 'active') ?? "Find a Client's record and care.",
				href: resolve('/practices/[practiceId]/clients', { practiceId })
			},
			...(isForOwnerOrAdmin
				? [
						{
							label: 'Credits',
							description:
								landing?.credit && landing.credit !== 'unavailable'
									? landing.credit.count
									: 'Buy Credits and see the balance.',
							href: resolve('/practices/[practiceId]/billing', { practiceId })
						}
					]
				: []),
			// An Offer goes to a Doula (#2141), so the card that counts hers is
			// not offered to someone who can never hold one (#2228).
			...(isDoula(session)
				? [
						{
							label: 'Your offers',
							description: countLine(landing?.openOffers.length, 'waiting on you') ?? 'Work you have been offered.',
							href: resolve('/practices/[practiceId]/offers', { practiceId })
						}
					]
				: []),
			...(isForOwnerOrAdmin
				? [
						{
							label: 'Staff',
							description: 'Who works here, and who is invited.',
							href: resolve('/practices/[practiceId]/staff', { practiceId })
						}
					]
				: []),
			{
				label: 'Plan Templates',
				description: 'The questions every birth plan and care plan starts from.',
				href: resolve('/practices/[practiceId]/settings/plan-templates', { practiceId })
			},
			{
				label: 'Contract Template',
				description: 'The terms every Contract is written from.',
				href: resolve('/practices/[practiceId]/settings/contract-template', { practiceId })
			},
			...(isForOwnerOrAdmin
				? [
						{
							label: 'Getting paid',
							description: 'How this Practice gets paid, and what Stripe still wants from it.',
							href: resolve('/practices/[practiceId]/settings/payments', { practiceId })
						}
					]
				: [])
		];
	});

	function notConnectedWords(connect: ConnectHealth): string {
		if (!isPracticeOwner) return adminAsk;
		return connect.billingMode === 'stripe' ? ownerAskStart : `${ownerAskStart} ${ownerAskEnd}`;
	}
</script>

{#snippet offerInbox()}
	<Heading level={2} variant="section" text="Offers awaiting your answer" />
	<Text
		text="Work you have been offered. Accepting puts you on the birth; declining is final for that offer."
		tone="variant"
	/>
	<OfferInbox offers={landing!.openOffers} onDecide={handleDecide} />
{/snippet}

{#snippet primary()}
	<!--
		#1533: first in the column, where she reads it before her work. Only
		in `primary`, which the empty Practice never draws (ADR-0048); the
		BFF also says no before First Value.
	-->
	{#if landing!.secondFactorRecommended}
		<SecondFactorRecommendation
			returnTo={resolve('/practices/[practiceId]', { practiceId: page.params.practiceId! })}
			onClose={closeRecommendation}
		/>
	{/if}
	<!-- An Offer goes to a Doula (#2141): a Staff member without the role
	     can never be offered work, so an inbox that could only ever say
	     "You have no offers." is not drawn for her. -->
	{#if isDoula(session)}{@render offerInbox()}{/if}

	<!--
		#455: computed from thread authorship (the latest Message on the
		thread came from the Client), not read state -- ADR-0028 (#454)
		settled that there is no notification bell, and read state is
		inherently per-person while this roll-up is Practice-scoped. "Who is
		waiting on me" belongs beside Offers: both answer "what needs me
		today", for any Staff role.
	-->
	<Heading level={2} variant="section" text="Clients waiting on a reply" />
	<!-- #2245: a failed read says so in place of the list, never above
		an empty one that says nobody is waiting. -->
	{#if waitingOnReplyError}
		<Notice variant="error" message={waitingOnReplyError} />
	{:else}
		<div class="waiting-table">
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
		</div>
	{/if}

	<!--
		#2228: what else needs her, each block only where it asks something
		of her, so on a quiet day the page holds the work and the way out of
		it. These used to sit in a rail beside the column, with a Credits
		block and a roster that only repeated the top bar; in the column
		they need no space held for them while the page loads.
	-->
	{#if isRequestBlockShown(landing!.requests)}{@render requestBlock(landing!.requests)}{/if}
	{#if isConnectCardShown(landing!.connect)}{@render connectBlock(landing!.connect)}{/if}
	{#if isRosterBlockShown(landing!.roster)}{@render rosterBlock(landing!.roster)}{/if}
{/snippet}

{#snippet rosterBlock(roster: RosterHealth | 'unavailable')}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="section" text="Your people" />
			{#if roster === 'unavailable'}
				<Text text="Could not load the roster just now." tone="muted" />
			{:else}
				<SummaryList
					rows={[
						{ label: 'Members', value: String(roster.members) },
						{ label: 'Invitations pending', value: String(roster.pendingInvitations) }
					]}
				/>
				<!-- A cluster, not two stack children: `stack-l` spaces with
				     `margin-block-start`, which an inline Badge and an inline
				     Link ignore, so they would share a line with a single space
				     between them. Drawn only with an expired invitation
				     (#2228), the one thing this block asks of her. -->
				<cluster-l space="var(--space-3)" align="center">
					<Badge
						label={`${roster.expiredInvitations} invitation${roster.expiredInvitations === 1 ? '' : 's'} expired`}
						variant="warning"
					/>
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
	Drawn only where a Request waits (#2228; #2032 had already taken away
	the link to an empty inbox).
-->
{#snippet requestBlock(requests: RequestHealth | 'unavailable')}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="section" text="Requests awaiting approval" />
			{#if requests === 'unavailable'}
				<Text text="Could not load pending requests just now." tone="muted" />
			{:else}
				<cluster-l space="var(--space-3)" align="center">
					<Badge label={`${requests.count}${requests.hasMore ? '+' : ''} waiting`} variant="warning" />
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

{#snippet connectBlock(connect: ConnectHealth | 'unavailable')}
	<section>
		<stack-l space="var(--space-3)">
			<Heading level={2} variant="section" text="Getting paid" />
			{#if connect === 'unavailable'}
				<Text text="Could not load your Stripe status just now." tone="muted" />
			{:else if connect.status === 'not_connected'}
				<Text text={notConnectedWords(connect)} />
				<Link
					href={resolve('/practices/[practiceId]/settings/payments', {
						practiceId: page.params.practiceId!
					})}
					label={isPracticeOwner ?'Set up card payments' : 'Getting paid'}
					variant={isPracticeOwner ? 'start' : 'secondary'}
				/>
			{:else}
				{#if connect.requirementsDue.length > 0}
					<Text
						text={`Stripe is waiting on ${connect.requirementsDue.length} more detail${connect.requirementsDue.length === 1 ? '' : 's'}.`}
						tone="variant"
					/>
				{/if}
				<cluster-l space="var(--space-3)" align="center">
					<Badge
						label={CONNECT_STATUS_BADGES[connect.status].label}
						variant={CONNECT_STATUS_BADGES[connect.status].variant}
					/>
					<Link
						href={resolve('/practices/[practiceId]/settings/payments', {
							practiceId: page.params.practiceId!
						})}
						label="Getting paid"
						variant="secondary"
					/>
				</cluster-l>
			{/if}
		</stack-l>
	</section>
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
			<!-- #2245: in place of the list, never above an empty one. -->
			{#if activityError}
				<Notice variant="error" message={activityError} />
			{:else}
				<div class="activity-table">
					<DataTable
						columns={activityLedgerColumns()}
						rows={recentActivity}
						hasMore={false}
						emptyMessage="Nothing has happened yet."
					/>
				</div>
			{/if}
			{#if recentActivity.length > 0}
				<Link
					href={resolve('/practices/[practiceId]/activity', { practiceId: page.params.practiceId! })}
					label="See all activity"
					variant="secondary"
				/>
			{/if}
		</stack-l>
	</section>
{/snippet}

<!--
	#2006: the drawing's quick-link cards, under the title, at every width.
	The same card the signed-out landing's doors are (`DoorLink`, #1645) in
	the same `grid-l`, so the row wraps to as many columns as fit. Offered
	only where that person can go: the role split is the top bar's own,
	and the three settings cards say what the Settings hub says about them.
	The drawing's Billing and Payments are named Credits and Getting paid
	here, the names #256 and ADR-0032 gave them after it was drawn.
-->
{#snippet quickLinks()}
	<nav aria-label="Quick links">
		<grid-l min="var(--door-min)" space="var(--space-4)">
			{#each quickLinkCards as card (card.href)}
				<DoorLink href={card.href} label={card.label} description={card.description} />
			{/each}
		</grid-l>
	</nav>
{/snippet}

{#snippet empty()}
	<!--
		The zero-Client state is the reason ADR-0018 makes `empty` a required
		prop. It names the work rather than the filing cabinet, and it offers
		one action, not a menu -- a menu is what made this page the abandon
		point in the first place.

		The words name nobody as the Doula (#1599). Signup gives a solo Owner
		and an agency Owner the same three roles and nothing asks her which
		she is before this screen, so "your visits" and "between you" were
		untrue for an Owner who attends no births. One set of words is true
		for the two of them.
	-->
	{#if isAmbientContractor(session)}
		<!--
			#2273: a contractor who holds neither Owner nor Admin originates
			nothing (ADR-0017), so the BFF refuses her a Client of her own,
			and the Practice's Credits are not hers to spend. Her empty
			Overview says how work reaches her and draws the Offers that wait
			on her, which an empty hub would otherwise hide: she is the person
			whose work arrives as one.
		-->
		<stack-l space="var(--space-4)">
			<Text
				text="Work at this Practice reaches you as an Offer. Once you accept one, your work with the Client is here."
			/>
			{@render offerInbox()}
		</stack-l>
	{:else}
		<stack-l space="var(--space-4)">
			<!--
				#290, moved here from signup by #2049: the person who set the
				Practice up holds every role, and is told so on the first screen
				she lands on. Only for a person who does: an invited Doula or
				Admin on an empty Practice holds the roles she was given.
			-->
			{#if hasEveryRole(session)}
				<Text text={EVERY_ROLE_SENTENCE} />
			{/if}
			<Text
				text="Nothing is here yet, because no Client is. Add one and this becomes the Client's Birth Plan, the Visits with the Client, and the Contract and Invoices between the Client and your Practice."
			/>
			<!--
				What uses a Credit, and how many the Practice has, said at the
				start point (#1612, GOV.UK's "Start using a service"): its own
				paragraph, before the link. A fact, not a link and not a live
				region, so the screen keeps its one action (ADR-0048). A person
				who cannot read the balance (ADR-0008), or whose read of it
				failed, gets the sentence with no number. No price: the Credits
				screen is the one place that says one (#285).
			-->
			<Text text={creditSentence} />
			<!--
				Straight to the name question, not through the search (#1609,
				ADR-0017's amendment of 2026-10-02). This snippet only shows while
				the Practice holds no Client record, and then the search can find
				nobody: it would be two presses and a screen with no result. The
				collision check at the save still runs. From the first Client on,
				the search is the only door again. `from` sends the name
				question's Back here.
			-->
			<Link href={startHref(page.params.practiceId!, 'overview')} label="Add your first Client" variant="start" />
		</stack-l>
	{/if}
{/snippet}

<!--
	The title is the nav item's own name (#2141), the GOV.UK rule that a
	link's words and the page's heading match. It used to welcome her to
	the Practice by name on every visit: a first-visit greeting shown
	forever, which said nothing the top bar's Practice name does not, and
	read "Welcome to Jamie Owner" when signup left the name to default.
-->
<OverviewHub
	title="Overview"
	isEmpty={landing ? !landing.hasClients : false}
	{primary}
	feed={landing ? activityFeed : undefined}
	links={quickLinks}
	{empty}
	loading={error || (areListsAnswered && landing) ? undefined : 'Loading your Practice'}
	loadError={error || undefined}
	{outcome}
/>

<style>
	@layer components {
		/* The floors of this hub's two tables (#2063), each lower than
		   `DataTable`'s own 55rem: both sit in the primary column, which is
		   about 840px at a 1280px window, and each is a few short columns.
		   Below its floor a table shows its records; from it, the table.
		   Measured on this route's typical pilot screen (#1981), the
		   activity table's eight rows of the week and the waiting table's
		   one Client. `DataTable.svelte`'s floor comment says why a route
		   can set a floor lower than the component's.

		   Measured 2026-10-26 on macOS/Chromium: the waiting table needs
		   427px of a 456px column and fits from 27rem. The activity table's
		   What column wraps since #2112, so its floor is When and Who beside
		   a sentence's longest word: it needs 455px of a 448px column, its
		   cells growing with the column's fluid type, and fits from 28.5rem
		   (it was 48rem with the column unwrapped). */
		.waiting-table {
			container: waiting-table / inline-size;
		}

		.activity-table {
			container: activity-table / inline-size;
		}

		.waiting-table > :global(*),
		.activity-table > :global(*) {
			font-size: var(--text-body-size);
			--data-table-table-display: none;
			--data-table-records-display: block;
		}

		@container waiting-table (min-width: 27rem) {
			.waiting-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}

		@container activity-table (min-width: 28.5rem) {
			.activity-table > :global(*) {
				--data-table-table-display: table;
				--data-table-records-display: none;
			}
		}
	}
</style>
