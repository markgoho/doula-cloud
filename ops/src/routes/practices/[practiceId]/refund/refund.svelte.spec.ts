/* eslint-disable unicorn/no-null -- doula-ops answers a null quote when nothing is refundable */
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { page } from 'vitest/browser';
import { SERVICE_PROBLEM } from '#lib/session.js';
import Page from './+page.svelte';

/*
 * The refund's screens, each carrying what it draws. Before the Screen
 * union, Continue set `step = 'check'` and a separate `quote`, and the
 * check drew only when both were there: a re-read that answered with no
 * quote left `step` at 'check' and drew the answers form again, saying
 * nothing about why.
 */
const appPage = vi.hoisted(() => ({
	params: { practiceId: 'p-1' },
	data: { operator: { name: 'Mark Goho', email: 'mark@doula.cloud' } }
}));
vi.mock('$app/state', () => ({ page: appPage }));
vi.mock('$app/paths', () => ({ resolve: (route: string) => route }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/firebase.js', () => ({ freshIdToken: vi.fn(async () => 'fresh-token') }));

const owners = [{ staffId: 'owner-1', name: 'Renata Alvarez' }];
const quote = { credits: 2, amountCents: 4274, taxCents: 274, maxCredits: 5 };

const fetcher = vi.fn<typeof fetch>();

beforeEach(() => {
	fetcher.mockReset();
	vi.stubGlobal('fetch', fetcher);
});

async function answer() {
	await page.getByRole('radio', { name: 'Renata Alvarez' }).click();
	await page.getByLabelText('Month').fill('10');
	await page.getByLabelText('Day').fill('30');
	await page.getByLabelText('Year').fill('2026');
	await page.getByRole('button', { name: 'Continue' }).click();
}

describe('refunding Credits', () => {
	it('says nothing is refundable when the quote is gone by Continue', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote }))
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote: null }));
		await render(Page);
		await expect.element(page.getByLabelText('Number of Credits')).toHaveValue('5');

		await answer();

		await expect.element(page.getByText('This Practice holds no purchased Credits that can be refunded.', { exact: false })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Continue' })).not.toBeInTheDocument();
	});

	it('checks the quote before the refund', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote }))
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote: { ...quote, credits: 5 } }));
		await render(Page);

		await answer();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Check the refund before you make it' })).toBeVisible();
		await expect.element(page.getByRole('button', { name: 'Refund $42.74' })).toBeVisible();
	});

	it('draws its heading and the problem when the read throws', async () => {
		fetcher.mockRejectedValueOnce(new TypeError('Failed to fetch'));

		await render(Page);

		await expect.element(page.getByRole('heading', { level: 1, name: 'Refund Credits' })).toBeVisible();
		await expect.element(page.getByText(SERVICE_PROBLEM)).toBeVisible();
	});

	it('says the service has a problem when the refund throws', async () => {
		fetcher
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote }))
			.mockResolvedValueOnce(Response.json({ practiceName: 'Lakeside', owners, quote }))
			.mockResolvedValueOnce(new Response(undefined, { status: 204 }))
			.mockRejectedValueOnce(new TypeError('Failed to fetch'));
		await render(Page);
		await answer();
		await page.getByLabelText('Password').fill('pw');
		await page.getByLabelText('Authenticator app code').fill('123456');

		await page.getByRole('button', { name: 'Refund $42.74' }).click();

		await expect.element(page.getByText(SERVICE_PROBLEM)).toBeVisible();
	});
});
