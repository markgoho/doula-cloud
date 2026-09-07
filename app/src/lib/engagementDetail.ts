/**
 * The reads and writes behind the Staff Engagement page that the page
 * itself used to own (#695, #841).
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
 * #841 moved the page's own writes in too -- sending a portal invite,
 * adding and reassigning a Visit, sending a Message and downloading its
 * attachment -- so the page holds no raw `apiFetchWithSession` call of its
 * own; each is a `SectionState.load`/`mutate` callback around one of these
 * functions instead of a hand-written `catch (error_) { ... instanceof
 * Error ... }` block.
 *
 * What is deliberately *not* here: the Contract, Invoice and Plan
 * sections. Those already delegate to `contract.ts`, `invoice.ts` and
 * `planInstance.ts`; all the page adds is a try/catch that turns a thrown
 * error into that section's own message. Lifting those five-line wrappers
 * into this module would create exactly the pass-through modules #695's
 * first change deleted thirteen of.
 */

import type { Fetcher } from './fetcher.js';

import { apiErrorMessage } from './api.js';
import type { CursorPage } from './paginatedList.svelte.js';

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
	/** When the Visit itself happens, distinct from createdAt (#250) --
	 * absent for a Visit not yet scheduled. */
	scheduledAt?: string;
	/** Free-text notes any Staff member who may read this Visit may also
	 * write (#251). `undefined` for a Visit that has never had notes
	 * written -- distinct from `''`, a Visit whose notes were written and
	 * then deliberately cleared. */
	notes?: string;
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
 * Sends a portal invite for this Engagement, returning the token the BFF
 * minted -- the page turns that into the sharable link, since building a
 * URL off `location.origin` belongs to the page, not this module.
 */
export async function sendPortalInvite(
	fetcher: Fetcher,
	reference: EngagementReference
): Promise<{ inviteToken: string }> {
	const response = await fetcher(portalInviteURL(reference), { method: 'POST' });
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as { inviteToken: string };
}

/** Adds a Visit to this Engagement, optionally scheduled (#250). The
 * caller reloads the list itself -- this only reports whether the add
 * succeeded. */
export async function createVisit(
	fetcher: Fetcher,
	reference: EngagementReference,
	scheduledAt?: string
): Promise<void> {
	const response = await fetcher(visitsURL(reference), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ scheduledAt })
	});
	if (!response.ok) throw new Error(await apiErrorMessage(response));
}

/**
 * Reassigns visitId to staffId.
 */
export async function reassignVisit(
	fetcher: Fetcher,
	reference: EngagementReference,
	visitId: string,
	staffId: string
): Promise<void> {
	const response = await fetcher(`${visitsURL(reference)}/${visitId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ staffId })
	});
	if (!response.ok) throw new Error(await apiErrorMessage(response));
}

/**
 * Sets, changes or clears visitId's own scheduled instant (#250) --
 * scheduledAt === undefined clears it, the same way an empty
 * `datetime-local` control reports itself.
 */
export async function scheduleVisit(
	fetcher: Fetcher,
	reference: EngagementReference,
	visitId: string,
	scheduledAt: string | undefined
): Promise<void> {
	const response = await fetcher(`${visitsURL(reference)}/${visitId}/schedule`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ scheduledAt })
	});
	if (!response.ok) throw new Error(await apiErrorMessage(response));
}

/**
 * Sets or clears visitId's own notes (#251). An empty string clears them
 * to the empty state, distinct from a Visit that has never had notes
 * written -- there is no way to send this route back to "never written",
 * matching notes.go's own NotesRequest doc comment.
 */
export async function saveVisitNotes(
	fetcher: Fetcher,
	reference: EngagementReference,
	visitId: string,
	notes: string
): Promise<void> {
	const response = await fetcher(`${visitsURL(reference)}/${visitId}/notes`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ notes })
	});
	if (!response.ok) throw new Error(await apiErrorMessage(response));
}

/**
 * Sends a Message, with or without an attachment -- the only two shapes
 * `MessageThread`'s composer can produce. Returns the created Message so
 * the caller can append it and fetch its attachment preview.
 */
export async function sendMessage<M>(
	fetcher: Fetcher,
	reference: EngagementReference,
	body: string,
	attachment: File | undefined
): Promise<M> {
	let response: Response;
	if (attachment) {
		const form = new FormData();
		form.set('body', body);
		form.set('attachment', attachment);
		response = await fetcher(messagesURL(reference), { method: 'POST', body: form });
	} else {
		response = await fetcher(messagesURL(reference), {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ body })
		});
	}
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as M;
}

/**
 * Downloads a Message's attachment as a Blob. The caller turns it into an
 * object URL and drives the browser's own download -- DOM work that has
 * no place in a module tested without one.
 */
export async function downloadAttachment(
	fetcher: Fetcher,
	reference: EngagementReference,
	messageId: string
): Promise<Blob> {
	const response = await fetcher(`${messagesURL(reference)}/${messageId}/attachment`);
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return response.blob();
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

