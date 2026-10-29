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
import { asDoula, birthTemplate, careTemplate, fixture } from './page.fixture.js';

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
			.element(testPage.getByLabelText('Question', { exact: true }).first())
			.toHaveValue(careTemplate.fields[0].label);

		await birthTab.click();

		await expect
			.element(testPage.getByLabelText('Question', { exact: true }).first())
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

/*
 * #865: this screen used to open on a bare field editor, so a Practice met
 * a form with no statement of what it was for and no way to know that
 * editing it is safe. One test per fact the intro has to carry, so a later
 * refactor that drops the `intro` prop fails here rather than quietly
 * shipping the bare screen back.
 *
 * `intro` matches a substring rather than the whole paragraph: the
 * assertion should fail when a fact goes missing, not when a comma moves.
 */
function intro(fact: string) {
	return testPage.getByText(fact, { exact: false });
}

describe('plan-templates settings screen: it introduces itself (#865)', () => {
	it('says what the questions on it are for', async () => {
		await setup();

		await expect
			.element(
				intro(
					'Every Care Plan and Birth Plan this Practice fills in starts from the questions set here'
				)
			)
			.toBeVisible();
	});

	it('says the seeded questions are a starting point meant to be changed', async () => {
		await setup();

		await expect
			.element(intro('the set DoulaCloud seeded is meant to be changed'))
			.toBeVisible();
	});

	it("says editing it never reaches a Client's completed plan (ADR-0001)", async () => {
		await setup();

		await expect
			.element(
				intro(
					'keeps the questions it was filled in against, so nothing changed here reaches a plan already written'
				)
			)
			.toBeVisible();
	});
});

/*
 * #2193 (MO-G12): stage 2 of the solo doula's journey goes from the Plan
 * Templates to the Contract Template, and this screen had no way on, so
 * the walk passed back through the Settings hub. The way on is drawn only
 * for someone who may change the Contract Template (`MAY_CHANGE`, Owner
 * only): a Doula may change the Plan Templates and nothing after them.
 */
function nextLink() {
	return testPage.getByRole('link', { name: 'Next, check your Contract Template' });
}

describe('plan-templates settings screen: the way on to the Contract Template (#2193)', () => {
	it('links an Owner straight on to the Contract Template', async () => {
		await setup();

		await expect.element(nextLink()).toBeVisible();
		await expect.element(nextLink()).toHaveAttribute('href', '/practices/practice-1/settings/contract-template');
	});

	it('draws no way on for a Doula, who may not change the Contract Template', async () => {
		const ownerData = pageState.data;
		pageState.data = toPageState({ ...fixture, ...asDoula }).data;
		try {
			await setup();
			await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
			expect(nextLink().query()).toBeNull();
		} finally {
			pageState.data = ownerData;
		}
	});
});

/*
 * #2194 (MO-G13): the save said "Saved." and nothing about which plans
 * change. The status names the plan type saved, that a plan filled in
 * from now on asks the new questions, and that one already filled in
 * keeps its own (ADR-0001's snapshot). It is a polite status in place and
 * takes no focus, because Save is still there.
 */
function saveButton() {
	return testPage.getByRole('button', { name: 'Save' });
}

// The editor keeps a status region of its own, so the save's is the one
// that says it saved.
function savedStatus() {
	return testPage.getByRole('status').filter({ hasText: 'questions saved.' });
}

describe('plan-templates settings screen: what a save says (#2194)', () => {
	it('names the Care Plan and what its save changes from here', async () => {
		await setup();

		await saveButton().click();

		await expect
			.element(savedStatus())
			.toHaveTextContent(
				'Care Plan questions saved. Every Care Plan filled in from now on asks them, and a Care Plan already filled in keeps the questions it had.'
			);
		await expect.element(saveButton()).toHaveFocus();
	});

	it('names the Birth Plan when the Birth Plan is the one saved', async () => {
		const { birthTab } = await setup();
		await birthTab.click();
		await expect.element(birthTab).toHaveAttribute('aria-selected', 'true');

		await saveButton().click();

		await expect
			.element(savedStatus())
			.toHaveTextContent(
				'Birth Plan questions saved. Every Birth Plan filled in from now on asks them, and a Birth Plan already filled in keeps the questions it had.'
			);
	});

	it('says nothing once another plan type is opened', async () => {
		const { birthTab } = await setup();
		await saveButton().click();
		await expect.element(savedStatus()).toBeVisible();

		await birthTab.click();

		await expect.element(birthTab).toHaveAttribute('aria-selected', 'true');
		expect(savedStatus().query()).toBeNull();
	});
});
