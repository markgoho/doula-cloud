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
import { refusalOrConfirmable, type Refusal } from './formErrors.js';
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
	/** The target statuses this caller may move the Engagement to from
	 * its current status (#253, ADR-0015) -- always present, empty when
	 * her role or the current status admits no move. The hub renders
	 * exactly these, never a hand-copied role table of its own. */
	statusMoves: string[];
	/** ADR-0015's birth outcome (#293) -- `live_birth`, `loss` or
	 * `unknown`, absent until a Practice records it. Staff-only: the
	 * portal's own Engagement read (portal.Detail) does not carry it,
	 * and nothing in `routes/portal/` reads this field. */
	birthOutcome?: string;
	/** The date the pregnancy ended, `YYYY-MM-DD` (#293) -- recorded with
	 * the outcome, and distinct from the day a Staff member typed it in.
	 * Absent for an `unknown` outcome, and for one not yet recorded. */
	pregnancyEndedOn?: string;
	/** The Client's portal-invite state (#255), using the same
	 * derivation the Clients list's own ClientListItem.portalInviteStatus
	 * carries -- absent when she has never been invited. */
	clientPortalInviteStatus?: string;
	/** Mirrors ClientListItem.emailSuppressed (#785, ADR-0029): whether
	 * the Client's address is currently suppressed. */
	clientEmailSuppressed?: boolean;
	/** Whether the Client has an email address on file at all -- absent
	 * on an older cached response, which reads as "has one" (the
	 * pre-#255 assumption). A Client with none cannot be invited to the
	 * portal at all. */
	clientHasEmail?: boolean;
	/** Whether Clients can pay this Practice at all (#270), sourced from
	 * practices.stripe_connect_card_payments_status being 'active'. A
	 * standing fact InvoiceSection reads before ever showing the Create
	 * Invoice form -- absent on an older cached response, which reads as
	 * "can pay" so a stale cache never manufactures a block that isn't
	 * real. */
	clientsCanPay?: boolean;
}

/** ADR-0015's six named reasons a completed Engagement may carry, in the
 * order the hub's radio group offers them. */
export const endingReasons: { value: string; label: string }[] = [
	{ value: 'care_complete', label: 'The work finished as agreed' },
	{ value: 'client_withdrew', label: 'She stopped, for her own reasons' },
	{ value: 'practice_ended', label: 'The Practice ended it' },
	{ value: 'transferred', label: 'She moved to another provider' },
	{ value: 'no_response', label: 'She stopped answering' },
	{ value: 'entered_in_error', label: 'This Engagement should never have existed' }
];

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
	/** ADR-0015's derived Visit type (#281). Never chosen and never
	 * stored: the BFF computes it fresh on every read from this Visit's
	 * own instant and the Engagement's pregnancy-end date, so this screen
	 * renders it rather than deriving its own copy of the same rule. */
	type: 'prenatal' | 'birth' | 'postpartum';
}

/** The label a Visit type reads as, falling back to the raw enum value so
 * an enum this build has not met yet still prints something -- the same
 * fallback `kindLabel` (engagementRequest.ts) uses, and #262's own rule:
 * a raw enum value never reaches the screen. */
export function visitTypeLabel(type: string): string {
	return { prenatal: 'Prenatal', birth: 'Birth', postpartum: 'Postpartum' }[type] ?? type;
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
 * Moves the Engagement's status (#253). endingReason/endingNote matter
 * only when status is 'completed' -- the BFF ignores them for every
 * other target, but the page only ever sends them then. Returns the new
 * status and this caller's next set of moves, so the page can update
 * both without a second read.
 */
export async function changeEngagementStatus(
	fetcher: Fetcher,
	reference: EngagementReference,
	status: string,
	endingReason?: string,
	endingNote?: string
): Promise<Pick<EngagementSummary, 'status' | 'statusMoves'>> {
	const response = await fetcher(`${engagementURL(reference)}/status`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ status, endingReason, endingNote })
	});
	if (!response.ok) throw new Error(await apiErrorMessage(response));
	return (await response.json()) as Pick<EngagementSummary, 'status' | 'statusMoves'>;
}

/**
 * ADR-0015's three birth outcomes, in the words that ADR itself defines
 * them with -- "the baby was born alive", "the pregnancy ended without a
 * living baby", "the Engagement ended and the Practice never learned".
 *
 * The wording is the decision here, not a label lookup. This is the one
 * question in the product that may be asked of a Practice about a
 * pregnancy that ended in a loss, so no option presumes a living baby
 * and none of them says "birth" as though a birth is what happened. The
 * order is best case, loss, never learned, because that is the order
 * ADR-0015 states them in and a reader scanning three radios should not
 * meet the loss first.
 */
export const BIRTH_OUTCOMES: readonly { value: string; label: string; description: string }[] = [
	{
		value: 'live_birth',
		label: 'The baby was born alive',
		description: 'Still the answer if the baby died later. A neonatal death is a separate, later event.'
	},
	{
		value: 'loss',
		label: 'The pregnancy ended without a living baby',
		description: 'Care after a loss continues on this same Engagement.'
	},
	{
		value: 'unknown',
		label: 'The Practice never learned what happened',
		description: 'For a Client who left before the end. No date is recorded with this answer.'
	}
];

/** The recorded outcome in the reader's words, or the stored value if
 * this build has not labeled it -- lenient for the reason `roleLabel` in
 * `roles.ts` gives. */
export function birthOutcomeLabel(outcome: string): string {
	return BIRTH_OUTCOMES.find((option) => option.value === outcome)?.label ?? outcome;
}

/**
 * The two facts the birth-outcome endpoint writes and answers with.
 */
export type BirthOutcomeFacts = Pick<EngagementSummary, 'birthOutcome' | 'pregnancyEndedOn'>;

/** What is sent. `birthOutcome: null` clears a recorded pair, which the
 * BFF takes only as a correction; `correction` is the caller's explicit
 * acknowledgment that a frozen pair is being rewritten. */
export interface BirthOutcomeRequest {
	birthOutcome: string | null;
	pregnancyEndedOn?: string;
	correction?: boolean;
}

/**
 * What a birth-outcome write turned out to be: recorded, a refusal to
 * fix, or `BIRTH_OUTCOME_FROZEN` -- the press-through an Owner confirms.
 */
export type BirthOutcomeResult = Refusal | { kind: 'recorded'; facts: BirthOutcomeFacts };

export function birthOutcomeURL(reference: EngagementReference): string {
	return `${engagementURL(reference)}/birth-outcome`;
}

/**
 * Records what happened to the pregnancy, and when (#293, #943).
 *
 * `PUT` and no `Idempotency-Key`: the endpoint reads the row first and
 * answers a request for what is already there as a no-op, so a retry
 * writes nothing and raises no second audit row.
 *
 * The 409 is not folded into the error list. `BIRTH_OUTCOME_FROZEN` says
 * the pair is already recorded and that the same request re-sent with
 * `correction: true` goes through, which is a confirmation an Owner
 * presses through rather than something she can fix by retyping --
 * `refusalOrConfirmable` is the same reading #610's sign-in eviction
 * already gets, and the code is matched rather than its prose (#692),
 * because this endpoint's *other* 409 (a correction offered where
 * nothing is recorded) is not a press-through at all.
 */
export async function recordBirthOutcome(
	fetcher: Fetcher,
	reference: EngagementReference,
	request: BirthOutcomeRequest,
	/**
	 * The BFF's own field names (`engagement.BirthOutcomeRequest`'s json
	 * tags) mapped onto the caller's controls, so a refusal it names
	 * lands on the right one (#488). Defaulted to the empty map: a
	 * caller with no form -- the clear-what-was-recorded path -- has no
	 * control to point at, and those entries stay plain text.
	 */
	fieldIds: Record<string, string> = {}
): Promise<BirthOutcomeResult> {
	const response = await fetcher(birthOutcomeURL(reference), {
		method: 'PUT',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request)
	});
	if (!response.ok) {
		return refusalOrConfirmable(response, fieldIds, 'BIRTH_OUTCOME_FROZEN');
	}
	/*
	 * Normalized to `undefined`, not passed through. `Detail` omits both
	 * fields when they are null (`omitempty`), but `BirthOutcomeResponse`
	 * does not -- a successful clear answers `birthOutcome: null`, and the
	 * page's own "is anything recorded?" test is `=== undefined`. Left as
	 * `null`, a cleared Engagement would go on rendering the recorded
	 * branch: the read-back, the Owner's correct and remove controls, and
	 * a next submit carrying `correction: true` onto an empty row, which
	 * the BFF answers with the 409 that is *not* a press-through. One
	 * shape reaches the page, whichever of the two endpoints it came from.
	 */
	const body = (await response.json()) as { birthOutcome?: string | null; pregnancyEndedOn?: string | null };
	return {
		kind: 'recorded',
		facts: {
			birthOutcome: body.birthOutcome ?? undefined,
			pregnancyEndedOn: body.pregnancyEndedOn ?? undefined
		}
	};
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

/**
 * Adds a Visit to this Engagement: for a named colleague (#268), and
 * optionally scheduled (#250). The caller reloads the list itself -- this
 * only reports whether the add succeeded.
 *
 * `staffId` undefined means "for me", which is what a Doula logging her
 * own Visit sends and what this route did before the field existed.
 * `JSON.stringify` drops an undefined value, so an omitted assignee
 * reaches the BFF as an absent key rather than as an explicit null -- the
 * two mean the same thing there, but the absent key is the one the
 * pre-#268 request already sent.
 *
 * Who may name somebody else is the BFF's rule, not this function's: it
 * sends what it is given and reports the refusal. See
 * `api/internal/visit/roles.go`.
 */
export async function createVisit(
	fetcher: Fetcher,
	reference: EngagementReference,
	scheduledAt?: string,
	staffId?: string
): Promise<void> {
	const response = await fetcher(visitsURL(reference), {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ scheduledAt, staffId })
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

/**
 * Who has been offered this Engagement, or `undefined` when the caller may
 * not read that at all.
 *
 * A Doula may not read who else was offered her work, and that rule was
 * buried in a bare `catch {}` in the route: returning `undefined` rather
 * than throwing says "not for you" in the type, so the section is left out
 * rather than shown broken.
 *
 * The Doulas the section offers are no longer read here (#268). They come
 * from `staff.ts`'s `loadVisitAssigneesOrNone`, which the page fetches
 * once and hands to the Offers section and to the Visit pickers alike --
 * the same people, one request, and one place that decides what a refusal
 * means. That read moved from the Practice roster to the Engagement in
 * #911; the Offers section takes the identity half of each row, which is
 * unchanged, and ignores the per-Engagement marking the pickers need.
 *
 * `loadOffers` is injected rather than imported so this module does not
 * depend on `offer.ts` for one call -- and so a test can drive the refusal
 * without standing up that module too.
 */
export async function loadEngagementOffersOrNone(
	fetcher: Fetcher,
	reference: EngagementReference,
	loadOffers: (fetcher: Fetcher, practiceId: string, engagementId: string) => Promise<unknown[]>
): Promise<unknown[] | undefined> {
	try {
		return await loadOffers(fetcher, reference.practiceId, reference.engagementId);
	} catch {
		// Not permitted to read who was offered this work -- see above.
		return undefined;
	}
}
