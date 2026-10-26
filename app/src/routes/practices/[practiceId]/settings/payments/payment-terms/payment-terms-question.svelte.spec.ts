/*
 * #768's setting on its own question page (#2046): how many days after an
 * Invoice is raised it falls due. Set by an Owner or an Admin; anyone
 * else reads it. A save returns to Getting paid with a message saying
 * what was saved.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { netDaysOutOfRangeMessage } from '#lib/invoice.js';
import { captureLanding } from '#lib/testOutcome.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../../routeFixture.js';
import { asDoula, fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const termsPath = '/api/practices/practice-1/payments/payment-terms';

/** The fixture's reads, and a PUT that stores what it is sent -- or
 * refuses it, where `putRefusal` says how. */
async function setup(subject: RouteFixture = fixture, putRefusal?: string) {
	Object.assign(pageState, toPageState(subject));
	const read = toApiResponder(subject);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			if (putRefusal) return Promise.resolve(jsonResponse({ message: putRefusal }, 400));
			const { netDays } = JSON.parse(String(init.body)) as { netDays: number };
			return Promise.resolve(jsonResponse({ netDays, isDefault: false }));
		}
		return read(path);
	});
	await render(Page, {});
}

function puts(): unknown[][] {
	return apiFetchWithSession.mock.calls.filter(
		(call: unknown[]) => call[0] === termsPath && (call[1] as RequestInit | undefined)?.method === 'PUT'
	);
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

describe('the payment terms question (#2046)', () => {
	it('asks one question, with the saved terms in the box and what a change reaches', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'How many days does a Client have to pay an invoice?' }))
			.toBeVisible();
		await expect.element(testPage.getByLabelText('How many days does a Client have to pay an invoice?')).toHaveValue(120);
		await expect.element(testPage.getByText(/keeps the terms it was billed under/)).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', '/practices/practice-1/settings/payments');
	});

	it('saves, and returns to Getting paid saying what was saved', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByLabelText('How many days does a Client have to pay an invoice?').fill('15');
		await testPage.getByRole('button', { name: 'Save payment terms' }).click();

		await expect.poll(() => landing.href).toBe('/practices/practice-1/settings/payments');
		expect(landing.message).toBe('Payment terms saved. The next invoice raised is due 15 days after it is raised.');
		const body = String((puts()[0]![1] as RequestInit).body);
		expect(JSON.parse(body)).toEqual({ netDays: 15 });
	});

	it('refuses an impossible term in the words of the field, before the request is made', async () => {
		await setup();

		await testPage.getByLabelText('How many days does a Client have to pay an invoice?').fill('0');
		await testPage.getByRole('button', { name: 'Save payment terms' }).click();

		// GOV.UK's Recover from validation errors pattern: said at the top
		// of the page as a link that reaches the box, and again above it.
		await expect
			.element(testPage.getByRole('link', { name: netDaysOutOfRangeMessage }))
			.toHaveAttribute('href', '#payment-terms-net-days');
		await expect
			.element(testPage.getByLabelText('How many days does a Client have to pay an invoice?'))
			.toHaveAccessibleDescription(new RegExp(netDaysOutOfRangeMessage));
		await expect
			.element(testPage.getByLabelText('How many days does a Client have to pay an invoice?'))
			.toHaveAttribute('aria-invalid', 'true');
		expect(puts()).toHaveLength(0);
		expect(goto).not.toHaveBeenCalled();
	});

	it("says the server's refusal, and stays on the question", async () => {
		await setup(fixture, 'Days to pay must be a whole number of days from 1 to 365');

		await testPage.getByRole('button', { name: 'Save payment terms' }).click();

		await expect
			.element(testPage.getByText('Days to pay must be a whole number of days from 1 to 365').first())
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('shows a Doula the terms, and who may change them, with no box and no Save', async () => {
		await setup({ ...fixture, ...asDoula });

		await expect.element(testPage.getByText('Due 120 days after an invoice is raised')).toBeVisible();
		await expect
			.element(testPage.getByText('Only a Practice Owner or Admin can change the payment terms.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save payment terms' })).not.toBeInTheDocument();
	});

	it('says the terms could not be loaded', async () => {
		await setup({ ...fixture, respond: () => jsonResponse({ message: 'could not load payment terms' }, 500) });

		await expect.element(testPage.getByText('could not load payment terms')).toBeVisible();
	});
});
