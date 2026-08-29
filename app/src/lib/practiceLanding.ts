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
export type { Fetcher } from './offer.js';
import { loadBalance } from './billing.js';
import { loadConnectStatus, type ConnectStatus } from './payments.js';
import type { Fetcher } from './offer.js';

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

export interface ConnectHealth {
	status: ConnectStatus;
	requirementsDue: string[];
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
	return landing.roster !== undefined || landing.credit !== undefined || landing.connect !== undefined;
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

async function loadSession(
	fetcher: Fetcher,
	practiceId: string
): Promise<{ practiceName: string; roles: string[] }> {
	const response = await fetcher(`/api/practices/${practiceId}/session`);
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json();
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
 * The endpoint still returns every row -- it is one of the four waiting
 * on a cursor (#446). The page counts the list and renders none of it, so
 * nothing here is unbounded to draw, but the request is far bigger than
 * the question; when #446 lands, this asks for one row instead.
 */
async function hasAnyClient(fetcher: Fetcher, practiceId: string): Promise<boolean> {
	const response = await fetcher(`/api/practices/${practiceId}/clients?all=true`);
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const clients: unknown[] = await response.json();
	return clients.length > 0;
}

async function loadRoster(fetcher: Fetcher, practiceId: string): Promise<RosterHealth> {
	const response = await fetcher(`/api/practices/${practiceId}/staff`);
	if (!response.ok) {
		throw new Error(await response.text());
	}
	const roster: {
		members: unknown[];
		invitations: { expired: boolean }[];
	} = await response.json();
	return {
		members: roster.members.length,
		pendingInvitations: roster.invitations.length,
		expiredInvitations: roster.invitations.filter((invitation) => invitation.expired).length
	};
}

/**
 * Loads everything the hub draws, in one call.
 *
 * The session comes first because its `roles` decide which of the other
 * requests are even allowed; the rest go together, so the hub costs two
 * round trips rather than five.
 */
export async function loadPracticeLanding(
	fetcher: Fetcher,
	practiceId: string
): Promise<PracticeLanding> {
	const session = await loadSession(fetcher, practiceId);
	const { roles } = session;

	const [openOffers, hasClients, roster, credit, connect] = await Promise.all([
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
			: undefined
	]);

	return { practiceName: session.practiceName, roles, openOffers, hasClients, roster, credit, connect };
}
