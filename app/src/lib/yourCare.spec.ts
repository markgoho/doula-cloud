import { describe, expect, it } from 'vitest';

import { formatCalendarDay, formatInstant, formatPortalVisit } from './dates.js';
import type { PortalVisit } from './portalVisits.js';
import { birthPlanCard, contractCard, messagesCard, nextVisit, whatHappensNext } from './yourCare.js';

const visit = (overrides: Partial<PortalVisit> = {}): PortalVisit => ({
	visitId: 'visit-1',
	scheduledAt: '2027-02-18T19:00:00Z',
	doulaName: 'Priya Raman',
	hasHappened: false,
	...overrides
});

describe('nextVisit', () => {
	it('is the soonest Visit that has not happened, whatever order the page arrived in', () => {
		const later = visit({ visitId: 'later', scheduledAt: '2027-04-20T15:00:00Z' });
		const soonest = visit({ visitId: 'soonest' });
		const past = visit({ visitId: 'past', scheduledAt: '2026-08-18T14:30:00Z', hasHappened: true });

		expect(nextVisit([later, soonest, past])?.visitId).toBe('soonest');
	});

	it('is nothing when every Visit has happened', () => {
		expect(nextVisit([visit({ hasHappened: true })])).toBeUndefined();
	});
});

describe('whatHappensNext', () => {
	it('names who is coming and when, then the due date', () => {
		const next = visit();

		expect(whatHappensNext(next, '2027-03-01')).toEqual({
			lead: `Priya Raman is coming to you on ${formatPortalVisit(next.scheduledAt, false)}.`,
			detail: `Your due date is ${formatCalendarDay('2027-03-01')}.`
		});
	});

	// CB-G5: nothing booked promises nothing, and no due date (after a
	// loss, or on a postpartum-only Engagement) leaves the line out.
	it('says nothing is booked, and leaves out a due date there is none of', () => {
		expect(whatHappensNext(undefined, undefined)).toEqual({ lead: 'No visit is booked yet.' });
	});
});

describe('birthPlanCard', () => {
	it('says when she confirmed she read it', () => {
		expect(birthPlanCard({ acknowledgedAt: '2026-08-20T12:00:00Z' })).toEqual({
			label: 'Read your birth plan',
			description: `You confirmed you read it on ${formatInstant('2026-08-20T12:00:00Z')}.`
		});
	});

	it('asks her to read one she has not confirmed since it last changed', () => {
		expect(birthPlanCard({})).toEqual({
			label: 'Read your birth plan',
			description: 'It is ready for you to read.'
		});
	});

	it('says nothing is written yet when there is no plan', () => {
		expect(birthPlanCard(undefined)).toEqual({
			label: 'Read your birth plan',
			description: 'Nothing is written in it yet.'
		});
	});
});

describe('contractCard', () => {
	it('says when she signed it', () => {
		expect(contractCard({ status: 'signed', signedAt: '2026-08-14T15:00:00Z' }, 'Rooted Birth Collective')).toEqual({
			label: 'Read what you signed',
			description: `You signed this on ${formatInstant('2026-08-14T15:00:00Z')}.`
		});
	});

	// signed_at is stamped in the same write as the status, so this is a
	// row the model should never hold; it still reads true if it does.
	it('says she signed it when the instant is missing', () => {
		expect(contractCard({ status: 'signed' }, 'Rooted Birth Collective').description).toBe('You signed this.');
	});

	it('asks her to sign one that waits on her', () => {
		expect(contractCard({ status: 'sent' }, 'Rooted Birth Collective')).toEqual({
			label: 'Read and sign your contract',
			description: 'It is ready for you to sign.'
		});
	});

	// The Contract page's own voided notice, word for word: it claims
	// nothing about money, because voiding touches no Invoice.
	it('says the Practice ended one that was voided', () => {
		expect(contractCard({ status: 'voided' }, 'Rooted Birth Collective')).toEqual({
			label: 'Read your contract',
			description: 'Rooted Birth Collective ended this contract.'
		});
	});

	it('offers no link when none has been sent', () => {
		expect(contractCard(undefined, 'Rooted Birth Collective')).toEqual({
			description: 'No contract has been sent for your care yet.'
		});
	});
});

describe('messagesCard', () => {
	const message = {
		messageId: 'message-1',
		senderType: 'staff',
		senderName: 'Priya Raman',
		body: 'Thursday works. See you at 2.',
		createdAt: '2026-08-28T15:00:00Z'
	};

	it('quotes the newest Message and who wrote it', () => {
		expect(messagesCard(message)).toEqual({
			label: 'Open your messages',
			description: `From Priya Raman on ${formatInstant(message.createdAt)}: “Thursday works. See you at 2.”`
		});
	});

	it('calls her own Message yours', () => {
		expect(messagesCard({ ...message, senderType: 'client', senderName: 'Camille' }).description).toBe(
			`From you on ${formatInstant(message.createdAt)}: “Thursday works. See you at 2.”`
		);
	});

	it('names the file when the Message is a file alone', () => {
		expect(messagesCard({ ...message, body: '', attachmentFilename: 'scan.pdf' }).description).toBe(
			`From Priya Raman on ${formatInstant(message.createdAt)}: scan.pdf`
		);
	});

	it('says there are none yet', () => {
		expect(messagesCard(undefined)).toEqual({ label: 'Open your messages', description: 'No messages yet.' });
	});
});
