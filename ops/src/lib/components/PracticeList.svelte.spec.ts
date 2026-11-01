/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import '../../../../app/src/lib/styles/app.css';
import PracticeList from './PracticeList.svelte';
import type { PracticeRow } from '../practices.ts';

function row(id: string, name: string, grant: PracticeRow['foundingGrant'] = null): PracticeRow {
	return {
		id,
		name,
		signedUpAt: '2026-09-14T15:00:00Z',
		owners: [{ name: 'Renata Alvarez', email: 'renata@example.com' }],
		staffCount: 3,
		stripeConnect: { connected: false, cardPayments: 'unsupported', payouts: 'unsupported', requirementsDue: [], updatedAt: null },
		creditsHeld: 12,
		engagementsStarted: 2,
		foundingGrant: grant,
		deletion: null,
		source: null,
		openFeedback: 0
	};
}

function answering(...bodies: { status: number; body: unknown }[]) {
	const fetcher = vi.fn<typeof fetch>();
	for (const { status, body } of bodies) {
		fetcher.mockResolvedValueOnce(Response.json(body, { status }));
	}
	return fetcher;
}

describe('PracticeList', () => {
	it('shows each Practice with its fields and links to its acts', async () => {
		const fetcher = answering({ status: 200, body: { items: [row('p-1', 'Rooted Birth Collective')], hasMore: false } });

		await render(PracticeList, { list: 'live', fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('heading', { name: 'Rooted Birth Collective' })).toBeVisible();
		await expect.element(page.getByText('Renata Alvarez, renata@example.com')).toBeVisible();
		await expect.element(page.getByText('Not recorded yet')).toBeVisible();
		await expect
			.element(page.getByRole('link', { name: 'Refund Credits to Rooted Birth Collective' }))
			.toHaveAttribute('href', '/practices/p-1/refund');
		await expect
			.element(page.getByRole('link', { name: 'Give a founding grant to Rooted Birth Collective' }))
			.toHaveAttribute('href', '/practices/p-1/founding-grant');
		await expect.element(page.getByRole('button', { name: 'Show more Practices' })).not.toBeInTheDocument();
	});

	it('offers no founding grant to a Practice that holds one', async () => {
		const grant = { credits: 9, givenBy: 'Mark Goho', givenAt: '2026-09-15T10:00:00Z' };
		const fetcher = answering({ status: 200, body: { items: [row('p-1', 'Rooted', grant)], hasMore: false } });

		await render(PracticeList, { list: 'live', fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: /Refund Credits/ })).toBeVisible();
		await expect.element(page.getByRole('link', { name: /founding grant/ })).not.toBeInTheDocument();
	});

	it('offers no acts on the Deleted list', async () => {
		const fetcher = answering({ status: 200, body: { items: [row('p-1', 'Gone')], hasMore: false } });

		await render(PracticeList, { list: 'deleted', fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('heading', { name: 'Gone' })).toBeVisible();
		await expect.element(page.getByRole('link')).not.toBeInTheDocument();
		expect(fetcher).toHaveBeenCalledWith('/api/practices/deleted', { credentials: 'include' });
	});

	it('shows more Practices from the next page', async () => {
		const fetcher = answering(
			{ status: 200, body: { items: [row('p-1', 'First')], hasMore: true, nextCursor: 'c-2' } },
			{ status: 200, body: { items: [row('p-2', 'Second')], hasMore: false } }
		);
		await render(PracticeList, { list: 'live', fetcher, onsignedout: vi.fn() });

		await page.getByRole('button', { name: 'Show more Practices' }).click();

		await expect.element(page.getByRole('heading', { name: 'Second' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'First' })).toBeVisible();
		expect(fetcher).toHaveBeenLastCalledWith('/api/practices?cursor=c-2', { credentials: 'include' });
		await expect.element(page.getByRole('button', { name: 'Show more Practices' })).not.toBeInTheDocument();
	});

	it('says when a list holds no Practice', async () => {
		await render(PracticeList, {
			list: 'deleted',
			fetcher: answering({ status: 200, body: { items: [], hasMore: false } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('No Practice has been deleted.')).toBeVisible();
	});

	it('says no Practice has signed up on an empty live list', async () => {
		await render(PracticeList, {
			list: 'live',
			fetcher: answering({ status: 200, body: { items: [], hasMore: false } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('No Practice has signed up yet.')).toBeVisible();
	});

	it('hands an ended session back', async () => {
		const onsignedout = vi.fn();

		await render(PracticeList, { list: 'live', fetcher: answering({ status: 401, body: { message: 'ended' } }), onsignedout });

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
	});

	it('shows a refusal', async () => {
		await render(PracticeList, {
			list: 'live',
			fetcher: answering({ status: 500, body: { message: 'broken' } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('Sorry, there is a problem with the service. Try again later.')).toBeVisible();
	});
});
