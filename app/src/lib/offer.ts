/**
 * An Offer of one Engagement's work (ADR-0008, #317): an Owner or Admin
 * offers it to an existing Doula or to an email address, and the person
 * offered accepts or declines. Decoupled from SvelteKit and the DOM so it
 * can be unit-tested directly -- mirrors invoice.ts and contract.ts.
 *
 * The four decidable facts (Client first initial, general area, exact due
 * date, fee) plus free-text terms are the whole of what an Offer carries.
 * They are a copy taken at send time, never a live view of the Engagement,
 * which is what lets the same record be read by someone who has no
 * account yet.
 */

/** A minimal fetch-shaped function, injected rather than imported -- see
 * contract.ts's Fetcher for why. */
export type Fetcher = (path: string, init?: RequestInit) => Promise<Response>;

/** One Offer as either side reads it. targetName/targetAddress are filled
 * only on the Practice-side read: her own inbox does not need to tell her
 * who she is. */
export interface Offer {
	offerId: string;
	state: OfferState;
	/** The three fields #230 calls the Client's own. They come back empty
	 * once the Offer reaches a terminal state: the record of the asking
	 * survives, the Client's details stop being served. */
	clientFirstInitial: string;
	clientArea: string;
	dueDate: string;
	amountCents?: number;
	terms?: string;
	employmentType: string;
	offeredAt: string;
	expiresAt: string;
	decidedAt?: string;
	targetName?: string;
	targetAddress?: string;
}

/** The six states an Offer's lifecycle passes through -- the offer_state
 * enum (00030), mirrored so a typo is a type error. */
export type OfferState = 'offered' | 'accepted' | 'declined' | 'withdrawn' | 'superseded' | 'expired';

/** What the pre-account read serves: the same decidable facts, and
 * nothing that would name the Client, the Practice, or the Engagement. */
export interface PreAccountOffer {
	offerId: string;
	state: OfferState;
	clientFirstInitial: string;
	clientArea: string;
	dueDate: string;
	amountCents?: number;
	terms?: string;
	expiresAt: string;
}

/** The body of a make-an-offer request. Exactly one of staffId and email
 * is set. Employment type is never sent: it is read off her Membership
 * for a staffId target, and an emailed Invitation always joins her as a
 * contractor, which is why that path always carries a fee. */
export interface NewOffer {
	staffId?: string;
	email?: string;
	amountCents?: number;
	terms?: string;
	clientFirstInitial: string;
	clientArea: string;
	dueDate: string;
}

/** Which Badge variant each state wears -- kept beside the labels so the
 * two never drift, and so neither screen owns a copy of the other's. */
export const offerStateVariants: Record<OfferState, 'info' | 'success' | 'warning' | 'neutral'> = {
	offered: 'info',
	accepted: 'success',
	declined: 'neutral',
	withdrawn: 'neutral',
	superseded: 'neutral',
	expired: 'warning'
};

/** Human-readable labels for each state, so a list reads as a story
 * rather than as enum values. */
export const offerStateLabels: Record<OfferState, string> = {
	offered: 'Awaiting a decision',
	accepted: 'Accepted',
	declined: 'Declined',
	withdrawn: 'Withdrawn',
	superseded: 'Taken by someone else',
	expired: 'Expired'
};

function engagementOffersPath(practiceId: string, engagementId: string): string {
	return `/api/practices/${practiceId}/engagements/${engagementId}/offers`;
}

function offerActionPath(practiceId: string, offerId: string, action: string): string {
	return `/api/practices/${practiceId}/offers/${offerId}/${action}`;
}

async function readOrThrow<T>(response: Response): Promise<T> {
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return response.json() as Promise<T>;
}

/** Loads every Offer made on one Engagement, newest first -- the
 * make-an-offer screen's own list of who has been asked and what each of
 * them said. Owner and Admin only. Only the first page: the BFF paginates
 * for the reason docs/api-design.md gives, not because a fan-out ever
 * reaches a second page (see invoice.ts's loadInvoices). */
export async function loadEngagementOffers(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string
): Promise<Offer[]> {
	const body = await readOrThrow<{ items: Offer[] }>(
		await fetcher(engagementOffersPath(practiceId, engagementId))
	);
	return body.items;
}

/**
 * Loads the caller's own Offers, open and past -- her inbox. First page
 * only, same as loadEngagementOffers.
 */
export async function loadInbox(fetcher: Fetcher, practiceId: string): Promise<Offer[]> {
	const body = await readOrThrow<{ items: Offer[] }>(await fetcher(`/api/practices/${practiceId}/offers`));
	return body.items;
}

/** Makes an Offer. Throws with the response body text on refusal -- a
 * missing fee for a contractor, an address that already holds a
 * membership, a target who is not a Doula. */
export async function createOffer(
	fetcher: Fetcher,
	practiceId: string,
	engagementId: string,
	offer: NewOffer
): Promise<{ offerId: string; expiresAt: string }> {
	return readOrThrow(
		await fetcher(engagementOffersPath(practiceId, engagementId), {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(offer)
		})
	);
}

/** Decides one of the caller's own Offers: 'accept' or 'decline'.
 * Accepting is what mints her attachment to the Engagement. */
export async function decideOffer(
	fetcher: Fetcher,
	practiceId: string,
	offerId: string,
	action: 'accept' | 'decline'
): Promise<{ offerId: string; state: OfferState }> {
	return readOrThrow(await fetcher(offerActionPath(practiceId, offerId, action), { method: 'POST' }));
}

/** Takes an Offer back. The Practice's answer to a fact that moved: an
 * Offer is never edited after it is sent, so this is half of
 * withdraw-and-re-offer. */
export async function withdrawOffer(
	fetcher: Fetcher,
	practiceId: string,
	offerId: string
): Promise<{ offerId: string; state: OfferState }> {
	return readOrThrow(await fetcher(offerActionPath(practiceId, offerId, 'withdraw'), { method: 'POST' }));
}

/** Reads an Offer with the emailed link's token and the six-digit code,
 * before the reader has any account. Uses a bare fetcher with no session:
 * the two credentials are the whole of the authentication. */
export async function loadPreAccountOffer(
	fetcher: Fetcher,
	offerId: string,
	token: string,
	code: string
): Promise<PreAccountOffer> {
	const query = new URLSearchParams({ token, code });
	return readOrThrow(await fetcher(`/api/offers/${offerId}?${query.toString()}`));
}

/** Declines an Offer with the same two credentials, without joining the
 * Practice first. Accepting cannot work this way -- an attachment needs a
 * person -- but saying no must not require joining. */
export async function declinePreAccountOffer(
	fetcher: Fetcher,
	offerId: string,
	token: string,
	code: string
): Promise<{ offerId: string; state: OfferState }> {
	return readOrThrow(
		await fetcher(`/api/offers/${offerId}/decline`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', 'X-Confirmed': 'true' },
			body: JSON.stringify({ token, code })
		})
	);
}

/** Formats an Offer's fee for display, or says so when there isn't one --
 * an employee's Offer settles her claim on the work, not her price for
 * it. The BFF omits the field entirely rather than sending a null (the
 * repo's own convention, see invoice.ts's paidAt), so undefined is the
 * only absent case to handle. */
export function formatFee(amountCents: number | undefined): string {
	if (amountCents === undefined) {
		return 'No per-Engagement fee';
	}
	return (amountCents / 100).toLocaleString('en-US', { style: 'currency', currency: 'USD' });
}

/**
 * Reports whether an Offer is still open and so still decidable.
 */
export function isOpen(offer: { state: OfferState }): boolean {
	return offer.state === 'offered';
}
