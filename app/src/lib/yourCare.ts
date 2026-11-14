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

import { NO_CONTRACT_MESSAGE, contractVoidedNotice, engagementStatusLabel } from './clientRegister.js';
import { formatCalendarDay, formatInstant, formatPortalVisit } from './dates.js';
import { formatMoney } from './money.js';
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

/** "What happens next", line by line: `ended` once care has ended, `lead`
 * for who is coming, `detail` for the due date. */
export interface WhatHappensNext {
	ended?: string;
	lead?: string;
	detail?: string;
}

/** "What happens next": that her care has ended, when it has, then who
 * is coming and when, then the due date where there is one.
 *
 * Ended care leads with the register's one label for a `completed`
 * Engagement, "Care ended" (#2217), the words the portal root list
 * already shows, true for every Client after a loss included (ADR-0015).
 * A Visit still booked stays named under it: completion ends no Visit
 * (ADR-0015), so the booking is the Practice's and the line says what the
 * record holds, as "still booked" so it does not read against "Care ended"
 * (#2658). With nothing booked the ended line stands alone, since
 * "No visit is booked yet." promises a booking ended care does not hold.
 *
 * The BFF already leaves the due date out once any birth outcome is
 * recorded, a loss (#2003) or a live birth (#2211) alike, and a completed
 * Engagement always carries one (00094), so this asks nothing about her
 * history. */
export function whatHappensNext(
	next: PortalVisit | undefined,
	dueDate: string | undefined,
	timeZone: string,
	hasCareEnded = false
): WhatHappensNext {
	const out: WhatHappensNext = hasCareEnded ? { ended: engagementStatusLabel('completed') } : {};
	if (next) {
		const when = formatPortalVisit(next.scheduledAt, false, timeZone);
		if (hasCareEnded) {
			out.lead = next.doulaName
				? `Your next visit is still booked: ${next.doulaName} is coming on ${when}.`
				: `Your next visit is still booked, on ${when}.`;
		} else {
			out.lead = next.doulaName ? `${next.doulaName} is coming to you on ${when}.` : `Your next visit is on ${when}.`;
		}
	} else if (!hasCareEnded) {
		out.lead = NO_NEXT_VISIT_MESSAGE;
	}
	if (dueDate) out.detail = `Your due date is ${formatCalendarDay(dueDate)}.`;
	return out;
}

/** The link text for money she still owes, first in "What happens next"
 * (#2697): one open Invoice names its amount and is linked to directly,
 * two or more name the total and are linked to the list. Nothing at all
 * for none, so a Client who owes nothing reads nothing about money. */
export function owedNextLine(openInvoiceCount: number, totalToPayCents: number, currency = 'usd'): string | undefined {
	if (openInvoiceCount < 1) return undefined;
	const amount = formatMoney(totalToPayCents, currency);
	if (openInvoiceCount === 1) return `Your invoice for ${amount} is ready to pay`;
	return `You have ${openInvoiceCount} invoices to pay, ${amount} in all`;
}

/** Her Birth Plan. Its acknowledgment is cleared by a Staff edit, so it
 * always means "read since it last changed" (GLOSSARY.md's Birth Plan
 * entry), and its absence asks her to read it. */
export function birthPlanCard(glance: BirthPlanGlance | undefined, timeZone: string): CareCard {
	const label = 'Read your birth plan';
	if (!glance) return { label, description: 'Nothing is written in it yet.' };
	if (glance.acknowledgedAt) {
		return { label, description: `You confirmed you read it on ${formatInstant(glance.acknowledgedAt, timeZone)}.` };
	}
	return { label, description: 'It is ready for you to read.' };
}

/**
Her Contract, where it stands.
*/
export function contractCard(glance: ContractGlance | undefined, practiceName: string, timeZone: string): CareCard {
	if (!glance) return { description: NO_CONTRACT_MESSAGE };
	if (glance.status === 'signed') {
		const description = glance.signedAt ? `You signed this on ${formatInstant(glance.signedAt, timeZone)}.` : 'You signed this.';
		return { label: 'Read what you signed', description };
	}
	if (glance.status === 'voided') {
		return { label: 'Read your contract', description: contractVoidedNotice(practiceName) };
	}
	return { label: 'Read and sign your contract', description: 'It is ready for you to sign.' };
}

/** Her thread: the newest Message, who sent it and when -- "you" for
 * her own -- quoted, or the file's name when it carried only a file. */
export function messagesCard(last: LastMessage | undefined, timeZone: string): CareCard {
	const label = 'Open your messages';
	if (!last) return { label, description: 'No messages yet.' };
	const who = last.senderType === 'client' ? 'you' : last.senderName;
	const what = last.body ? `“${last.body}”` : last.attachmentFilename;
	return { label, description: `From ${who} on ${formatInstant(last.createdAt, timeZone)}: ${what}` };
}
