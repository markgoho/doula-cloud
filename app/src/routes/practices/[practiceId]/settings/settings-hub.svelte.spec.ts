import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { zoneLabel } from '#lib/timezones.js';
import Page from './+page.svelte';
import { practiceSession, toApiResponder, toPageState } from '../../../routeFixture.js';
import { asDoula, fixture, ownerWithoutSecondFactor, pageUrl, respond } from './page.fixture.js';

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description; so are the reads it answers (`respond`).
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession, apiBaseURL: () => '' }));

async function setup(variant: Partial<typeof fixture> = {}, answer = toApiResponder(fixture)) {
	Object.assign(pageState, toPageState({ ...fixture, ...variant }));
	apiFetchWithSession.mockImplementation(answer);
	await render(Page, {});
}

/*
 * A row's action link. Every action reads "Change", told apart only by
 * the visually-hidden words it is described by -- an accessible
 * description, which no role query filters on, so this reads the
 * `aria-describedby` target the way a screen reader does.
 */
function actionFor(changes: string): HTMLAnchorElement {
	const link = [...document.querySelectorAll<HTMLAnchorElement>('dd a[aria-describedby]')].find(
		(candidate) => document.querySelector(`#${CSS.escape(candidate.getAttribute('aria-describedby')!)}`)?.textContent === changes
	);
	if (!link) throw new Error(`no row action is described by "${changes}"`);
	return link;
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('the Settings index', () => {
	it('shows an Owner four groups, each setting with its value, and deletion set apart', async () => {
		await setup();

		for (const heading of ['Your Practice', 'Clients and care', 'Money', 'Security and data', 'Delete this Practice']) {
			await expect.element(testPage.getByRole('heading', { name: heading })).toBeVisible();
		}
		await expect.element(testPage.getByRole('link', { name: pageUrl })).toHaveAttribute('href', pageUrl);
		await expect.element(testPage.getByText(zoneLabel('America/Indiana/Indianapolis'))).toBeVisible();
		// An archived field and a section heading are not questions.
		await expect.element(testPage.getByText('2 questions', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Care Plan: 3 questions. Birth Plan: 1 question.')).toBeVisible();
		await expect.element(testPage.getByText('Written', { exact: true })).toBeVisible();
		await expect
			.element(testPage.getByText('From the day a doula takes the birth, until 14 days past the due date'))
			.toBeVisible();
		await expect.element(testPage.getByText('Taking payments, payouts on hold')).toBeVisible();
		await expect.element(testPage.getByText('Birth: $12,345.00. Postpartum: $9,876.00')).toBeVisible();
		await expect.element(testPage.getByText('128 Credits')).toBeVisible();
		await expect.element(testPage.getByText('On for every Staff member')).toBeVisible();
		await expect.element(testPage.getByText('2 addresses')).toBeVisible();

		expect(actionFor('the Practice name').getAttribute('href')).toBe('/practices/practice-1/settings/name');
		expect(actionFor('your website').getAttribute('href')).toBe('/practices/practice-1/settings/website');
		expect(actionFor('multi-factor authentication').getAttribute('href')).toBe(
			'/practices/practice-1/settings/mfa'
		);
		await expect.element(testPage.getByRole('link', { name: 'Buy' })).toHaveAttribute('href', '/practices/practice-1/billing');
		await expect
			.element(testPage.getByRole('link', { name: 'Delete this Practice' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/delete');
	});

	// #288: a download the browser fetches from the BFF, not a page.
	it('gives an Owner the export as a download from the BFF', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Download' }))
			.toHaveAttribute('href', '/api/practices/practice-1/export');
	});

	it('shows a Doula only what she may change: the Plan Templates', async () => {
		await setup(asDoula);

		await expect.element(testPage.getByRole('heading', { name: 'Clients and care' })).toBeVisible();
		expect(actionFor('the Plan Templates').getAttribute('href')).toBe('/practices/practice-1/settings/plan-templates');
		for (const hidden of ['Your Practice', 'Money', 'Security and data', 'Delete this Practice']) {
			await expect.element(testPage.getByRole('heading', { name: hidden })).not.toBeInTheDocument();
		}
		await expect.element(testPage.getByText('Client Fields')).not.toBeInTheDocument();
		await expect.element(testPage.getByText('Contract Template')).not.toBeInTheDocument();
		// Nothing is read that her index does not show.
		expect(apiFetchWithSession.mock.calls.map(([path]) => path as string).toSorted((a, b) => a.localeCompare(b))).toEqual([
			'/api/practices/practice-1/plan-templates/birth_plan',
			'/api/practices/practice-1/plan-templates/care_plan'
		]);
	});

	it('gives an Admin the Owner-or-Admin settings and none of the Owner-only ones', async () => {
		await setup({ pageData: practiceSession(['admin']) });

		expect(actionFor('the timezone').getAttribute('href')).toBe('/practices/practice-1/settings/timezone');
		expect(actionFor('the rates').getAttribute('href')).toBe('/practices/practice-1/settings/rates');
		expect(actionFor('the blocked email addresses').getAttribute('href')).toBe(
			'/practices/practice-1/settings/blocked-addresses'
		);
		for (const ownerOnly of ['Practice name', 'Your website', 'Contract Template', 'Multi-factor authentication']) {
			await expect.element(testPage.getByText(ownerOnly, { exact: true })).not.toBeInTheDocument();
		}
		await expect.element(testPage.getByRole('link', { name: 'Download' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('heading', { name: 'Delete this Practice' })).not.toBeInTheDocument();
	});

	// #1532: the archive needs a second factor. She is told so before she
	// tries, in place of a download link the BFF would refuse.
	it('tells an Owner with no second factor that exporting needs one, in place of the download', async () => {
		await setup(ownerWithoutSecondFactor);

		await expect.element(testPage.getByRole('link', { name: 'Download' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText("You need two-factor authentication before you can export this Practice's data."))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Set up two-factor authentication' }))
			.toHaveAttribute('href', `/mfa/enroll?returnTo=${encodeURIComponent(`${pageState.url.pathname}#security-and-data`)}`);
	});

	// #2185: enrolling sends her back to the group that holds Download,
	// not to the top of the hub with the export eleven rows down. The
	// fragment resolves through an id, which the accessible tree does not
	// carry, so the section is read off the heading's DOM.
	it('names the group that holds the export, so enrolling lands her on it', async () => {
		await setup(ownerWithoutSecondFactor);

		const group = testPage.getByRole('heading', { name: 'Security and data' }).element().closest('section');
		expect(group?.id).toBe('security-and-data');
	});

	it('says which value could not be read, and still shows the rest', async () => {
		await setup({}, (path: string) =>
			Promise.resolve(path.endsWith('/rates') ? jsonResponse({ message: 'internal error' }, 500) : respond(path))
		);

		await expect.element(testPage.getByText('Could not load this. Open it to see why.')).toBeVisible();
		await expect.element(testPage.getByText('128 Credits')).toBeVisible();
	});
});
