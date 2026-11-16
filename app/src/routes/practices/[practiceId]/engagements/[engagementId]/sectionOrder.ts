/*
 * The order of the Engagement record's sections (#2755): what this phase of
 * care asks someone to do now comes first, and a section the Engagement
 * read says is empty sinks below the filled ones. It reads the Engagement
 * read alone, so the order is settled on first paint and nothing moves when
 * a later read lands. Pure: the clock is passed in as today's calendar day.
 */

export type SectionHeading =
	| 'On call'
	| 'Birth outcome'
	| 'Contract'
	| 'Visits'
	| 'Care Plan'
	| 'Birth Plan'
	| 'Invoices'
	| 'Offers'
	| 'Doula fees'
	| 'Note when work started'
	| 'Messages'
	| 'Activity';

export type CarePhase = 'intake' | 'before-on-call' | 'on-call' | 'postpartum' | 'completed' | 'loss';

export interface SectionOrderInput {
	status: string;
	kind: string;
	birthOutcome?: string;
	/**
	Today in the Practice's zone, `YYYY-MM-DD`.
	*/
	today: string;
	dueDate?: string;
	onCallWindow?: { from: string; to: string };
	/**
	A birth with no Doula on it is listed as needing cover from today under
	the grant-day rule (GLOSSARY, On-call window), which says nothing of how
	far along the care is: the order does not call it on call.
	*/
	hasDoula: boolean;
	hasVisits: boolean;
	hasMessages: boolean;
	hasInvoices: boolean;
	hasOffers: boolean;
	hasContract: boolean;
	hasCarePlan: boolean;
	hasBirthPlan: boolean;
	/**
	Offers and Doula fees are an Owner's or an Admin's.
	*/
	isOffersVisible: boolean;
	isDoulaFeesVisible: boolean;
	hasStartNote: boolean;
}

/**
 * The first that matches wins: a loss, a completed Engagement, postpartum
 * care (or a live birth), birth care whose on-call days have begun and has
 * no outcome, intake, and otherwise active birth care before its days.
 */
export function carePhaseOf(input: SectionOrderInput): CarePhase {
	if (input.birthOutcome === 'loss') return 'loss';
	if (input.status === 'completed') return 'completed';
	if (input.birthOutcome !== undefined || input.kind === 'postpartum') return 'postpartum';
	if (input.hasDoula && input.onCallWindow && input.today >= input.onCallWindow.from) return 'on-call';
	if (input.status === 'intake') return 'intake';
	return 'before-on-call';
}

const TABLE: Record<CarePhase, readonly SectionHeading[]> = {
	intake: ['Contract', 'Messages', 'Visits', 'Invoices', 'Birth Plan', 'Care Plan', 'On call', 'Offers', 'Doula fees', 'Birth outcome'],
	'before-on-call': ['Visits', 'Birth Plan', 'Messages', 'Care Plan', 'Contract', 'Invoices', 'On call', 'Offers', 'Doula fees', 'Birth outcome'],
	'on-call': ['On call', 'Birth Plan', 'Visits', 'Messages', 'Care Plan', 'Contract', 'Invoices', 'Offers', 'Doula fees', 'Birth outcome'],
	postpartum: ['Visits', 'Care Plan', 'Messages', 'Invoices', 'Birth outcome', 'Contract', 'Birth Plan', 'Offers', 'Doula fees'],
	completed: ['Invoices', 'Birth outcome', 'Visits', 'Care Plan', 'Birth Plan', 'Contract', 'Messages', 'Offers', 'Doula fees'],
	loss: ['Messages', 'Visits', 'Care Plan', 'Birth outcome', 'Invoices', 'Contract', 'Offers', 'Doula fees', 'Birth Plan']
};

/**
After an outcome or completion On call keeps one line, just before Activity.
*/
const ENDED: ReadonlySet<CarePhase> = new Set(['postpartum', 'completed', 'loss']);

export function sectionOrder(input: SectionOrderInput): SectionHeading[] {
	const phase = carePhaseOf(input);
	const isDueReached = input.kind === 'birth' && input.dueDate !== undefined && input.today >= input.dueDate;
	const isOutcomeRecorded = input.birthOutcome !== undefined;

	let table = [...TABLE[phase]];
	// From the due date the outcome is what someone opens the page for.
	if (phase === 'on-call' && isDueReached && !isOutcomeRecorded) {
		table = table.filter((heading) => heading !== 'Birth outcome');
		table.splice(1, 0, 'Birth outcome');
	}

	const isEmpty = (heading: SectionHeading): boolean => {
		switch (heading) {
			case 'Contract': {
				return !input.hasContract;
			}
			case 'Visits': {
				return !input.hasVisits;
			}
			case 'Messages': {
				return !input.hasMessages;
			}
			case 'Invoices': {
				return !input.hasInvoices;
			}
			case 'Care Plan': {
				return !input.hasCarePlan;
			}
			case 'Birth Plan': {
				return !input.hasBirthPlan;
			}
			case 'Offers': {
				return !input.hasOffers;
			}
			case 'On call': {
				// Before its on-call days there is nothing to say about tonight.
				return phase !== 'on-call';
			}
			case 'Birth outcome': {
				return !isOutcomeRecorded && !isDueReached;
			}
			default: {
				return false;
			}
		}
	};

	const shown = table.filter((heading) => {
		if (heading === 'Offers') return input.isOffersVisible;
		if (heading === 'Doula fees') return input.isDoulaFeesVisible;
		return true;
	});

	const filled = shown.filter((heading) => !isEmpty(heading));
	const empty = shown.filter((heading) => isEmpty(heading));
	const order: SectionHeading[] = [...filled, ...empty];
	if (ENDED.has(phase)) order.push('On call');
	if (input.hasStartNote) order.push('Note when work started');
	order.push('Activity');
	return order;
}
