import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../routeFixture.js';
import { fixture, ownerWithoutSecondFactor } from './page.fixture.js';
// The stack's gap is a token, which exists only once the tokens load; the
// real app loads them in the root layout.
import '#lib/styles/app.css';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession, apiBaseURL: () => 'https://api.example.test' }));

async function setup(subject: RouteFixture = fixture) {
	Object.assign(pageState, toPageState(subject));
	apiFetchWithSession.mockImplementation(toApiResponder(subject));
	await render(Page, {});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('Delete-this-Practice settings screen', () => {
	it('offers an Owner with a second factor the Delete button', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Delete this Practice' })).toBeVisible();
	});

	// #2029: GOV.UK's Warning text -- the consequence, and the export that
	// can be taken first, are said before the button, not only inside the
	// confirm dialog it opens.
	it('says what deleting does, and offers the export, before the button', async () => {
		await setup();

		const button = testPage.getByRole('button', { name: 'Delete this Practice' });
		await expect.element(button).toBeVisible();
		const warning = testPage.getByText(
			"Deleting starts a 30-day countdown. After it, every Client's records are erased and cannot be recovered."
		);
		await expect.element(warning).toBeVisible();
		const exportLink = testPage.getByRole('link', { name: "Download this Practice's data first" });
		await expect.element(exportLink).toHaveAttribute('href', 'https://api.example.test/api/practices/practice-1/export');
		expect(warning.element().getBoundingClientRect().top).toBeLessThan(button.element().getBoundingClientRect().top);
		expect(exportLink.element().getBoundingClientRect().top).toBeLessThan(button.element().getBoundingClientRect().top);
	});

	// #2020: a region whose snippet renders nothing draws no blank band;
	// here the body holds the warning, so the space between the title and
	// the button is the intro, the warning and the link, each one gap apart.
	it('draws no empty region between the title and the button', async () => {
		await setup();

		const button = testPage.getByRole('button', { name: 'Delete this Practice' });
		await expect.element(button).toBeVisible();
		const title = testPage.getByRole('heading', { level: 1, name: 'Delete this Practice' }).element();
		const regions = [...title.parentElement!.children].filter((child) => getComputedStyle(child).display !== 'none');
		expect(regions.every((region) => region.getBoundingClientRect().height > 0)).toBe(true);
	});

	// #1532: starting deletion needs a second factor. She is told so before
	// she tries, with the way to set one up, and enrollment brings her back
	// here.
	it('tells an Owner with no second factor that deleting needs one, in place of the button', async () => {
		await setup({ ...fixture, ...ownerWithoutSecondFactor });

		await expect
			.element(testPage.getByText('You need two-factor authentication before you can delete this Practice.'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Set up two-factor authentication' }))
			.toHaveAttribute('href', `/mfa/enroll?returnTo=${encodeURIComponent('/practices/practice-1/settings/delete')}`);
		await expect.element(testPage.getByRole('button', { name: 'Delete this Practice' })).not.toBeInTheDocument();
		// The warning still stands; the export needs a second factor too, so
		// it is not offered as a link that would only be refused.
		await expect.element(testPage.getByText(/Deleting starts a 30-day countdown/)).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: "Download this Practice's data first" }))
			.not.toBeInTheDocument();
	});
});
