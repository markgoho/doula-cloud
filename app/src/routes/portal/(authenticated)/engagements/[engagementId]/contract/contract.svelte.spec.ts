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
