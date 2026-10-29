/*
 * #2193: "Add a question" asks the kind of answer first, one radio
 * question in #1252's human names with nothing chosen, then opens the new
 * question's page with that kind.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { FIELD_TYPE_OPTIONS } from '#lib/planTemplate.js';
import Page from './+page.svelte';
import { toPageState, type RouteFixture } from '../../../../../../routeFixture.js';
import { fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

async function setup(subject: RouteFixture = fixture) {
	Object.assign(pageState, toPageState(subject));
	await render(Page, {});
}

beforeEach(() => {
	goto.mockReset();
});

describe('the kind of answer for a new Plan Template question (#2193)', () => {
	it('asks one question, every kind by its human name, with nothing chosen', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('group', { name: 'Birth Plan What kind of answer do you need?' }))
			.toBeVisible();
		const radios = testPage.getByRole('radio').elements();
		expect(radios.map((radio) => (radio as HTMLInputElement).labels?.[0]?.textContent?.trim())).toEqual(
			FIELD_TYPE_OPTIONS.map((option) => option.label)
		);
		expect(radios.some((radio) => (radio as HTMLInputElement).checked)).toBe(false);
		await expect
			.element(testPage.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/plan-templates?plan=birth_plan');
	});

	it("opens the new question's page with the kind chosen", async () => {
		await setup();

		await testPage.getByRole('radio', { name: 'Choose one' }).click();
		await testPage.getByRole('button', { name: 'Continue' }).click();

		expect(goto).toHaveBeenCalledWith('/practices/practice-1/settings/plan-templates/birth_plan/new?kind=single_select');
	});

	it('asks again when no kind is chosen', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Continue' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Select the kind of answer you need' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('says so for a plan type there is not', async () => {
		await setup({ ...fixture, params: { ...fixture.params, planType: 'nursing_plan' } });

		await expect.element(testPage.getByText('There is no plan of that kind.')).toBeVisible();
	});
});
