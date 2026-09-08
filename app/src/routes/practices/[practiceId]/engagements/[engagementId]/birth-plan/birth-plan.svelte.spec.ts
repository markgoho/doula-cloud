import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { toApiResponder, toPageState } from '../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';
import Page from './+page.svelte';
import type { PageProps as PageProperties } from './$types';

/*
 * `page.params`/`page.url` come off the route's own fixture (#596), the
 * same installation the continuum sweep uses. `data` is a component prop
 * here (this route's own `+page.ts`, not an ancestor `+layout.ts`), so
 * it is installed per test from `fixture.props.data` instead.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const fixtureData = fixture.props?.data as PageProperties['data'];

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
}));

describe('The Practice-side Birth Plan (#280)', () => {
	it('names the Client, links back to the Engagement, and shows the plan', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page, { data: fixtureData, params: fixture.params });

		await expect
			.element(page.getByRole('heading', { name: "Anne-Marie Ochieng-Whitfield's Birth Plan" }))
			.toBeVisible();
		await expect.element(page.getByRole('link', { name: 'Back to Anne-Marie Ochieng-Whitfield' })).toBeVisible();
		await expect
			.element(page.getByText('Who do you want with you, and what should we know about them?'))
			.toBeVisible();
	});

	it('offers a Print control once the plan has loaded', async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Page, { data: fixtureData, params: fixture.params });

		await expect.element(page.getByRole('button', { name: 'Print' })).toBeVisible();
	});

	// The applicable-but-not-yet-created case: a 404 from the real
	// endpoint (loadInstance's own "not created yet" branch) reads
	// plainly, never as an empty document.
	it('says plainly that no Birth Plan has been created yet, rather than rendering an empty document', async () => {
		apiFetchWithSession.mockResolvedValue({ status: 404, ok: false, text: () => Promise.resolve('none') } as Response);

		await render(Page, { data: fixtureData, params: fixture.params });

		await expect
			.element(page.getByText('No Birth Plan has been created for this Engagement yet.'))
			.toBeVisible();
		expect(page.getByRole('button', { name: 'Print' }).elements()).toHaveLength(0);
	});

	it('reports a failed load in words, announced as an alert', async () => {
		apiFetchWithSession.mockResolvedValue({
			status: 500,
			ok: false,
			text: () => Promise.resolve('plan read failed')
		} as Response);

		await render(Page, { data: fixtureData, params: fixture.params });

		await expect.element(page.getByRole('alert')).toHaveTextContent('plan read failed');
	});
});

// #306: the same PDF the Client's own portal page downloads, from the
// same rendering -- mirrors birth-plan.svelte.spec.ts's own describe
// block on the portal side, and moved here from the Engagement hub's
// spec, which no longer offers this control at all (#280).
describe('the Birth Plan PDF download (#306), on its own Practice-side page', () => {
	it('offers a download of the Birth Plan, reachable by keyboard and naming the PDF', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/pdf')
				? Promise.resolve(new Response(new Blob(['%PDF-1.4'], { type: 'application/pdf' }), { status: 200 }))
				: toApiResponder(fixture)(path)
		);

		await render(Page, { data: fixtureData, params: fixture.params });
		const download = page.getByRole('button', { name: 'Download Birth Plan (PDF)' });
		await expect.element(download).toBeVisible();

		await download.click();

		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${fixture.params.practiceId}/engagements/${fixture.params.engagementId}/plans/birth_plan/pdf`
		);
	});

	it('reports a failed PDF fetch in words rather than swallowing it', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			path.endsWith('/pdf')
				? Promise.resolve(new Response('no plan instance found for this engagement and plan type', { status: 500 }))
				: toApiResponder(fixture)(path)
		);

		await render(Page, { data: fixtureData, params: fixture.params });
		await page.getByRole('button', { name: 'Download Birth Plan (PDF)' }).click();

		await expect
			.element(page.getByRole('alert'))
			.toHaveTextContent('no plan instance found for this engagement and plan type');
	});
});
