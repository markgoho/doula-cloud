/*
 * #866: the Care Plan / Birth Plan switcher used to mark the current type
 * by disabling that button -- a disabled control drops out of the tab
 * order and is announced as unavailable, not as current. These specs
 * exercise the GOV.UK Tabs replacement: `aria-selected` names the current
 * type, a roving `tabindex` keeps both reachable, and arrow keys move
 * focus between them without switching the plan shown (manual
 * activation -- selecting a tab fetches its template).
 */
import { page as testPage, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { birthTemplate, careTemplate, fixture } from './page.fixture.js';

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

async function setup() {
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	await render(Page, {});
	return {
		careTab: testPage.getByRole('tab', { name: 'Care Plan' }),
		birthTab: testPage.getByRole('tab', { name: 'Birth Plan' })
	};
}

describe('plan-templates settings screen: the plan-type switcher (#866)', () => {
	it('marks the current plan type with aria-selected, never with disabled', async () => {
		const { careTab, birthTab } = await setup();

		await expect.element(careTab).toBeVisible();
		await expect.element(careTab).toHaveAttribute('aria-selected', 'true');
		await expect.element(careTab).not.toHaveAttribute('disabled');
		await expect.element(birthTab).toHaveAttribute('aria-selected', 'false');
		await expect.element(birthTab).not.toHaveAttribute('disabled');
	});

	it('keeps every plan type reachable and focusable, current or not', async () => {
		const { careTab, birthTab } = await setup();

		await expect.element(careTab).toHaveAttribute('tabindex', '0');
		await expect.element(birthTab).toHaveAttribute('tabindex', '-1');

		birthTab.element().focus();
		expect(document.activeElement).toBe(birthTab.element());
	});

	it('switches the panel and the current tab when a tab is activated', async () => {
		const { careTab, birthTab } = await setup();

		await expect
			.element(testPage.getByLabelText('Field label').first())
			.toHaveValue(careTemplate.fields[0].label);

		await birthTab.click();

		await expect
			.element(testPage.getByLabelText('Field label').first())
			.toHaveValue(birthTemplate.fields[0].label);
		await expect.element(birthTab).toHaveAttribute('aria-selected', 'true');
		await expect.element(birthTab).toHaveAttribute('tabindex', '0');
		await expect.element(careTab).toHaveAttribute('aria-selected', 'false');
		await expect.element(careTab).toHaveAttribute('tabindex', '-1');
	});

	it('moves focus with arrow keys without switching the plan shown', async () => {
		const { careTab, birthTab } = await setup();

		careTab.element().focus();
		await userEvent.keyboard('{ArrowRight}');

		expect(document.activeElement).toBe(birthTab.element());
		// Manual activation: moving focus alone does not select the tab or
		// fetch its template -- only Enter/Space (a real click) does.
		await expect.element(careTab).toHaveAttribute('aria-selected', 'true');

		await userEvent.keyboard('{ArrowLeft}');
		expect(document.activeElement).toBe(careTab.element());
	});
});
