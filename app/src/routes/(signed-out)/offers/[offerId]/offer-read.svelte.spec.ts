import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { fixture } from './page.fixture.js';

/*
 * The pre-account Offer read's own refusal path (#1107).
 *
 * This screen was the last signed-out form in the app relying on the
 * browser's own bubble to stop an empty access code. What is asserted
 * here is the half a static gate cannot see: that the page refuses the
 * submit itself, before anything reaches the network, and reports the
 * refusal twice from one string -- once in the summary at the top, once
 * beside the control.
 *
 * `page.params`/`page.url` are read through `#lib/appState.svelte.js`,
 * which reads `$app/state` -- mocked at that source so the shim itself
 * runs, the same arrangement `confirm-sign-in-address`'s spec uses. Each
 * case starts from the fixture's own URL, so this spec and the continuum
 * sweep describe one screen.
 */
const realPage = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: realPage }));
vi.mock('$app/paths', () => ({ resolve: (route: string) => route }));

const apiFetch = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetch }));

const offerBody = {
	offerId: 'offer-1',
	state: 'offered',
	clientFirstInitial: 'R',
	clientArea: 'Brighton',
	dueDate: '2027-03-14',
	amountCents: 90_000,
	employmentType: 'contractor',
	offeredAt: '2026-09-01T00:00:00Z',
	expiresAt: '2026-09-30T00:00:00Z'
};

function jsonOK(body: unknown): Response {
	return { ok: true, status: 200, json: () => Promise.resolve(body) } as Response;
}

function refusal(message: string, status = 400): Response {
	return {
		ok: false,
		status,
		text: () => Promise.resolve(JSON.stringify({ message })),
		json: () => Promise.resolve({ message })
	} as Response;
}

interface SetupOptions {
	url?: string;
	/** What each call answers, in order; the last one repeats. */
	responses?: Response[];
}

async function setup({ url = fixture.url, responses = [jsonOK(offerBody)] }: SetupOptions = {}) {
	realPage.url = new URL(url);
	realPage.params = { offerId: 'offer-1' };
	const calls: string[] = [];
	let index = 0;
	apiFetch.mockImplementation((path: string) => {
		calls.push(path);
		return Promise.resolve(responses[Math.min(index++, responses.length - 1)]);
	});
	await render(Page);
	return { calls };
}

const openButton = () => testPage.getByRole('button', { name: 'Open offer' });
const summaryHeading = () => testPage.getByRole('heading', { name: 'There is a problem' });

beforeEach(() => {
	apiFetch.mockReset();
});

describe('the pre-account Offer read refuses its own submit', () => {
	it('shows no summary before anything has been submitted', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Access code')).toBeVisible();
		expect(summaryHeading().elements()).toHaveLength(0);
	});

	it('refuses an empty code in our words, without calling the endpoint', async () => {
		const { calls } = await setup();

		await openButton().click();

		await expect.element(summaryHeading()).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Enter the six-digit code from your email' }))
			.toBeVisible();
		expect(calls).toHaveLength(0);
	});

	it('links the summary entry at the control it is about', async () => {
		await setup();

		await openButton().click();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter the six-digit code from your email' }))
			.toHaveAttribute('href', '#offer-access-code');
		await expect
			.element(testPage.getByLabelText('Access code'))
			.toHaveAttribute('id', 'offer-access-code');
	});

	it('reports the same refusal beside the control, not only once at the top', async () => {
		await setup();

		await openButton().click();

		// Two renderings of one string: the summary's link and the field's
		// own message. `getByText` would find both, so the field's is read
		// off the control's aria-describedby instead.
		await expect
			.element(testPage.getByLabelText('Access code'))
			.toHaveAttribute('aria-describedby', 'offer-access-code-error');
		expect(document.querySelector('#offer-access-code-error')?.textContent).toBe(
			'Enter the six-digit code from your email'
		);
	});

	it('refuses a code that is not six digits, and says what a code looks like', async () => {
		const { calls } = await setup();

		await testPage.getByLabelText('Access code').fill('12ab');
		await openButton().click();

		await expect
			.element(
				testPage.getByRole('link', { name: 'The code from your email is six digits, like 123456' })
			)
			.toBeVisible();
		expect(calls).toHaveLength(0);
	});

	it('opens the Offer when the code is six digits', async () => {
		const { calls } = await setup();

		await testPage.getByLabelText('Access code').fill('123456');
		await openButton().click();

		await expect.element(testPage.getByText('Brighton')).toBeVisible();
		expect(calls).toEqual(['/api/offers/offer-1?token=offer-token-1&code=123456']);
	});

	/*
	 * A wrong code and an expired token come back as one sentence the BFF
	 * wrote, and the screen does not decide which of the two it thinks
	 * happened -- so the entry is untargeted, the same reasoning
	 * `recovery-code` writes down for its own refusal.
	 */
	it("shows the endpoint's own refusal in the summary, pointed at no field", async () => {
		await setup({ responses: [refusal('this offer has expired')] });

		await testPage.getByLabelText('Access code').fill('123456');
		await openButton().click();

		await expect.element(summaryHeading()).toBeVisible();
		await expect.element(testPage.getByText('this offer has expired')).toBeVisible();
		expect(testPage.getByRole('link', { name: 'this offer has expired' }).elements()).toHaveLength(
			0
		);
	});

	it('still refuses a link with no token at all, before asking for a code', async () => {
		await setup({ url: 'https://example.test/(signed-out)/offers/offer-1' });

		await expect
			.element(
				testPage.getByText(
					'This link is missing its token. Open the offer from the email you were sent.'
				)
			)
			.toBeVisible();
		expect(openButton().elements()).toHaveLength(0);
	});
});
