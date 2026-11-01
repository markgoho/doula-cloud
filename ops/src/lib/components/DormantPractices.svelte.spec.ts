/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import DormantPractices from './DormantPractices.svelte';
import type { DormantPractice } from '../dormantPractices.ts';

function row(id: string, name: string): DormantPractice {
	return { id, name, signedUpAt: '2023-05-01T14:00:00Z', balance: 4, lastContactAt: null };
}

function answering(...bodies: { status: number; body: unknown }[]) {
	const fetcher = vi.fn<typeof fetch>();
	for (const { status, body } of bodies) {
		fetcher.mockResolvedValueOnce(Response.json(body, { status }));
	}
	return fetcher;
}

describe('DormantPractices', () => {
	it('shows each Practice with its balance and last contact', async () => {
		const fetcher = answering({ status: 200, body: { items: [row('p-1', 'Quiet Hills Doulas')], hasMore: false } });

		await render(DormantPractices, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('heading', { name: 'Quiet Hills Doulas' })).toBeVisible();
		await expect.element(page.getByText('4 Credits')).toBeVisible();
		await expect.element(page.getByText('Never')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Show more Practices' })).not.toBeInTheDocument();
		expect(fetcher).toHaveBeenCalledWith('/api/practices/dormant', { credentials: 'include' });
	});

	it('shows more Practices from the next page', async () => {
		const fetcher = answering(
			{ status: 200, body: { items: [row('p-1', 'First')], hasMore: true, nextCursor: 'c-2' } },
			{ status: 200, body: { items: [row('p-2', 'Second')], hasMore: false } }
		);
		await render(DormantPractices, { fetcher, onsignedout: vi.fn() });

		await page.getByRole('button', { name: 'Show more Practices' }).click();

		await expect.element(page.getByRole('heading', { name: 'Second' })).toBeVisible();
		await expect.element(page.getByRole('heading', { name: 'First' })).toBeVisible();
		expect(fetcher).toHaveBeenLastCalledWith('/api/practices/dormant?cursor=c-2', { credentials: 'include' });
		await expect.element(page.getByRole('button', { name: 'Show more Practices' })).not.toBeInTheDocument();
	});

	it('says when no Practice is dormant', async () => {
		await render(DormantPractices, {
			fetcher: answering({ status: 200, body: { items: [], hasMore: false } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('No Practice holds Credits that nobody has used in two years.')).toBeVisible();
	});

	it('hands an ended session back', async () => {
		const onsignedout = vi.fn();

		await render(DormantPractices, { fetcher: answering({ status: 401, body: { message: 'ended' } }), onsignedout });

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
	});

	it('shows a refusal', async () => {
		await render(DormantPractices, {
			fetcher: answering({ status: 500, body: { message: 'broken' } }),
			onsignedout: vi.fn()
		});

		await expect.element(page.getByText('Sorry, there is a problem with the service. Try again later.')).toBeVisible();
	});
});
