import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// Same reason the Staff roster's own spec gives: DataTable's frame needs
// stack-l's display:block default to be a container-query context, and
// ListPage's <center-l max="none"> only lifts the default var(--measure)
// cap through the custom element's own attribute handling.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, suppressions } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * Both the `page` this route reads and the rows it lists come from the
 * fixture beside it (#596), so the screen this spec asserts on and the
 * screen the continuum sweep measures are one description.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
// Only apiFetchWithSession: this route reads refusals through
// #lib/emailSuppression.js, which imports the real apiErrorMessage
// directly rather than through api.js, so the BFF's own sentence is what
// these tests see.
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const [bounced, complained] = suppressions;

interface MockOptions {
	rows?: typeof suppressions;
	listResponse?: Response;
	clearResponse?: Response;
}

/*
 * The API double is stateful: a successful clear takes its row out of
 * what the next list read answers, so a test can assert on the screen a
 * person ends up looking at rather than on the request that got her
 * there.
 */
function mockApi({ rows = suppressions, listResponse, clearResponse }: MockOptions = {}) {
	const state = structuredClone(rows);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.endsWith('/clear')) {
			if (clearResponse) return Promise.resolve(clearResponse);
			const { address } = JSON.parse(String(init?.body));
			const index = state.findIndex((row) => row.address === address);
			state.splice(index, 1);
			return Promise.resolve(jsonResponse(undefined, 204));
		}
		return Promise.resolve(listResponse ?? jsonResponse({ suppressions: state }));
	});
}

async function setup(options: MockOptions = {}) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 46rem. Every assertion here is about the <table>, and the stacked
	// copy of a row's actions is hidden rather than absent, so the wider
	// viewport is what keeps one accessible name per control.
	await testPage.viewport(1024, 800);
	mockApi(options);
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

// The trigger button says only "Unblock" (#515): the address it acts on
// is a hidden sibling joined by aria-describedby, so no accessible query
// names it directly.
function describedByText(button: ReturnType<typeof testPage.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	// An address holds '@' and '.', so the id is escaped rather than
	// interpolated raw -- an attribute selector alone would still leave
	// the quote character unhandled.
	return document.querySelector(`#${CSS.escape(describedBy)}`)?.textContent?.trim() ?? '';
}

describe('the blocked email addresses screen', () => {
	it('lists every blocked address with why the mail stopped and when', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: bounced!.address, exact: true }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'The email could not be delivered' }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: 'The recipient marked the email as spam' }))
			.toBeVisible();
		// ADR-0022: the exact instant stays underneath the rendered day.
		await expect
			.element(testPage.getByText('Mar 14, 2027').first())
			.toHaveAttribute('datetime', bounced!.createdAt);
	});

	// ADR-0029, the whole point of the screen: a bounce is an address, a
	// complaint is a person's own answer, and only one of them is lifted.
	it('offers an Unblock only on the bounce, and says the complaint stays', async () => {
		await setup();

		const unblock = testPage.getByRole('button', { name: 'Unblock', exact: true });
		await expect.element(unblock).toBeVisible();
		expect(describedByText(unblock)).toBe(bounced!.address);
		await expect
			.element(testPage.getByText('This block stays. It cannot be undone.').first())
			.toBeVisible();
	});

	it('names the address, and what happens next, before anything is lifted', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Unblock', exact: true }).click();

		await expect
			.element(
				testPage
					.getByRole('dialog', { name: 'Unblock this address' })
					.getByText(`Doula Cloud writes to ${bounced!.address} again.`, { exact: false })
			)
			.toBeVisible();
	});

	it('takes an unblocked address off the list and says it can receive email again', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Unblock', exact: true }).click();
		await testPage
			.getByRole('dialog', { name: 'Unblock this address' })
			.getByRole('button', { name: 'Unblock this address' })
			.click();

		await expect
			.element(testPage.getByText(`${bounced!.address} can receive email again.`))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: bounced!.address, exact: true }))
			.not.toBeInTheDocument();
		// The complaint is untouched -- it was never this action's row.
		await expect
			.element(testPage.getByRole('cell', { name: complained!.address, exact: true }))
			.toBeVisible();
	});

	/*
	 * A 502 says nothing was changed, and that clause is what decides
	 * whether trying again is safe. It reaches the row verbatim.
	 */
	it("shows the provider failure's own words on the row that earned them", async () => {
		await setup({
			clearResponse: jsonResponse(
				{ message: 'could not reach the email provider; nothing was changed' },
				502
			)
		});

		await testPage.getByRole('button', { name: 'Unblock', exact: true }).click();
		await testPage
			.getByRole('dialog', { name: 'Unblock this address' })
			.getByRole('button', { name: 'Unblock this address' })
			.click();

		await expect
			.element(
				testPage.getByText('could not reach the email provider; nothing was changed').first()
			)
			.toBeVisible();
	});

	// #744's reassuring case, and the one most Practices are in.
	it('says plainly that nothing is blocked when nothing is', async () => {
		await setup({ rows: [] });

		await expect
			.element(
				testPage.getByText(
					'No blocked addresses. Every address this Practice writes to can still receive its email.'
				)
			)
			.toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	// Owner or Admin only, server-side. Somebody who typed the URL meets
	// the endpoint's own sentence rather than an empty table.
	it("reports the BFF's refusal instead of an empty list", async () => {
		await setup({
			listResponse: jsonResponse({ message: 'forbidden: owner or admin only' }, 403)
		});

		await expect.element(testPage.getByText('forbidden: owner or admin only')).toBeVisible();
	});

	// ADR-0024's conformance commitment, checked on the real route.
	it('never scrolls the document sideways at 320px', async () => {
		await setup();
		await testPage.viewport(320, 800);

		await expect
			.element(testPage.getByRole('heading', { name: 'Blocked email addresses' }))
			.toBeVisible();
		expect(document.documentElement.scrollWidth).toBe(document.documentElement.clientWidth);
	});
});
