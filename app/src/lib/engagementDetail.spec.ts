import { describe, expect, it, vi } from 'vitest';

import { errorsFromCause } from './formErrors.js';
import { jsonResponse } from './testResponse.js';
import {
	attachDoula,
	attachDoulaWords,
	attachSelf,
	birthOutcomeLabel,
	birthOutcomeURL,
	changeEngagementDueDate,
	changeEngagementKind,
	changeEngagementStatus,
	createVisit,
	doulaSummaryItem,
	downloadAttachment,
	engagementStatusText,
	loadAttachmentPreviews,
	loadEngagement,
	loadEngagementOffersOrNone,
	loadMessagesPage,
	loadVisitsPage,
	messagesURL,
	portalInviteURL,
	reassignVisit,
	recordBirthOutcome,
	saveVisitNotes,
	scheduleVisit,
	sendMessage,
	sendPortalInvite,
	sortVisits,
	visitsURL,
	visitTypeLabel,
	yourFeeRows,
	type EngagementReference,
	type Visit
} from './engagementDetail.js';

vi.mock('./api.js', () => ({
	apiFetchWithSession: vi.fn()
}));

const reference: EngagementReference = { practiceId: 'practice-1', engagementId: 'engagement-1' };
const base = '/api/practices/practice-1/engagements/engagement-1';

describe('loadEngagement', () => {
	it('reads the Engagement', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ clientName: 'Tasha Bell' }));

		const detail = await loadEngagement(fetcher, reference);

		expect(fetcher).toHaveBeenCalledWith(base);
		expect(detail.clientName).toBe('Tasha Bell');
	});

	it('throws a refusal, so load turns it into the route error', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted to read this', 403));

		await expect(loadEngagement(fetcher, reference)).rejects.toThrow('not permitted to read this');
	});
});

describe('loadVisitsPage', () => {
	it('reads the first page without a cursor parameter', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadVisitsPage(fetcher, reference, '');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits`);
	});

	it('encodes a cursor onto the next page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadVisitsPage(fetcher, reference, 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits?cursor=a%20b%2Fc`);
	});

	it('throws a refusal, so PaginatedList can catch it', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadVisitsPage(fetcher, reference, '')).rejects.toThrow('nope');
	});

	it('keeps the endpoint order, newest first', async () => {
		const visits: Visit[] = [
			{ visitId: 'v2', staffId: 's1', staffName: 'Maya', createdAt: '2027-02-01', type: 'prenatal' },
			{ visitId: 'v1', staffId: 's1', staffName: 'Maya', createdAt: '2027-01-01', type: 'prenatal' }
		];
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: visits, hasMore: false }));

		const page = await loadVisitsPage(fetcher, reference, '');

		expect(page.items.map((v) => v.visitId)).toEqual(['v2', 'v1']);
	});
});

describe('loadMessagesPage', () => {
	// The one list on this page whose order is flipped: the BFF answers
	// newest-first like every other cursor list, but a thread reads
	// oldest-at-the-top. This used to be a bare .toReversed() inline in the
	// route, where nothing said why.
	it('reverses the page, because a thread reads oldest first', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse({
				items: [{ messageId: 'newest' }, { messageId: 'oldest' }],
				nextCursor: 'c1',
				hasMore: true
			})
		);

		const page = await loadMessagesPage<{ messageId: string }>(fetcher, reference, '');

		expect(page.items.map((m) => m.messageId)).toEqual(['oldest', 'newest']);
		expect(page.nextCursor).toBe('c1');
		expect(page.hasMore).toBe(true);
	});

	// Older messages page backwards from a cursor, which is the prepend
	// this list does instead of PaginatedList's append.
	it('encodes a cursor onto the older page', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ items: [], hasMore: false }));

		await loadMessagesPage(fetcher, reference, 'a b/c');

		expect(fetcher).toHaveBeenCalledWith(`${base}/messages?cursor=a%20b%2Fc`);
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(loadMessagesPage(fetcher, reference, '')).rejects.toThrow('nope');
	});
});

function blobResponse(): Response {
	return new Response(new Blob(['x'], { type: 'image/png' }), { status: 200 });
}

describe('loadAttachmentPreviews', () => {
	it('fetches only the image attachments not already loaded', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[
				{ messageId: 'has-image', attachmentContentType: 'image/png' },
				{ messageId: 'has-pdf', attachmentContentType: 'application/pdf' },
				{ messageId: 'no-attachment' },
				{ messageId: 'already-here', attachmentContentType: 'image/jpeg' }
			],
			{ 'already-here': 'blob:existing' }
		);

		expect(fetcher).toHaveBeenCalledTimes(1);
		expect(fetcher).toHaveBeenCalledWith(`${messagesURL(reference)}/has-image/attachment`);
		expect(Object.keys(loaded)).toEqual(['has-image']);
	});

	// A thumbnail that will not load is a missing image, not a broken
	// thread, so the refusal is skipped rather than thrown.
	it('skips an attachment the caller may not read', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[{ messageId: 'refused', attachmentContentType: 'image/png' }],
			{}
		);

		expect(loaded).toEqual({});
	});

	it('returns the new URLs rather than mutating what it was given', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());
		const existing = { 'already-here': 'blob:existing' };

		const loaded = await loadAttachmentPreviews(
			fetcher,
			reference,
			[{ messageId: 'fresh', attachmentContentType: 'image/png' }],
			existing
		);

		expect(existing).toEqual({ 'already-here': 'blob:existing' });
		expect(Object.keys(loaded)).toEqual(['fresh']);
	});
});

describe('loadEngagementOffersOrNone', () => {
	it('returns the offers the injected read answers with', async () => {
		const fetcher = vi.fn();
		const loadOffers = vi.fn().mockResolvedValue([{ offerId: 'o1' }]);

		expect(await loadEngagementOffersOrNone(fetcher, reference, loadOffers)).toEqual([
			{ offerId: 'o1' }
		]);
		expect(loadOffers).toHaveBeenCalledWith(fetcher, 'practice-1', 'engagement-1');
	});

	// The read is Owner/Admin. Its refusal is what tells the page the
	// caller is a Doula, and a Doula may not read who else was offered her
	// work -- so the section is left out rather than shown broken.
	it('answers undefined when the Offers read refuses', async () => {
		const fetcher = vi.fn();
		const loadOffers = vi.fn().mockRejectedValue(new Error('not permitted to read this'));

		expect(await loadEngagementOffersOrNone(fetcher, reference, loadOffers)).toBeUndefined();
	});
});

describe('changeEngagementStatus', () => {
	it('patches the status alone for a move that is not completing', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ status: 'active', statusMoves: ['completed'] }));

		const result = await changeEngagementStatus(fetcher, reference, 'active');

		expect(fetcher).toHaveBeenCalledWith(`${base}/status`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ status: 'active' })
		});
		expect(result).toEqual({ status: 'active', statusMoves: ['completed'] });
	});

	it('sends the ending reason and note when completing', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ status: 'completed', statusMoves: ['active'] }));

		await changeEngagementStatus(fetcher, reference, 'completed', 'care_complete', 'Baby arrived safely.');

		expect(fetcher).toHaveBeenCalledWith(`${base}/status`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				status: 'completed',
				endingReason: 'care_complete',
				endingNote: 'Baby arrived safely.'
			})
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('endingReason is required', 400));

		await expect(changeEngagementStatus(fetcher, reference, 'completed')).rejects.toThrow(
			'endingReason is required'
		);
	});
});

describe('changeEngagementKind', () => {
	it('puts the target kind', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ engagementId: 'engagement-1', kind: 'postpartum' }));

		const result = await changeEngagementKind(fetcher, reference, 'postpartum');

		expect(fetcher).toHaveBeenCalledWith(`${base}/kind`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ kind: 'postpartum' })
		});
		expect(result).toEqual({ engagementId: 'engagement-1', kind: 'postpartum' });
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('only a Practice Owner, Admin or Doula can do that', 403));

		await expect(changeEngagementKind(fetcher, reference, 'postpartum')).rejects.toThrow(
			'only a Practice Owner, Admin or Doula can do that'
		);
	});
});

describe('changeEngagementDueDate', () => {
	it('puts the new due date, and answers with the one now held', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ engagementId: 'engagement-1', dueDate: '2027-03-15' }));

		const result = await changeEngagementDueDate(fetcher, reference, '2027-03-15');

		expect(fetcher).toHaveBeenCalledWith(`${base}/due-date`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ dueDate: '2027-03-15' })
		});
		expect(result).toEqual({ dueDate: '2027-03-15' });
	});

	it('throws a refusal that keeps the field it names', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'The due date must be a calendar day, YYYY-MM-DD',
					details: { dueDate: 'Enter the due date as a real date, like 2027-04-23' }
				},
				400
			)
		);

		let refusal: unknown;
		try {
			await changeEngagementDueDate(fetcher, reference, 'soon');
		} catch (error) {
			refusal = error;
		}

		expect(errorsFromCause(refusal, { dueDate: 'due-date-month' })).toEqual([
			{ message: 'Enter the due date as a real date, like 2027-04-23', targetId: 'due-date-month' }
		]);
	});
});

// #1598: the reader puts herself on the Engagement.
describe('attachSelf', () => {
	it('puts with no body, and answers who is on the Engagement', async () => {
		const answer = {
			engagementId: 'engagement-1',
			doulas: [{ staffId: 'staff-1', name: 'Hana Kim' }],
			canAttachSelf: false
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(answer));

		const result = await attachSelf(fetcher, reference);

		expect(fetcher).toHaveBeenCalledWith(`${base}/attachments/me`, { method: 'PUT' });
		expect(result).toEqual({ doulas: answer.doulas, canAttachSelf: false });
	});

	it('throws a refusal', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('You contract for this Practice, so you go on an Engagement by accepting an Offer.', 403));

		await expect(attachSelf(fetcher, reference)).rejects.toThrow(
			'You contract for this Practice, so you go on an Engagement by accepting an Offer.'
		);
	});
});

// #2230: an Owner or an Admin puts a named Doula on the Engagement.
describe('attachDoula', () => {
	it('puts with no body at her own path, and answers who is on the Engagement', async () => {
		const answer = {
			engagementId: 'engagement-1',
			doulas: [{ staffId: 'staff-2', name: 'Priya Raman' }],
			canAttachSelf: true
		};
		const fetcher = vi.fn().mockResolvedValue(jsonResponse(answer));

		const result = await attachDoula(fetcher, reference, 'staff-2');

		expect(fetcher).toHaveBeenCalledWith(`${base}/attachments/staff-2`, { method: 'PUT' });
		expect(result).toEqual({ doulas: answer.doulas, canAttachSelf: true });
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('Lena Vasquez contracts for this Practice.', 422));

		await expect(attachDoula(fetcher, reference, 'staff-3')).rejects.toThrow('Lena Vasquez contracts for this Practice.');
	});
});

// #2569: the question says who stays, because saving adds.
const on = (count: number) =>
	['Kimiko Nakashima', 'Priya Raman', 'Joss Adeyemi', 'Delia Marchetti', 'Rowan Petrosyan']
		.slice(0, count)
		.map((name, index) => ({ staffId: `staff-${index}`, name }));

describe('attachDoulaWords', () => {
	it('keeps "Who is the Doula?" and "Save the Doula" where nobody is on', () => {
		expect(attachDoulaWords([])).toEqual({ question: 'Who is the Doula?', button: 'Save the Doula' });
	});

	it.each([
		[1, 'Who joins Kimiko Nakashima?'],
		[2, 'Who joins Kimiko Nakashima and Priya Raman?'],
		[3, 'Who joins Kimiko Nakashima, Priya Raman and Joss Adeyemi?'],
		[4, 'Who joins Kimiko Nakashima, Priya Raman, Joss Adeyemi and 1 other?'],
		[5, 'Who joins Kimiko Nakashima, Priya Raman, Joss Adeyemi and 2 others?']
	])('names who is on with %i on it', (count, question) => {
		expect(attachDoulaWords(on(count))).toEqual({ question, button: 'Add this Doula' });
	});
});

describe('doulaSummaryItem', () => {
	const hana = { staffId: 'staff-1', name: 'Hana Kim' };
	const renata = { staffId: 'staff-2', name: 'Renata Alvarez' };

	it('says "No Doula yet" for a live Engagement nobody is on', () => {
		expect(doulaSummaryItem([], 'intake')).toEqual({ label: 'Doula', value: 'No Doula yet' });
		expect(doulaSummaryItem([], 'active')).toEqual({ label: 'Doula', value: 'No Doula yet' });
	});

	it('names the one Doula', () => {
		expect(doulaSummaryItem([hana], 'active')).toEqual({ label: 'Doula', value: 'Hana Kim' });
	});

	it('names each Doula, in the order they were attached, under a plural label', () => {
		expect(doulaSummaryItem([hana, renata], 'active')).toEqual({
			label: 'Doulas',
			value: 'Hana Kim, Renata Alvarez'
		});
	});

	it('has no row for a completed Engagement, which holds no Attachment and will get no Doula', () => {
		expect(doulaSummaryItem([], 'completed')).toBeUndefined();
	});
});

describe('sendPortalInvite', () => {
	it('posts and returns the invite token', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ inviteToken: 'tok-1' }));

		const created = await sendPortalInvite(fetcher, reference);

		expect(fetcher).toHaveBeenCalledWith(`${base}/portal-invite`, { method: 'POST' });
		expect(created).toEqual({ inviteToken: 'tok-1' });
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(sendPortalInvite(fetcher, reference)).rejects.toThrow('nope');
	});
});

describe('createVisit', () => {
	it('posts to the Visits endpoint with no scheduledAt', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await createVisit(fetcher, reference);

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ scheduledAt: undefined, staffId: undefined })
		});
	});

	it('posts the given scheduledAt', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await createVisit(fetcher, reference, '2027-03-15T14:30:00Z');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ scheduledAt: '2027-03-15T14:30:00Z', staffId: undefined })
		});
	});

	// #268: naming a colleague at creation, with no reassign step after
	// it. An absent assignee still means "for me" -- the case above.
	it('posts the named assignee', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await createVisit(fetcher, reference, undefined, 'staff-7');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ scheduledAt: undefined, staffId: 'staff-7' })
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(createVisit(fetcher, reference)).rejects.toThrow('nope');
	});

	// #2540: who the BFF emailed about the new Visit, or nobody.
	it('hands back who was emailed', async () => {
		const notified = vi.fn().mockResolvedValue(jsonResponse({ notifiedStaffIds: ['staff-7'] }));
		await expect(createVisit(notified, reference, undefined, 'staff-7')).resolves.toEqual(['staff-7']);

		const silent = vi.fn().mockResolvedValue(jsonResponse({}));
		await expect(createVisit(silent, reference)).resolves.toEqual([]);
	});
});

describe('scheduleVisit', () => {
	it('patches the Visit with the new scheduledAt', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await scheduleVisit(fetcher, reference, 'visit-1', '2027-03-15T14:30:00Z');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits/visit-1/schedule`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ scheduledAt: '2027-03-15T14:30:00Z' })
		});
	});

	it('clears scheduledAt when given undefined', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await scheduleVisit(fetcher, reference, 'visit-1', undefined);

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits/visit-1/schedule`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ scheduledAt: undefined })
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(scheduleVisit(fetcher, reference, 'visit-1', '2027-03-15T14:30:00Z')).rejects.toThrow('nope');
	});
});

describe('saveVisitNotes', () => {
	it('patches the Visit with its new notes', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await saveVisitNotes(fetcher, reference, 'visit-1', 'Client seemed anxious about the birth plan.');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits/visit-1/notes`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ notes: 'Client seemed anxious about the birth plan.' })
		});
	});

	it('clears notes to the empty string', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await saveVisitNotes(fetcher, reference, 'visit-1', '');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits/visit-1/notes`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ notes: '' })
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(saveVisitNotes(fetcher, reference, 'visit-1', 'text')).rejects.toThrow('nope');
	});
});

describe('reassignVisit', () => {
	it('patches the Visit with the new staffId', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({}));

		await reassignVisit(fetcher, reference, 'visit-1', 'staff-2');

		expect(fetcher).toHaveBeenCalledWith(`${base}/visits/visit-1`, {
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ staffId: 'staff-2' })
		});
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(reassignVisit(fetcher, reference, 'visit-1', 'staff-2')).rejects.toThrow('nope');
	});

	// #2436: who the BFF emailed about the move, or nobody when it says none.
	it('hands back who was emailed', async () => {
		const notified = vi.fn().mockResolvedValue(jsonResponse({ notifiedStaffIds: ['staff-2', 'staff-1'] }));
		await expect(reassignVisit(notified, reference, 'visit-1', 'staff-2')).resolves.toEqual(['staff-2', 'staff-1']);

		const silent = vi.fn().mockResolvedValue(jsonResponse({}));
		await expect(reassignVisit(silent, reference, 'visit-1', 'staff-2')).resolves.toEqual([]);
	});
});

describe('sendMessage', () => {
	it('sends a JSON body when there is no attachment', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ messageId: 'm1' }));

		const created = await sendMessage(fetcher, reference, 'hello', undefined);

		expect(fetcher).toHaveBeenCalledWith(`${base}/messages`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ body: 'hello' })
		});
		expect(created).toEqual({ messageId: 'm1' });
	});

	it('sends a multipart form when there is an attachment', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ messageId: 'm2' }));
		const attachment = new File(['x'], 'photo.png', { type: 'image/png' });

		await sendMessage(fetcher, reference, 'see this', attachment);

		const [path, init] = fetcher.mock.calls[0] as [string, RequestInit];
		expect(path).toBe(`${base}/messages`);
		expect(init.method).toBe('POST');
		expect(init.body).toBeInstanceOf(FormData);
		expect((init.body as FormData).get('body')).toBe('see this');
		expect((init.body as FormData).get('attachment')).toBe(attachment);
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(sendMessage(fetcher, reference, 'hello', undefined)).rejects.toThrow('nope');
	});

	it('throws a refusal that keeps the field the BFF names', async () => {
		const words = 'The selected file must be smaller than 10MB';
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ code: 'PAYLOAD_TOO_LARGE', message: words, details: { attachment: words } }, 413));

		await expect(sendMessage(fetcher, reference, '', new File(['x'], 'big.png'))).rejects.toMatchObject({
			message: words,
			details: { attachment: words }
		});
	});
});

describe('downloadAttachment', () => {
	it('returns the attachment as a Blob', async () => {
		const fetcher = vi.fn().mockResolvedValue(blobResponse());

		const blob = await downloadAttachment(fetcher, reference, 'm1');

		expect(fetcher).toHaveBeenCalledWith(`${base}/messages/m1/attachment`);
		expect(blob).toBeInstanceOf(Blob);
	});

	it('throws a refusal', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('nope', 403));

		await expect(downloadAttachment(fetcher, reference, 'm1')).rejects.toThrow('nope');
	});
});

describe('recordBirthOutcome', () => {
	it('puts the outcome and the date, with no idempotency key', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ birthOutcome: 'loss', pregnancyEndedOn: '2026-08-14' }));

		const result = await recordBirthOutcome(fetcher, reference, {
			birthOutcome: 'loss',
			pregnancyEndedOn: '2026-08-14'
		});

		expect(fetcher).toHaveBeenCalledWith(`${base}/birth-outcome`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ birthOutcome: 'loss', pregnancyEndedOn: '2026-08-14' })
		});
		expect(result).toEqual({
			kind: 'recorded',
			facts: { birthOutcome: 'loss', pregnancyEndedOn: '2026-08-14' }
		});
	});

	// The response body, not Detail: BirthOutcomeResponse carries no
	// `omitempty`, so a clear really does answer `birthOutcome: null`, and
	// the page's own "is anything recorded?" test is `=== undefined`.
	it('normalizes a cleared pair to absent, so the page stops reading it as recorded', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			// eslint-disable-next-line unicorn/no-null
			jsonResponse({ engagementId: 'engagement-1', birthOutcome: null, pregnancyEndedOn: null })
		);

		const result = await recordBirthOutcome(fetcher, reference, {
			// eslint-disable-next-line unicorn/no-null -- the wire value for a clear.
			birthOutcome: null,
			correction: true
		});

		expect(result).toEqual({ kind: 'recorded', facts: {} });
	});

	it('reads BIRTH_OUTCOME_FROZEN as the press-through, not as an error', async () => {
		const fetcher = vi.fn().mockResolvedValue(
			jsonResponse(
				{
					code: 'BIRTH_OUTCOME_FROZEN',
					message: 'this Engagement already has a birth outcome; only a Practice Owner can correct it'
				},
				409
			)
		);

		const result = await recordBirthOutcome(fetcher, reference, { birthOutcome: 'loss' });

		expect(result).toEqual({
			kind: 'confirmable',
			message: 'this Engagement already has a birth outcome; only a Practice Owner can correct it'
		});
	});

	it('reads the endpoint other 409 as a refusal to fix, not a press-through', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(
				jsonResponse({ message: 'this Engagement has no birth outcome to correct' }, 409)
			);

		const result = await recordBirthOutcome(fetcher, reference, {
			birthOutcome: 'loss',
			correction: true
		});

		expect(result).toEqual({
			kind: 'errors',
			errors: [{ message: 'this Engagement has no birth outcome to correct' }]
		});
	});
});

describe('birthOutcomeLabel', () => {
	it('names each stored outcome in ADR-0015 own words', () => {
		expect(birthOutcomeLabel('live_birth')).toBe('The baby was born alive');
		expect(birthOutcomeLabel('loss')).toBe('The pregnancy ended without a living baby');
		expect(birthOutcomeLabel('unknown')).toBe('The Practice never learned what happened');
	});

	it('prints an outcome this build has not labeled rather than hiding it', () => {
		expect(birthOutcomeLabel('stillbirth')).toBe('stillbirth');
	});
});

describe('URL builders', () => {
	it('build every path off the one Engagement reference', () => {
		expect(visitsURL(reference)).toBe(`${base}/visits`);
		expect(messagesURL(reference)).toBe(`${base}/messages`);
		expect(portalInviteURL(reference)).toBe(`${base}/portal-invite`);
		expect(birthOutcomeURL(reference)).toBe(`${base}/birth-outcome`);
	});
});

describe('engagementStatusText', () => {
	it.each([
		['intake', 'Intake'],
		['active', 'Active'],
		['completed', 'Completed'],
		['something-new', 'something-new']
	])('labels the %s status as %s', (status, expected) => {
		expect(engagementStatusText(status)).toBe(expected);
	});
});

describe('visitTypeLabel', () => {
	it.each([
		['prenatal', 'Prenatal'],
		['birth', 'Birth'],
		['postpartum', 'Postpartum'],
		['something-new', 'something-new']
	])('labels the %s type as %s', (type, expected) => {
		expect(visitTypeLabel(type)).toBe(expected);
	});
});

function visit(visitId: string, createdAt: string, scheduledAt?: string): Visit {
	return {
		visitId,
		staffId: 'staff-1',
		staffName: 'Priya Raman',
		createdAt,
		...(scheduledAt && { scheduledAt }),
		type: 'prenatal'
	};
}

describe('sortVisits (#1999)', () => {
	it('lists the scheduled Visits newest first, then the unscheduled ones newest logged first', () => {
		const sorted = sortVisits([
			visit('logged-early', '2026-07-01T15:00:00Z'),
			visit('nov-5', '2026-08-01T15:00:00Z', '2026-11-05T15:00:00Z'),
			visit('aug-4', '2026-08-02T15:00:00Z', '2026-08-04T15:00:00Z'),
			visit('logged-late', '2026-09-01T15:00:00Z'),
			visit('aug-18', '2026-08-03T15:00:00Z', '2026-08-18T15:00:00Z')
		]);

		expect(sorted.map((entry) => entry.visitId)).toEqual(['nov-5', 'aug-18', 'aug-4', 'logged-late', 'logged-early']);
	});

	it('leaves the list it was given as it was', () => {
		const given = [visit('a', '2026-07-01T15:00:00Z'), visit('b', '2026-07-02T15:00:00Z')];
		sortVisits(given);
		expect(given.map((entry) => entry.visitId)).toEqual(['a', 'b']);
	});
});

describe('yourFeeRows', () => {
	it('puts the terms she accepted beside her fee (#2269)', () => {
		expect(yourFeeRows({ amountCents: 45_000, acceptedOn: '2026-10-27', terms: 'Backup for the birth.' })).toEqual([
			{ label: 'Your fee', value: '$450.00, the fee you accepted on Oct 27, 2026' },
			{ label: 'Terms', value: 'Backup for the birth.' }
		]);
	});

	it('draws no Terms row for an Offer that had none', () => {
		expect(yourFeeRows({ amountCents: 45_000, acceptedOn: '2026-10-27' })).toHaveLength(1);
	});
});
