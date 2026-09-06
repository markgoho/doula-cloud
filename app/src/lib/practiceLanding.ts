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
export type { Fetcher } from './offer.js';
import { loadBalance } from './billing.js';
import { loadConnectStatus, type ConnectStatus } from './payments.js';
import type { Fetcher } from './offer.js';
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
	roster: Block<RosterHealth>;
	credit: Block<CreditHealth>;
	connect: Block<ConnectHealth>;
	requests: Block<RequestHealth>;
}

/*
 * `GET .../staff` and `GET .../billing` are `ownerAndAdmin`.
 */
export function canReadRoster(roles: string[]): boolean {
	return roles.includes('owner') || roles.includes('admin');
}

/** `GET .../payments/connect` is Owner-only -- narrower than the roster,
 * which is why the two gates are separate rather than one `isAdmin`. */
export function canReadConnect(roles: string[]): boolean {
	return roles.includes('owner');
}

/** Whether `OverviewHub`'s optional `secondary` region has anything in
 * it. A Doula reaches none of the three blocks, so she gets no rail at
 * all rather than an empty aside beside her Offers. */
export function hasSecondary(landing: PracticeLanding): boolean {
	return (
		landing.roster !== undefined ||
		landing.credit !== undefined ||
		landing.connect !== undefined ||
		landing.requests !== undefined
	);
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

async function readOpenOffers(fetcher: Fetcher, practiceId: string): Promise<Offer[]> {
	const offers = await loadInbox(fetcher, practiceId);
	return offers.filter((offer) => isOpen(offer));
}

/*
 * `?all=true` matters, and it is not a detail: `client.ListHandler`
 * defaults to Clients who have work -- an Engagement, or a pending
 * Engagement Request -- so a Practice whose first Client has not been
 * engaged yet reads as having none. That puts the first-run empty state
 * in front of somebody who has already done the one thing it asks for,
 * which is the exact failure this page exists to stop. The question here
 * is whether the Practice has a Client at all, so it asks that.
 *
 * Now that #446 has the endpoint cursor-paginated, one page (at most 30
 * rows) answers the question -- items.length > 0 -- without ever needing
 * hasMore or a second request.
 */
async function hasAnyClient(fetcher: Fetcher, practiceId: string): Promise<boolean> {
	const response = await fetcher(`/api/practices/${practiceId}/clients?all=true`);
	if (!response.ok) {
		throw new Error(await apiErrorMessage(response));
	}
	const clients: { items: unknown[] } = await response.json();
	return clients.items.length > 0;
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
	session: { practiceName: string; roles: string[] }
): Promise<PracticeLanding> {
	const { roles } = session;

	const [openOffers, hasClients, roster, credit, connect, requests] = await Promise.all([
		readOpenOffers(fetcher, practiceId),
		hasAnyClient(fetcher, practiceId),
		canReadRoster(roles) ? block(() => loadRoster(fetcher, practiceId)) : undefined,
		canReadRoster(roles)
			? block(async () => {
					const balance = await loadBalance(fetcher, practiceId);
					return { balance: balance.balance };
				})
			: undefined,
		canReadConnect(roles)
			? block(async () => {
					const status = await loadConnectStatus(fetcher, practiceId);
					return { status: status.status, requirementsDue: status.requirementsDue };
				})
			: undefined,
		// Same Owner/Admin gate the inbox endpoint itself holds -- a Doula
		// cannot decide a Request, so she is not told one is waiting here.
		canReadRoster(roles)
			? block(async () => {
					const page = await loadPendingRequests(fetcher, practiceId);
					return { count: page.items.length, hasMore: page.hasMore };
				})
			: undefined
	]);

	return {
		practiceName: session.practiceName,
		roles,
		openOffers,
		hasClients,
		roster,
		credit,
		connect,
		requests
	};
}
