import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import FeedbackList from './FeedbackList.svelte';
import type { FeedbackSummary } from '../feedback.ts';

const waiting: FeedbackSummary['issue'] = { state: 'pending', attempts: 0 };

function piece(id: string, sentAt: string, issue = waiting): FeedbackSummary {
	return { id, kind: 'not_working', routeId: '/portal/[clientId]', sentAt, issue };
}

function answering(...bodies: { status: number; body: unknown }[]) {
	const fetcher = vi.fn<typeof fetch>();
	for (const { status, body } of bodies) {
		fetcher.mockResolvedValueOnce(Response.json(body, { status }));
	}
	return fetcher;
}

describe('FeedbackList', () => {
	it('shows each piece as a link to it, with its kind, route and issue', async () => {
		const deadLettered = piece('f-1', '2026-09-20T15:30:00Z', {
			state: 'dead_lettered',
			attempts: 8,
			lastError: '401 Bad credentials',
			deadLetterReason: 'retries_exhausted'
		});
		const fetcher = answering({ status: 200, body: { items: [deadLettered], hasMore: false } });

		await render(FeedbackList, { list: 'unopened', fetcher, onsignedout: vi.fn() });

		const link = page.getByRole('link', { name: 'September 20, 2026 at 11:30 AM EDT' });
		await expect.element(link).toBeVisible();
		expect(link.element()).toHaveAttribute('href', '/feedback/f-1');
		await expect.element(page.getByText('Something is not working')).toBeVisible();
		await expect.element(page.getByText('Not opened: all 8 attempts failed, and nothing will try again')).toBeVisible();
		await expect.element(page.getByText('401 Bad credentials')).toBeVisible();
		expect(fetcher).toHaveBeenCalledWith('/api/feedback?issue=unopened', { credentials: 'include' });
	});

	it('shows more Feedback from the next page', async () => {
		const fetcher = answering(
			{ status: 200, body: { items: [piece('f-1', '2026-09-21T15:30:00Z')], hasMore: true, nextCursor: 'c-2' } },
			{ status: 200, body: { items: [piece('f-2', '2026-09-20T15:30:00Z')], hasMore: false } }
		);
		await render(FeedbackList, { list: 'all', fetcher, onsignedout: vi.fn() });

		await page.getByRole('button', { name: 'Show more Feedback' }).click();

		await expect.element(page.getByRole('link', { name: 'September 20, 2026 at 11:30 AM EDT' })).toBeVisible();
		await expect.element(page.getByRole('link', { name: 'September 21, 2026 at 11:30 AM EDT' })).toBeVisible();
		expect(fetcher).toHaveBeenLastCalledWith('/api/feedback?cursor=c-2', { credentials: 'include' });
		await expect.element(page.getByRole('button', { name: 'Show more Feedback' })).not.toBeInTheDocument();
	});

	it('says when there is no Feedback, and when no issue failed to open', async () => {
		await render(FeedbackList, {
			list: 'all',
			fetcher: answering({ status: 200, body: { items: [], hasMore: false } }),
			onsignedout: vi.fn()
		});
		await expect.element(page.getByText('No Feedback has been sent yet.')).toBeVisible();

		await render(FeedbackList, {
			list: 'unopened',
			fetcher: answering({ status: 200, body: { items: [], hasMore: false } }),
			onsignedout: vi.fn()
		});
		await expect.element(page.getByText('Every issue has opened, or is waiting to open.')).toBeVisible();
	});

	it('hands an ended session back', async () => {
		const onsignedout = vi.fn();

		await render(FeedbackList, { list: 'all', fetcher: answering({ status: 401, body: { message: 'ended' } }), onsignedout });

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
	});

	it('shows a refusal', async () => {
		await render(FeedbackList, {
			list: 'all',
			fetcher: answering({ status: 500, body: { message: 'broken' } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('Sorry, there is a problem with the service. Try again later.')).toBeVisible();
	});
});
