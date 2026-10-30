import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { contract, fixture } from './page.fixture.js';
import Page from './+page.svelte';
// The loading Skeleton reserves space with `var(--text-body-size)`, which
// only exists once the tokens are loaded -- the real app loads them in the
// root layout. See invoices.svelte.spec.ts's identical import.
import '#lib/styles/app.css';

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
	apiFetchWithSession
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
		// #2037: through Notice, as information -- nothing went wrong.
		await expect.element(page.getByRole('status')).toHaveTextContent('Riverside Doula Collective ended this contract.');
		// NH-G5: never the Staff ContractStatus component's own wording.
		expect(page.getByText(/Voided —/).elements()).toHaveLength(0);
	});

	it('offers no signature step once voided', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided' }));

		await expect.element(page.getByText('No longer active')).toBeVisible();
		expect(page.getByRole('button', { name: /sign/i }).elements()).toHaveLength(0);
	});
});

// #2037: signing is the peak of her journey, and it ends in a Notice that
// says what happens next, where the form stood -- focused, because the
// Sign button that caused it is gone.
describe('Client-portal Contract signing ends in a Notice (#2037)', () => {
	it('replaces the form with a focused status Notice and shows the signed Badge', async () => {
		await setup((path) =>
			jsonResponse(path.endsWith('/sign') ? { ...contract, status: 'signed', hasSignedPdf: true } : contract)
		);

		await page.getByRole('button', { name: 'I agree to sign electronically, continue' }).click();
		await page.getByLabelText('Full legal name').fill('Anne-Marie Ochieng-Whitfield');
		await page.getByLabelText('I have read this contract and I am signing it electronically').click();
		await page.getByRole('button', { name: 'Sign', exact: true }).click();

		const notice = page.getByRole('status');
		await expect
			.element(notice)
			.toHaveTextContent(
				'You signed this contract. Riverside Doula Collective can see it is signed, and your copy is ready to download.'
			);
		await expect.element(notice).toHaveFocus();
		await expect.element(page.getByText('Signed', { exact: true })).toBeVisible();
		expect(page.getByRole('button', { name: 'Sign', exact: true }).elements()).toHaveLength(0);
		await expect.element(page.getByRole('button', { name: 'Download signed contract (PDF)' })).toBeVisible();
	});

	it('announces nothing on a Contract that was already signed when the page opened', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'signed', hasSignedPdf: true }));

		await expect.element(page.getByText('Signed', { exact: true })).toBeVisible();
		expect(page.getByRole('status').elements()).toHaveLength(0);
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
		expect(page.getByRole('button', { name: 'Download signed contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('offers a download of the signed Contract once signed, reachable by keyboard and naming the PDF', async () => {
		await setup((path) =>
			path.endsWith('/pdf')
				? new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 })
				: jsonResponse({ ...contract, status: 'signed', hasSignedPdf: true })
		);
		const download = page.getByRole('button', { name: 'Download signed contract (PDF)' });
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
			.element(page.getByRole('button', { name: 'Download signed contract (PDF)' }))
			.toBeVisible();
	});

	it('offers no download on a voided Contract that was never signed', async () => {
		await setup(() => jsonResponse({ ...contract, status: 'voided', hasSignedPdf: false }));

		await expect.element(page.getByText('No longer active')).toBeVisible();
		expect(page.getByRole('button', { name: 'Download signed contract (PDF)' }).elements()).toHaveLength(0);
	});

	it('reports a failed PDF fetch in words rather than swallowing it (#305 is what fails this locally/in CI)', async () => {
		await setup((path) =>
			path.endsWith('/pdf')
				? new Response('signed PDF not found', { status: 500 })
				: jsonResponse({ ...contract, status: 'signed', hasSignedPdf: true })
		);
		await page.getByRole('button', { name: 'Download signed contract (PDF)' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('signed PDF not found');
	});
});

// #1574: the page sits in DocumentPage's frame in every state, and each
// state has exactly one <h1> naming the screen -- not only the loaded one.
describe('Client-portal Contract names the screen in every state (#1574)', () => {
	it('while it loads', async () => {
		await setup(() => new Promise<Response>(() => {}));

		await expect.element(page.getByRole('status', { name: 'Loading contract' })).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('when it cannot be read', async () => {
		await setup(() => new Response('the Contract could not be read', { status: 500 }));

		await expect.element(page.getByText('the Contract could not be read')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('falls back to a generic sentence when the failure is not an Error', async () => {
		await setup(() => Promise.reject('nope'));

		await expect.element(page.getByText('Failed to load contract')).toBeVisible();
	});

	it('when none has been sent yet', async () => {
		await setup(() => new Response('none', { status: 404 }));

		await expect.element(page.getByText('No contract has been sent for your care yet.')).toBeVisible();
		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	});

	it('once loaded, with the back link above the h1', async () => {
		await setup();

		await expect.element(page.getByRole('heading', { level: 1, name: 'Contract' })).toBeVisible();
		expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
		await expect
			.element(page.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/portal/engagements/engagement-1');
	});
});
