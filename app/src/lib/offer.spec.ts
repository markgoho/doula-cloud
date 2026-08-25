import { describe, expect, it, vi } from 'vitest';
import {
	createOffer,
	decideOffer,
	declinePreAccountOffer,
	formatFee,
	isOpen,
	loadEngagementOffers,
	loadInbox,
	loadPreAccountOffer,
	offerStateLabels,
	withdrawOffer,
	type Offer
} from './offer.js';

function jsonResponse(body: unknown, status = 200): Response {
	return {
		ok: status >= 200 && status < 300,
		status,
		text: () => Promise.resolve(typeof body === 'string' ? body : JSON.stringify(body)),
		json: () => Promise.resolve(body)
	} as Response;
}

const offer: Offer = {
	offerId: 'offer-1',
	state: 'offered',
	clientFirstInitial: 'R',
	clientArea: 'North side',
	dueDate: '2027-01-04',
	amountCents: 45_000,
	terms: 'Two prenatal visits.',
	employmentType: 'contractor',
	offeredAt: '2026-08-01T00:00:00Z',
	expiresAt: '2026-08-08T00:00:00Z'
};

describe('loadEngagementOffers', () => {
	it('fetches the engagement offers path and returns the decoded list', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offers: [offer] }));

		const result = await loadEngagementOffers(fetcher, 'practice-1', 'eng-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/offers');
		expect(result).toEqual([offer]);
	});

	it('throws with the response body text when the caller may not read who was asked', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('not permitted to read this', 403));

		await expect(loadEngagementOffers(fetcher, 'practice-1', 'eng-1')).rejects.toThrow('not permitted to read this');
	});
});

describe('loadInbox', () => {
	it('fetches the practice offers path and returns her own offers', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offers: [offer] }));

		const result = await loadInbox(fetcher, 'practice-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/offers');
		expect(result).toEqual([offer]);
	});
});

describe('createOffer', () => {
	it('posts the offer and returns the created id', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse({ offerId: 'offer-1', expiresAt: '2026-08-08T00:00:00Z' }));

		const result = await createOffer(fetcher, 'practice-1', 'eng-1', {
			staffId: 'staff-1',
			amountCents: 45_000,
			clientFirstInitial: 'R',
			clientArea: 'North side',
			dueDate: '2027-01-04'
		});

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/engagements/eng-1/offers', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				staffId: 'staff-1',
				amountCents: 45_000,
				clientFirstInitial: 'R',
				clientArea: 'North side',
				dueDate: '2027-01-04'
			})
		});
		expect(result.offerId).toBe('offer-1');
	});

	it('throws with the refusal text when the fee rule is not met', async () => {
		const fetcher = vi
			.fn()
			.mockResolvedValue(jsonResponse('a fee is required when offering work to a contractor', 400));

		await expect(
			createOffer(fetcher, 'practice-1', 'eng-1', {
				staffId: 'staff-1',
				clientFirstInitial: 'R',
				clientArea: 'North side',
				dueDate: '2027-01-04'
			})
		).rejects.toThrow('a fee is required when offering work to a contractor');
	});
});

describe('decideOffer', () => {
	it('posts to the accept path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offerId: 'offer-1', state: 'accepted' }));

		const result = await decideOffer(fetcher, 'practice-1', 'offer-1', 'accept');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/offers/offer-1/accept', { method: 'POST' });
		expect(result.state).toBe('accepted');
	});

	it('posts to the decline path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offerId: 'offer-1', state: 'declined' }));

		await decideOffer(fetcher, 'practice-1', 'offer-1', 'decline');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/offers/offer-1/decline', { method: 'POST' });
	});

	it('throws when someone else has already taken the work', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('that offer is no longer open -- it is superseded', 409));

		await expect(decideOffer(fetcher, 'practice-1', 'offer-1', 'accept')).rejects.toThrow('superseded');
	});
});

describe('withdrawOffer', () => {
	it('posts to the withdraw path', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offerId: 'offer-1', state: 'withdrawn' }));

		const result = await withdrawOffer(fetcher, 'practice-1', 'offer-1');

		expect(fetcher).toHaveBeenCalledWith('/api/practices/practice-1/offers/offer-1/withdraw', { method: 'POST' });
		expect(result.state).toBe('withdrawn');
	});
});

describe('loadPreAccountOffer', () => {
	it('sends both credentials in the query string', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ ...offer, offerId: 'offer-1' }));

		await loadPreAccountOffer(fetcher, 'offer-1', 'tok en', '123456');

		expect(fetcher).toHaveBeenCalledWith('/api/offers/offer-1?token=tok+en&code=123456');
	});

	it('throws when the code is wrong', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse('that code is not right', 403));

		await expect(loadPreAccountOffer(fetcher, 'offer-1', 'token', '000000')).rejects.toThrow(
			'that code is not right'
		);
	});
});

describe('declinePreAccountOffer', () => {
	it('posts both credentials in the body', async () => {
		const fetcher = vi.fn().mockResolvedValue(jsonResponse({ offerId: 'offer-1', state: 'declined' }));

		const result = await declinePreAccountOffer(fetcher, 'offer-1', 'token', '123456');

		expect(fetcher).toHaveBeenCalledWith('/api/offers/offer-1/decline', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ token: 'token', code: '123456' })
		});
		expect(result.state).toBe('declined');
	});
});

describe('formatFee', () => {
	it('formats cents as US dollars', () => {
		expect(formatFee(45_000)).toBe('$450.00');
	});

	it('says so when an Offer carries no fee at all', () => {
		expect(formatFee(undefined)).toBe('No per-Engagement fee');
	});
});

describe('isOpen', () => {
	it('is true only while the Offer is still awaiting a decision', () => {
		expect(isOpen({ state: 'offered' })).toBe(true);
		expect(isOpen({ state: 'accepted' })).toBe(false);
		expect(isOpen({ state: 'expired' })).toBe(false);
	});
});

describe('offerStateLabels', () => {
	it('names every state the BFF can return', () => {
		expect(Object.keys(offerStateLabels)).toEqual([
			'offered',
			'accepted',
			'declined',
			'withdrawn',
			'superseded',
			'expired'
		]);
	});
});
