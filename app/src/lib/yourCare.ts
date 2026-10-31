/**
 * The Client's "Your care" page (#2007): what happens next, and one card
 * each for her Birth Plan, her Contract and her Messages, each saying
 * where it stands in one line with the way to it.
 *
 * The facts come from `portal.Detail` (api/internal/portal/detail.go),
 * which reads one line from each owning package rather than the whole
 * document, so the hub carries no Contract prose, no Birth Plan answers
 * and no page of Messages to say one sentence about each.
 *
 * Every sentence is fixed wording for every Client (ADR-0005): it says
 * what the record holds and nothing it does not. In particular a voided
 * Contract reads the Contract page's own notice, which claims nothing
 * about money, and nothing promises a Visit nobody has booked (CB-G5).
 */

import { NO_CONTRACT_MESSAGE, contractVoidedNotice } from './clientRegister.js';
import { formatCalendarDay, formatInstant, formatPortalVisit } from './dates.js';
import { inReadingOrder, type PortalVisit } from './portalVisits.js';

/**
`portal.Detail.contract`: absent until a Contract reaches her.
*/
export interface ContractGlance {
	status: string;
	signedAt?: string;
}

/** `portal.Detail.birthPlan`: absent until a Birth Plan is started, and
 * wherever none is offered. */
export interface BirthPlanGlance {
	acknowledgedAt?: string;
}

/** `portal.Detail.lastMessage`: the newest Message, as her own thread
 * reads it. */
export interface LastMessage {
	messageId: string;
	senderType: string;
	senderName: string;
	body: string;
	attachmentFilename?: string;
	createdAt: string;
}

/**
A card's link and the one line under it. No label means no link.
*/
export interface CareCard {
	label?: string;
	description: string;
}

export const NEXT_HEADING = 'What happens next';

/** What happens next with no Visit to come. Not the Visits list's own
 * empty line, which sits below it on the same page; like that line it
 * reports her list and promises nobody (CB-G5). */
export const NO_NEXT_VISIT_MESSAGE = 'No visit is booked yet.';

/**
The soonest Visit still to come, the one "What happens next" names.
*/
export function nextVisit(items: PortalVisit[]): PortalVisit | undefined {
	return inReadingOrder(items).find((each) => !each.hasHappened);
}

/** "What happens next": who is coming and when, then the due date where
 * there is one. The BFF already leaves the due date out once any birth
 * outcome is recorded, a loss (#2003) or a live birth (#2211) alike, so
 * this asks nothing about her history. */
export function whatHappensNext(next: PortalVisit | undefined, dueDate: string | undefined): { lead: string; detail?: string } {
	const lead = next
		? `${next.doulaName} is coming to you on ${formatPortalVisit(next.scheduledAt, false)}.`
		: NO_NEXT_VISIT_MESSAGE;
	return dueDate ? { lead, detail: `Your due date is ${formatCalendarDay(dueDate)}.` } : { lead };
}

/** Her Birth Plan. Its acknowledgment is cleared by a Staff edit, so it
 * always means "read since it last changed" (GLOSSARY.md's Birth Plan
 * entry), and its absence asks her to read it. */
export function birthPlanCard(glance: BirthPlanGlance | undefined): CareCard {
	const label = 'Read your birth plan';
	if (!glance) return { label, description: 'Nothing is written in it yet.' };
	if (glance.acknowledgedAt) {
		return { label, description: `You confirmed you read it on ${formatInstant(glance.acknowledgedAt)}.` };
	}
	return { label, description: 'It is ready for you to read.' };
}

/**
Her Contract, where it stands.
*/
export function contractCard(glance: ContractGlance | undefined, practiceName: string): CareCard {
	if (!glance) return { description: NO_CONTRACT_MESSAGE };
	if (glance.status === 'signed') {
		const description = glance.signedAt ? `You signed this on ${formatInstant(glance.signedAt)}.` : 'You signed this.';
		return { label: 'Read what you signed', description };
	}
	if (glance.status === 'voided') {
		return { label: 'Read your contract', description: contractVoidedNotice(practiceName) };
	}
	return { label: 'Read and sign your contract', description: 'It is ready for you to sign.' };
}

/** Her thread: the newest Message, who sent it and when -- "you" for
 * her own -- quoted, or the file's name when it carried only a file. */
export function messagesCard(last: LastMessage | undefined): CareCard {
	const label = 'Open your messages';
	if (!last) return { label, description: 'No messages yet.' };
	const who = last.senderType === 'client' ? 'you' : last.senderName;
	const what = last.body ? `“${last.body}”` : last.attachmentFilename;
	return { label, description: `From ${who} on ${formatInstant(last.createdAt)}: ${what}` };
}
