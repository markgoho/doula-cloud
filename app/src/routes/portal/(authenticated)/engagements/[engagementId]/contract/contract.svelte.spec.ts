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

async function setup(respond: (path: string) => Promise<Response> | Response = toApiResponder(fixture)) {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(respond);
	await render(Page);
}

describe('Client-portal Contract status (#212, NH-G5)', () => {
	it('shows the register label for a sent Contract, not the raw enum', async () => {
		await setup();

		await expect.element(page.getByText('Ready for your signature')).toBeVisible();
		expect(page.getByText('sent', { exact: true }).elements()).toHaveLength(0);
	});

	it('shows the register label and a Client-worded terminal notice for a voided Contract', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided' }));

		await expect.element(page.getByText('No longer active')).toBeVisible();
		await expect.element(page.getByText('Riverside Doula Collective ended this Contract.')).toBeVisible();
		// NH-G5: never the Staff ContractStatus component's own wording.
		expect(page.getByText(/Voided —/).elements()).toHaveLength(0);
	});

	it('offers no signature step once voided', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided' }));

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
		await setup();

		await expect.element(page.getByText('Ready for your signature')).toBeVisible();
		expect(page.getByRole('button', { name: 'Download signed Contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('offers a download of the signed Contract once signed, reachable by keyboard and naming the PDF', async () => {
		await setup((path) =>
			path.endsWith('/pdf')
				? new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 })
				: jsonResponse({ ...contract, status: 'signed', hasSignedPdf: true })
		);
		const download = page.getByRole('button', { name: 'Download signed Contract (PDF)' });
		await expect.element(download).toBeVisible();

		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith('/api/portal/engagements/engagement-1/contract/pdf');
	});

	// #1119: the control used to be gated on `status === 'signed'`, so the
	// moment her Practice voided the Contract she had signed, she met no
	// way to her own copy -- while the endpoint went on serving it (#299).
	// The gate is the PDF's existence now, the same fact the endpoint
	// keys on, so a void cannot take her copy away.
	it('offers the download on a voided Contract she signed', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided', hasSignedPdf: true }));

		await expect.element(page.getByText('No longer active')).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Download signed Contract (PDF)' }))
			.toBeVisible();
	});

	it('offers no download on a voided Contract that was never signed', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided', hasSignedPdf: false }));

		await expect.element(page.getByText('No longer active')).toBeVisible();
		expect(page.getByRole('button', { name: 'Download signed Contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('reports a failed PDF fetch in words rather than swallowing it (#305 is what fails this locally/in CI)', async () => {
		await setup((path) =>
			path.endsWith('/pdf')
				? new Response('signed PDF not found', { status: 500 })
				: jsonResponse({ ...contract, status: 'signed', hasSignedPdf: true })
		);
		await page.getByRole('button', { name: 'Download signed Contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});
});
