/**
 * What the Practice landing page shows, and to whom (#423).
 *
 * The page is archetype B -- the overview hub -- and it is Tasha Bell's
 * abandon point in `docs/journeys/evaluator-doula.md`: "an empty filing
 * cabinet, not proof". So the zero-Client case is a first-class result
 * here, not a branch the page happens to take.
 *
 * The role gates below mirror `api/main.go` exactly. They are UX only --
 * `ownerAndAdmin` and `RequireOwner` on the BFF are what actually refuse
 * -- but a region that renders and then 403s is worse than one that never
 * renders, so the page asks only for what the caller may have.
 */
import { isOpen, loadInbox, type Offer } from './offer.js';
import { apiErrorMessage } from './apiErrorMessage.js';
import { loadPendingRequests } from './engagementRequest.js';
import { loadClientCount } from './client.js';
export type { Fetcher } from './fetcher.js';
import { creditCount, loadBalance } from './billing.js';
import { loadConnectStatus, type ConnectStatus } from './payments.js';
import { loadBillingMode, type BillingMode } from './invoice.js';
import type { Fetcher } from './fetcher.js';
import { hasSecondFactor, isDoula, isOwner } from './roles.js';
import type { CursorPage } from './paginatedList.svelte.js';

/*
 * How the roster is doing: people in, and addresses still waiting.
 */
export interface RosterHealth {
	members: number;
	pendingInvitations: number;
	/** Pending invitations already past `expiresAt`. They need inviting
	 * again or revoking, and nothing else on the page would say so. */
	expiredInvitations: number;
}

export interface CreditHealth {
	balance: number;
	/** The balance in words, "3 Welcome credits" or "N Credits"
	 * (`creditCount`, #1612), for the empty Practice's sentence. */
	count: string;
}

/*
 * How many Requests are waiting on a decision. A count, not the rows: the
 * hub says somebody is stopped and hands off to the inbox (#503), which
 * is where a decision is actually made. `hasMore` travels because the
 * count is one page deep -- a Practice with more than a page of waiting
 * Requests reads as "30+", which is honest rather than wrong.
 */
export interface RequestHealth {
	count: number;
	hasMore: boolean;
}

export interface ConnectHealth {
	status: ConnectStatus;
	requirementsDue: string[];
	/**
	 * How the Practice bills (#1589), from the same read the Getting paid
	 * screen makes (`loadBillingMode`), kept on the Connect block because
	 * only the Getting paid card uses it. `'unchosen'` is a Practice that
	 * has not answered yet; `'unavailable'` is a read that failed, which
	 * shows the card rather than hiding it.
	 */
	billingMode: BillingMode | 'unchosen' | 'unavailable';
}

/** Whether the overview draws the Getting paid card (#1589). A Practice
 * that bills by hand answered the question, and a card that goes on
 * showing what it did not do reads its use back to it (ADR-0048) -- so
 * it is not drawn in any Connect state. Nor is it drawn once Stripe takes
 * payments and wants nothing more (#2228): the Overview shows what needs
 * her, and the Getting paid screen in Settings still says how it stands. */
export function isConnectCardShown(connect: Block<ConnectHealth>): connect is ConnectHealth | 'unavailable' {
	if (connect === undefined) return false;
	if (connect === 'unavailable') return true;
	if (connect.billingMode === 'by_hand') return false;
	return connect.status !== 'active' || connect.requirementsDue.length > 0;
}

/*
 * #2228: the Overview's other two blocks, drawn only where they ask
 * something of her. With nothing to ask they would repeat the top bar's
 * Staff and the Requests inbox, and each would be one more thing on the
 * page at once (Hick's Law). A read that failed still draws its block,
 * so the failure is said rather than hidden.
 */
export function isRosterBlockShown(roster: Block<RosterHealth>): roster is RosterHealth | 'unavailable' {
	return roster === 'unavailable' || (roster !== undefined && roster.expiredInvitations > 0);
}

export function isRequestBlockShown(requests: Block<RequestHealth>): requests is RequestHealth | 'unavailable' {
	return requests === 'unavailable' || (requests !== undefined && requests.count > 0);
}

/**
 * One Engagement whose thread's latest Message came from the Client --
 * awaiting a staff reply (#455). Computed server-side from thread
 * authorship, not read state: ADR-0028 (#454) settled that there is no
 * notification bell, and read state is inherently per-person while this
 * roll-up is Practice-scoped.
 */
export interface WaitingOnReply {
	engagementId: string;
	clientName: string;
	lastMessageAt: string;
}

/** One page of #455's roll-up, cursor-paginated on its own -- like
 * `loadPracticeActivityPage` in `activityLedger.ts`, unlike the one-shot
 * blocks `loadPracticeLanding` merges below. A Practice with more
 * Engagements waiting than one page holds needs "Load more", not a
 * silently truncated list, so it is not folded into `PracticeLanding`. */
export async function loadWaitingOnReplyPage(
	fetcher: Fetcher,
	practiceId: string,
	cursor: string
): Promise<CursorPage<WaitingOnReply>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`/api/practices/${practiceId}/messages/awaiting-reply${query}`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	return response.json();
}

/**
 * One `secondary` block. `undefined` means the caller's roles do not
 * reach it, so the block is not drawn at all; `'unavailable'` means she
 * may see it and the endpoint failed, so the block says so rather than
 * disappearing and leaving her to guess whether she has any credits.
 */
export type Block<T> = T | 'unavailable' | undefined;

export interface PracticeLanding {
	practiceName: string;
	roles: string[];
	/** Only the Offers still awaiting an answer. `loadInbox` also returns
	 * decided ones; they belong on the Offers screen, not on a hub whose
	 * primary region is "what needs me today". */
	openOffers: Offer[];
	hasClients: boolean;
	/** How many Clients are in active care, for the Clients card's count
	 * (#2095). The same read answers `hasClients`. */
	activeClients: number;
	roster: Block<RosterHealth>;
	credit: Block<CreditHealth>;
	connect: Block<ConnectHealth>;
	requests: Block<RequestHealth>;
	/** Whether the hub shows the notice that recommends a second factor
	 * (#1533). The BFF decides it; see `isSecondFactorRecommended`. */
	secondFactorRecommended: boolean;
}

/** Who is asked about the second-factor notice (#1533): an Owner whose
 * session has no second factor. `GET .../second-factor-recommendation`
 * is Owner-only, and it never recommends to a session that has one, so
 * nobody else is asked. */
export function canBeRecommendedASecondFactor(session: { roles: string[]; secondFactor?: boolean }): boolean {
	return isOwner(session) && !hasSecondFactor(session);
}

const secondFactorRecommendationPath = (practiceId: string) =>
	`/api/practices/${practiceId}/second-factor-recommendation`;

/** Whether the notice shows. A failed read shows nothing: the notice is a
 * recommendation, and the five acts that need a second factor say so on
 * their own screens whatever this answers. */
async function isSecondFactorRecommended(fetcher: Fetcher, practiceId: string): Promise<boolean> {
	try {
		const response = await fetcher(secondFactorRecommendationPath(practiceId));
		if (!response.ok) return false;
		const body: { recommended?: boolean } = await response.json();
		return body.recommended === true;
	} catch {
		return false;
	}
}

/** Closes the notice for every act on the list she has reached now
 * (#1533). The BFF keeps the close, so it holds on every device and
 * after sign-out, and the notice comes back once at each new act. */
export async function closeSecondFactorRecommendation(fetcher: Fetcher, practiceId: string): Promise<void> {
	const response = await fetcher(`${secondFactorRecommendationPath(practiceId)}/close`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
}

/*
 * `GET .../staff` and `GET .../billing` are `ownerAndAdmin`.
 */
export function canReadRoster(roles: string[]): boolean {
	return roles.includes('owner') || roles.includes('admin');
}

/** `GET .../payments/connect` is Owner-or-Admin, the same pair as the
 * roster (#267): ADR-0008's read table gives Stripe Connect state to the
 * Admin for the reason it already gives her Invoice history -- this is
 * the state of the rail those Invoices are paid on. Kept as its own
 * function rather than folded into `canReadRoster` because the two
 * answer about different endpoints, and one of them may narrow again. */
export function canReadConnect(roles: string[]): boolean {
	return roles.includes('owner') || roles.includes('admin');
}

/** Resolves a best-effort block: its value, or `'unavailable'` when the
 * endpoint refused or the network did. Only the three `secondary` blocks
 * use this; a failure in the critical path still fails the page. */
async function block<T>(load: () => Promise<T>): Promise<Block<T>> {
	try {
		return await load();
	} catch {
		return 'unavailable';
	}
}

// A billing mode that cannot be read does not take the Connect block down
// with it: the card is shown, not hidden (#1589).
async function readBillingMode(
	fetcher: Fetcher,
	practiceId: string
): Promise<ConnectHealth['billingMode']> {
	try {
		return (await loadBillingMode(fetcher, practiceId)) ?? 'unchosen';
	} catch {
		return 'unavailable';
	}
}

async function readOpenOffers(fetcher: Fetcher, practiceId: string): Promise<Offer[]> {
	const offers = await loadInbox(fetcher, practiceId);
	return offers.filter((offer) => isOpen(offer));
}


// The pending/expired invitation counts are read off the roster's first
// page only (#446 paginates invitations at 30/page): undercounting past
// that is an accepted approximation for a 14-doula pilot practice, not a
// promise this is exact for an unbounded invitation history.
async function loadRoster(fetcher: Fetcher, practiceId: string): Promise<RosterHealth> {
	const response = await fetcher(`/api/practices/${practiceId}/staff`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const roster: {
		members: unknown[];
		invitations: { items: { expired: boolean }[] };
	} = await response.json();
	return {
		members: roster.members.length,
		pendingInvitations: roster.invitations.items.length,
		expiredInvitations: roster.invitations.items.filter((invitation) => invitation.expired).length
	};
}

/**
 * Loads everything the hub draws, in one call.
 *
 * `session` is `practices/[practiceId]/+layout.ts`'s already-resolved
 * Membership (#835), not fetched again here -- its `roles` still decide
 * which of the other requests are even allowed, so the hub costs one
 * round trip rather than two.
 */
export async function loadPracticeLanding(
	fetcher: Fetcher,
	practiceId: string,
	session: { practiceName: string; roles: string[]; secondFactor?: boolean }
): Promise<PracticeLanding> {
	const { roles } = session;

	const [openOffers, clientCount, roster, credit, connect, requests, secondFactorRecommended] = await Promise.all([
		// An Offer goes to a Doula (#2141), so a caller without the role has
		// none to answer and the hub draws no inbox for her: nothing to read.
		isDoula(session) ? readOpenOffers(fetcher, practiceId) : [],
		// One row from the BFF (#2095), where this was a whole page of the
		// list read to learn whether it was empty.
		loadClientCount(fetcher, practiceId),
		canReadRoster(roles) ? block(() => loadRoster(fetcher, practiceId)) : undefined,
		canReadRoster(roles)
			? block(async () => {
					const balance = await loadBalance(fetcher, practiceId);
					return { balance: balance.balance, count: creditCount(balance) };
				})
			: undefined,
		canReadConnect(roles)
			? block(async () => {
					const [status, billingMode] = await Promise.all([
						loadConnectStatus(fetcher, practiceId),
						readBillingMode(fetcher, practiceId)
					]);
					return { status: status.status, requirementsDue: status.requirementsDue, billingMode };
				})
			: undefined,
		// Same Owner/Admin gate the inbox endpoint itself holds -- a Doula
		// cannot decide a Request, so she is not told one is waiting here.
		canReadRoster(roles)
			? block(async () => {
					const page = await loadPendingRequests(fetcher, practiceId);
					return { count: page.items.length, hasMore: page.hasMore };
				})
			: undefined,
		canBeRecommendedASecondFactor(session)
			? isSecondFactorRecommended(fetcher, practiceId)
			: false
	]);

	return {
		practiceName: session.practiceName,
		roles,
		openOffers,
		hasClients: clientCount.records > 0,
		activeClients: clientCount.active,
		roster,
		credit,
		connect,
		requests,
		secondFactorRecommended
	};
}
