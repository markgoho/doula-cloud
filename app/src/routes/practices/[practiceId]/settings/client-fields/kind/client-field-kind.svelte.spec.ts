/*
 * #2193: a new Client field starts with its kind of answer, one radio
 * question with nothing chosen.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toPageState } from '../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

async function setup() {
	await render(Page, {});
}

beforeEach(() => {
	goto.mockReset();
});

describe('the kind of answer for a new Client field (#2193)', () => {
	it('asks with nothing chosen, and goes back to the list', async () => {
		await setup();

		expect(testPage.getByRole('radio').elements()).toHaveLength(6);
		expect(testPage.getByRole('radio').elements().some((radio) => (radio as HTMLInputElement).checked)).toBe(false);
		await expect
			.element(testPage.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/client-fields');
	});

	it("opens the new field's page with the kind chosen", async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Yes or no' }).click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith('/practices/practice-1/settings/client-fields/new?kind=checkbox');
	});

	it('asks again when no kind is chosen', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Select the kind of answer you need' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
