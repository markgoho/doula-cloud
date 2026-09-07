import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { contract, fixture } from './page.fixture.js';
import Page from './+page.svelte';

/*
 * NH-G5 (#212): the portal Contract view reads `clientRegister.ts` for its
 * status wording, not the Staff `ContractStatus` component's bare enum
 * and "Voided --" copy. `toApiResponder(fixture)` answers the sent-Contract
 * happy path; the voided branch is a departure from it, spread rather than
 * a fresh Contract object that re-states the fields it shares.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

describe('Client-portal Contract status (#212, NH-G5)', () => {
	it('shows the register label for a sent Contract, not the raw enum', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page);

		await expect.element(page.getByText('Ready for your signature')).toBeVisible();
		expect(page.getByText('sent', { exact: true }).elements()).toHaveLength(0);
	});

	it('shows the register label and a Client-worded terminal notice for a voided Contract', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...contract, status: 'voided' }));

		await render(Page);

		await expect.element(page.getByText('No longer active')).toBeVisible();
		await expect.element(page.getByText('Riverside Doula Collective ended this Contract.')).toBeVisible();
		// NH-G5: never the Staff ContractStatus component's own wording.
		expect(page.getByText(/Voided —/).elements()).toHaveLength(0);
	});

	it('offers no signature step once voided', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...contract, status: 'voided' }));

		await render(Page);

		await expect.element(page.getByText('No longer active')).toBeVisible();
		expect(page.getByRole('button', { name: /sign/i }).elements()).toHaveLength(0);
	});
});

// #302: the Client is offered her copy from the same page she signed on,
// once she has signed. `respond` (a spread over `contract`, per this
// route's own convention above) answers the initial load; the pdf path is
// a second fetch this route's own handler makes only once the download
// button is clicked.
describe('Client-portal signed Contract download (#302)', () => {
	it('offers no download before the Contract has been signed', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page);

		await expect.element(page.getByText('Ready for your signature')).toBeVisible();
		expect(page.getByRole('button', { name: 'Download signed Contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('offers a download of the signed Contract once signed, reachable by keyboard and naming the PDF', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				path.endsWith('/pdf')
					? new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 })
					: jsonResponse({ ...contract, status: 'signed' })
			)
		);

		await render(Page);
		const download = page.getByRole('button', { name: 'Download signed Contract (PDF)' });
		await expect.element(download).toBeVisible();

		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/contract/pdf');
	});

	it('reports a failed PDF fetch in words rather than swallowing it (#305 is what fails this locally/in CI)', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				path.endsWith('/pdf')
					? new Response('signed PDF not found', { status: 500 })
					: jsonResponse({ ...contract, status: 'signed' })
			)
		);

		await render(Page);
		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});
});
