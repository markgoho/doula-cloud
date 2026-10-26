/*
 * #2030: Rates is a form. Enter saves, a refused value is said in the
 * error summary and beside its box, and a save says what was saved,
 * read back from the server, "no rate" included. #2080: an emptied box
 * over a stored rate removes it.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Rate } from '#lib/rates.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, rates } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

interface SetupOptions {
	isLoadRefused?: boolean;
	isPutRefused?: boolean;
}

/** A rate card the PUTs and DELETEs write into, so the read after a save
 * answers with what was stored, as the BFF does. */
async function setup({ isLoadRefused = false, isPutRefused = false }: SetupOptions = {}) {
	const stored: Rate[] = rates.rates.map((rate) => ({ ...rate }));
	const puts: { kind: string; amountCents: number }[] = [];
	const removals: string[] = [];
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'DELETE') {
			const kind = path.split('/').at(-1)!;
			removals.push(kind);
			// eslint-disable-next-line unicorn/no-null -- the BFF's JSON null for a kind with no rate
			stored.find((rate) => rate.kind === kind)!.amountCents = null;
			// eslint-disable-next-line unicorn/no-null -- the BFF's JSON null for a kind with no rate
			return Promise.resolve(jsonResponse({ kind, amountCents: null }));
		}
		if (init?.method === 'PUT') {
			if (isPutRefused) return Promise.resolve(jsonResponse({ message: 'amountCents must be a positive integer' }, 400));
			const kind = path.split('/').at(-1)!;
			const { amountCents } = JSON.parse(String(init.body)) as { amountCents: number };
			puts.push({ kind, amountCents });
			stored.find((rate) => rate.kind === kind)!.amountCents = amountCents;
			return Promise.resolve(jsonResponse({ kind, amountCents }));
		}
		if (isLoadRefused) return Promise.resolve(jsonResponse({ message: 'could not load rates' }, 500));
		return Promise.resolve(jsonResponse({ rates: stored }));
	});
	await render(Page, {});
	return { puts, removals };
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('Rates settings screen (#2030)', () => {
	it('says what each rate is for, in dollars, and that postpartum may stay empty', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Birth rate')).toHaveValue(2500);
		await expect.element(testPage.getByText(/A new birth Contract is drafted at this amount/)).toBeVisible();
		await expect.element(testPage.getByText(/Leave it empty if you do not offer postpartum care/)).toBeVisible();
	});

	it('saves on Enter, and says what was saved, "no rate" included', async () => {
		const { puts } = await setup();

		await testPage.getByLabelText('Birth rate').fill('1800');
		await testPage.getByLabelText('Birth rate').element().closest('form')!.requestSubmit();

		await expect
			.element(testPage.getByRole('status').filter({ hasText: 'Rates saved. Birth: $1,800.00. Postpartum: no rate.' }))
			.toBeVisible();
		expect(puts).toEqual([{ kind: 'birth', amountCents: 180_000 }]);
	});

	it('refuses a value that is not a rate in the summary and beside its box, and sends nothing', async () => {
		const { puts } = await setup();

		await testPage.getByLabelText('Postpartum rate').fill('0');
		await testPage.getByRole('button', { name: 'Save' }).click();

		const message = 'Enter a postpartum rate greater than $0, or leave it empty';
		await expect.element(testPage.getByRole('link', { name: message })).toHaveAttribute('href', '#rate-postpartum');
		await expect.element(testPage.getByRole('link', { name: message })).toBeVisible();
		expect(puts).toEqual([]);
		expect(document.title).toMatch(/^Error: /);
	});

	it('removes a stored rate whose box was emptied, and says "no rate" for it (#2080)', async () => {
		const { puts, removals } = await setup();

		await testPage.getByLabelText('Birth rate').fill('');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect
			.element(testPage.getByRole('status').filter({ hasText: 'Rates saved. Birth: no rate. Postpartum: no rate.' }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('Birth rate')).toHaveDisplayValue('');
		expect(puts).toEqual([]);
		// Postpartum had no rate to remove, so nothing is sent for it.
		expect(removals).toEqual(['birth']);
	});

	it("reports the server's refusal in the summary", async () => {
		await setup({ isPutRefused: true });

		await testPage.getByLabelText('Birth rate').fill('1800');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('amountCents must be a positive integer')).toBeVisible();
	});

	it('says the rates could not be loaded, with no form to fill', async () => {
		await setup({ isLoadRefused: true });

		await expect.element(testPage.getByText('could not load rates')).toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
	});
});
