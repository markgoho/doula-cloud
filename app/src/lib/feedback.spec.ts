import { describe, expect, it, vi } from 'vitest';
import { RefusalError } from './formErrors.js';
import {
	browserName,
	feedbackSentNotice,
	kindOptions,
	sendPortalFeedback,
	sendStaffFeedback,
	type PortalFeedbackInput,
	type StaffFeedbackInput
} from './feedback.js';

const input: StaffFeedbackInput = {
	kind: 'not_working',
	text: 'The invoice list will not load.',
	page: { url: '/practices/practice-1/invoices', route: { id: '/practices/[practiceId]/invoices' } },
	appBuild: 'dev',
	screenWidth: 1280,
	practiceId: 'practice-1'
};

const portalInput: PortalFeedbackInput = {
	kind: 'idea_or_request',
	text: 'A way to download the contract as a PDF.',
	page: {
		url: '/portal/engagements/engagement-1/contract',
		route: { id: '/portal/(authenticated)/engagements/[engagementId]/contract' }
	},
	appBuild: 'dev',
	screenWidth: 1024,
	engagementId: 'engagement-1'
};

describe('kindOptions', () => {
	it('lists the three kinds #1498 Q5 decided, in the reviewed order', () => {
		expect(kindOptions).toEqual([
			{ value: 'not_working', label: 'Something is not working' },
			{ value: 'idea_or_request', label: 'An idea or a request' },
			{ value: 'something_else', label: 'Something else' }
		]);
	});
});

describe('sendStaffFeedback', () => {
	it('posts the input as JSON to /api/staff/feedback', async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response('{"id":"f-1"}', { status: 201 }));

		await sendStaffFeedback(fetcher, input);

		expect(fetcher).toHaveBeenCalledWith('/api/staff/feedback', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(input)
		});
	});

	it('throws a RefusalError carrying the refusal when the BFF refuses the send', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(new Response('{"message":"Select the kind of feedback"}', { status: 400 }));

		await expect(sendStaffFeedback(fetcher, input)).rejects.toThrow(RefusalError);
	});
});

describe('sendPortalFeedback', () => {
	it('posts the input as JSON to /api/portal/feedback', async () => {
		const fetcher = vi.fn().mockResolvedValue(new Response('{"id":"f-2"}', { status: 201 }));

		await sendPortalFeedback(fetcher, portalInput);

		expect(fetcher).toHaveBeenCalledWith('/api/portal/feedback', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(portalInput)
		});
	});

	it('throws a RefusalError carrying the refusal when the BFF refuses the send', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(new Response('{"message":"Select the kind of feedback"}', { status: 400 }));

		await expect(sendPortalFeedback(fetcher, portalInput)).rejects.toThrow(RefusalError);
	});
});

describe('feedbackSentNotice', () => {
	it('names the replier and the address when one is known', () => {
		expect(feedbackSentNotice('jordan@fingerlakesbirth.example', 'Mark Goho, who builds DoulaCloud,')).toBe(
			'Feedback sent. Thank you. If a reply would help, Mark Goho, who builds DoulaCloud, will email you at jordan@fingerlakesbirth.example.'
		);
		expect(feedbackSentNotice('alex.rivera@example.com', 'the DoulaCloud team')).toBe(
			'Feedback sent. Thank you. If a reply would help, the DoulaCloud team will email you at alex.rivera@example.com.'
		);
	});

	it('drops the "will email you at" clause rather than interpolating an empty address', () => {
		expect(feedbackSentNotice('', 'the DoulaCloud team')).toBe('Feedback sent. Thank you.');
	});
});

describe('browserName', () => {
	it.each([
		['reports Edge before Chrome, since Edge also carries "Chrome/"', 'Mozilla/5.0 Edg/128.0 Chrome/128.0', 'Edge'],
		['reports Chrome', 'Mozilla/5.0 Chrome/128.0 Safari/537.36', 'Chrome'],
		['reports Firefox', 'Mozilla/5.0 Firefox/128.0', 'Firefox'],
		['reports Safari', 'Mozilla/5.0 Version/17.0 Safari/605.1', 'Safari'],
		['reports Unknown browser for anything else', 'curl/8.0', 'Unknown browser']
	])('%s', (_name, userAgent, expected) => {
		expect(browserName(userAgent)).toBe(expected);
	});
});
