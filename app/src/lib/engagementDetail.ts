/**
 * The reads behind the Staff Engagement page that the page itself used to
 * own (#695).
 *
 * That route is 780 lines and fetches seven sections, and its only
 * interface was the rendered page: a spec had to stub every endpoint and
 * render everything to assert anything, so the one that exists answers
 * six of the seven with a 403 and exercises the due-date summary alone.
 * What is here is what the page was doing by hand -- building URLs,
 * reading a refusal, reversing a page of Messages, deciding that an Offers
 * refusal means "not permitted" rather than "broken". Each is now a
 * function a test can call.
 *
 * What is deliberately *not* here: the Contract, Invoice and Plan
 * sections. Those already delegate to `contract.ts`, `invoice.ts` and
 * `planInstance.ts`; all the page adds is a try/catch that turns a thrown
 * error into that section's own message. Lifting those five-line wrappers
 * into this module would create exactly the pass-through modules #695's
 * first change deleted thirteen of.
 */

import { apiErrorMessage } from './api.js';
import type { CursorPage } from './paginatedList.svelte.js';

/** A fetch-shaped function, injected so these can be unit-tested without
 * mocking the global fetch or SvelteKit's `$app` modules -- the same
 * seam `invoice.ts` and `planInstance.ts` already take. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** Which Engagement, at which Practice. Passed as one value because every
 * function here needs both and neither is ever meaningful alone. */
export interface EngagementReference {
	practiceId: string;
	engagementId: string;
}

export interface EngagementSummary {
	engagementId: string;
	clientId: string;
	clientName: string;
	status: string;
	createdAt: string;
	dueDate?: string;
}

export interface Visit {
	visitId: string;
	staffId: string;
	staffName: string;
	createdAt: string;
}

/**
 * The only two facts these functions need about a Message: which one it
 * is, and whether it carries an image.
 *
 * Structural on purpose, and narrower than the Message the thread
 * renders. That type lives on `MessageThread.svelte`, and a `.ts` module
 * importing a component for a type would drag a Svelte compile into
 * anything that touches it -- including a unit test that only wants to
 * check a cursor. Callers pass their own richer type and it satisfies
 * this.
 */
export interface MessageReference {
	messageId: string;
	attachmentContentType?: string;
}

export interface Doula {
	staffId: string;
	name: string;
	employmentType: string;
}

export function engagementURL({ practiceId, engagementId }: EngagementReference): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}`;
}

export function visitsURL(reference: EngagementReference): string {
	return `${engagementURL(reference)}/visits`;
}

export function messagesURL(reference: EngagementReference): string {
	return `${engagementURL(reference)}/messages`;
}

export function portalInviteURL(reference: EngagementReference): string {
	return `${engagementURL(reference)}/portal-invite`;
}

/**
 * The Engagement itself. The one blocking read on this route: without it
 * there is no page, which is why it is the only section that runs in
 * `+page.ts`'s load rather than after mount.
 *
 * Throws on a refusal, so SvelteKit's load turns it into the route's
 * error boundary rather than the page holding its own error string.
 */
export async function loadEngagement(
	fetcher: Fetcher,
	reference: EngagementReference
): Promise<EngagementSummary> {
	const response = await fetcher(engagementURL(reference));
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as EngagementSummary;
}

/**
 * One page of Visits, newest first from the BFF (#446).
 *
 * Throws on a refusal so `PaginatedList` can catch it -- the shape all six
 * paging lists use.
 */
export async function loadVisitsPage(
	fetcher: Fetcher,
	reference: EngagementReference,
	cursor: string
): Promise<CursorPage<Visit>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${visitsURL(reference)}${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as CursorPage<Visit>;
}

/**
 * One page of Messages, reversed.
 *
 * The BFF answers newest-first, like every other cursor list, but a
 * Message thread reads oldest-at-the-top -- so this is the one list on the
 * page whose paging is a prepend rather than an append, which is why it
 * does not use `PaginatedList`. The reversal lived inline in the route as
 * a bare `.toReversed()`, where nothing said why.
 */
export async function loadMessagesPage<M>(
	fetcher: Fetcher,
	reference: EngagementReference,
	cursor: string
): Promise<CursorPage<M>> {
	const query = cursor ? `?cursor=${encodeURIComponent(cursor)}` : '';
	const response = await fetcher(`${messagesURL(reference)}${query}`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	const page = (await response.json()) as CursorPage<M>;
	return { ...page, items: page.items.toReversed() };
}

/**
 * Object URLs for the image attachments in `items` that do not already
 * have one, keyed by message id.
 *
 * Returns them rather than mutating a caller's map, so a test can read the
 * answer without a component. The caller owns revoking them --
 * `URL.createObjectURL` allocates against the document, and only the
 * component knows when its own teardown has come.
 *
 * A fetch that refuses is skipped rather than throwing: an attachment that
 * will not load is a missing thumbnail, not a broken thread.
 */
export async function loadAttachmentPreviews(
	fetcher: Fetcher,
	reference: EngagementReference,
	items: readonly MessageReference[],
	alreadyLoaded: Readonly<Record<string, string>>
): Promise<Record<string, string>> {
	const wanted = items.filter(
		(message) =>
			message.attachmentContentType?.startsWith('image/') &&
			!Object.hasOwn(alreadyLoaded, message.messageId)
	);

	const loaded: Record<string, string> = {};
	await Promise.all(
		wanted.map(async (message) => {
			const response = await fetcher(`${messagesURL(reference)}/${message.messageId}/attachment`);
			if (!response.ok) return;
			loaded[message.messageId] = URL.createObjectURL(await response.blob());
		})
	);
	return loaded;
}

/** What the Offers section needs, or `undefined` when the caller may not
 * read it. */
export interface OffersSection {
	offers: unknown[];
	doulas: Doula[];
}

/**
 * The Offers section: who has been offered this Engagement, and which
 * Doulas could be.
 *
 * Two reads, both Owner/Admin, and `undefined` if either refuses. That is
 * the section's actual rule and it was buried in a bare `catch {}` in the
 * route: a Doula may not read who else was offered her work, so the
 * section is left out rather than shown broken. Returning `undefined`
 * rather than throwing says "not for you" in the type, which an empty
 * catch could not.
 *
 * `loadOffers` is injected rather than imported so this module does not
 * depend on `offer.ts` for one call -- and so a test can drive the refusal
 * without standing up that module too.
 */
export async function loadOffersSection(
	fetcher: Fetcher,
	reference: EngagementReference,
	loadOffers: (fetcher: Fetcher, practiceId: string, engagementId: string) => Promise<unknown[]>
): Promise<OffersSection | undefined> {
	try {
		const offers = await loadOffers(fetcher, reference.practiceId, reference.engagementId);
		const response = await fetcher(`/api/practices/${reference.practiceId}/staff`);
		if (!response.ok) return undefined;
		const roster = (await response.json()) as { members: (Doula & { roles: string[] })[] };
		return {
			offers,
			doulas: roster.members
				.filter((member) => member.roles.includes('doula'))
				.map(({ staffId, name, employmentType }) => ({ staffId, name, employmentType }))
		};
	} catch {
		// Not permitted to read who was offered this work -- see above.
		return undefined;
	}
}

