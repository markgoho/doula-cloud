/* eslint-disable unicorn/no-null -- doula-ops answers null for a field with nothing in it */
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import OperatorLog from './OperatorLog.svelte';
import type { LogRow } from '../operatorLog.ts';

function row(id: string, overrides: Partial<LogRow> = {}): LogRow {
	return { id, at: '2026-11-02T14:05:00Z', operator: 'Mark Goho', action: 'sign_in', subject: null, details: {}, ...overrides };
}

function answering(...bodies: { status: number; body: unknown }[]) {
	const fetcher = vi.fn<typeof fetch>();
	for (const { status, body } of bodies) {
		fetcher.mockResolvedValueOnce(Response.json(body, { status }));
	}
	return fetcher;
}

describe('OperatorLog', () => {
	it('shows each row with its kind, operator and New York time', async () => {
		const fetcher = answering({ status: 200, body: { items: [row('a-1')], hasMore: false } });

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('heading', { name: 'Signed in' })).toBeVisible();
		await expect.element(page.getByText('Mark Goho')).toBeVisible();
		await expect.element(page.getByText('November 2, 2026 at 9:05 AM EST')).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Show more' })).not.toBeInTheDocument();
	});

	it('names what a row acted on and says what it did', async () => {
		const fetcher = answering({
			status: 200,
			body: {
				items: [
					row('a-1', {
						action: 'founding_grant',
						subject: { kind: 'practice', id: 'p-1', name: 'Gone Births', deleted: true },
						details: { staffCount: 3, credits: 30 }
					}),
					row('a-2', {
						action: 'second_factor_cleared',
						subject: { kind: 'staff', id: 's-1', name: 'Priya Raman', deleted: false }
					})
				],
				hasMore: false
			}
		});

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByText('Gone Births (deleted)')).toBeVisible();
		await expect.element(page.getByText('30 Credits for 3 Staff')).toBeVisible();
		await expect.element(page.getByText('Priya Raman')).toBeVisible();
	});

	it('links a Feedback read to the piece it read', async () => {
		const fetcher = answering({
			status: 200,
			body: {
				items: [row('a-1', { action: 'feedback_read', subject: { kind: 'feedback', id: 'f-1', name: null, deleted: false } })],
				hasMore: false
			}
		});

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('link', { name: 'Open the piece of Feedback' })).toHaveAttribute('href', '/feedback/f-1');
	});

	it('shows more rows from the next page', async () => {
		const fetcher = answering(
			{ status: 200, body: { items: [row('a-1')], hasMore: true, nextCursor: 'c-2' } },
			{ status: 200, body: { items: [row('a-2', { action: 'sign_out' })], hasMore: false } }
		);
		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await page.getByRole('button', { name: 'Show more' }).click();

		await expect.element(page.getByRole('heading', { name: 'Signed out' })).toBeVisible();
		expect(fetcher).toHaveBeenLastCalledWith('/api/operator-log?cursor=c-2', { credentials: 'include' });
		await expect.element(page.getByRole('button', { name: 'Show more' })).not.toBeInTheDocument();
	});

	it('shows an action this build has no words for as recorded', async () => {
		const fetcher = answering({ status: 200, body: { items: [row('a-1', { action: 'newer_act' })], hasMore: false } });

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByRole('heading', { name: 'newer_act' })).toBeVisible();
	});

	it('says so when the log is empty', async () => {
		const fetcher = answering({ status: 200, body: { items: [], hasMore: false } });

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByText('Nothing has been recorded yet.')).toBeVisible();
	});

	it('shows a refusal', async () => {
		const fetcher = answering({ status: 400, body: { code: 'INVALID_ARGUMENT', message: 'This page of the list cannot be found. Start from the first page.' } });

		await render(OperatorLog, { fetcher, onsignedout: vi.fn() });

		await expect.element(page.getByText('This page of the list cannot be found. Start from the first page.')).toBeVisible();
	});

	it('hands back once the session has ended', async () => {
		const onsignedout = vi.fn();
		const fetcher = answering({ status: 401, body: { code: 'UNAUTHORIZED', message: 'Sign in.' } });

		await render(OperatorLog, { fetcher, onsignedout });

		await vi.waitFor(() => expect(onsignedout).toHaveBeenCalledOnce());
	});
});
