/*
 * The Plan Templates list (#2193): GOV.UK's "Add another" shape. A
 * Summary list of the plan type's questions, a Change on each to its own
 * page, one "Add a question", and no Save of its own -- what a question's
 * save did is said here when it lands (#2194, #1710).
 *
 * #866's Care Plan / Birth Plan switcher stays: `aria-selected` names the
 * current type, a roving `tabindex` keeps both reachable, and arrow keys
 * move focus between them without switching the plan shown (manual
 * activation -- selecting a tab fetches its template).
 */
import { page as testPage, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../routeFixture.js';
import { asDoula, birthTemplate, careTemplate, fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const replaceState = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ replaceState, goto: vi.fn() }));

const takeOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ takeOutcome }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession
}));

interface SetupOptions {
	subject?: RouteFixture;
	url?: string;
	outcome?: string;
}

async function setup({ subject = fixture, url = fixture.url, outcome }: SetupOptions = {}) {
	Object.assign(pageState, toPageState({ ...subject, url }));
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(toApiResponder(subject));
	replaceState.mockReset();
	takeOutcome.mockReset();
	takeOutcome.mockReturnValue(outcome);
	await render(Page, {});
	return {
		careTab: testPage.getByRole('tab', { name: 'Care Plan' }),
		birthTab: testPage.getByRole('tab', { name: 'Birth Plan' })
	};
}

function changeLinks() {
	return testPage.getByRole('link', { name: 'Change', exact: true });
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

	it('switches the list, the current tab and the address when a tab is activated', async () => {
		const { careTab, birthTab } = await setup();

		await expect.element(changeLinks().first()).toHaveAccessibleDescription(careTemplate.fields[0].label);

		await birthTab.click();

		await expect.element(changeLinks().first()).toHaveAccessibleDescription(birthTemplate.fields[0].label);
		await expect.element(birthTab).toHaveAttribute('aria-selected', 'true');
		await expect.element(birthTab).toHaveAttribute('tabindex', '0');
		await expect.element(careTab).toHaveAttribute('aria-selected', 'false');
		await expect.element(careTab).toHaveAttribute('tabindex', '-1');
		expect(String(replaceState.mock.calls.at(-1)?.[0])).toBe(
			'https://example.test/practices/practice-1/settings/plan-templates?plan=birth_plan'
		);
	});

	it('opens on the plan type the address names', async () => {
		const { birthTab } = await setup({ url: `${fixture.url}?plan=birth_plan` });

		await expect.element(birthTab).toHaveAttribute('aria-selected', 'true');
		await expect.element(changeLinks().first()).toHaveAccessibleDescription(birthTemplate.fields[0].label);
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

describe('plan-templates settings screen: the list (#2193)', () => {
	it("gives each question a Change to its own page, on the plan type's address", async () => {
		const { birthTab } = await setup();

		await expect.element(changeLinks().first()).toHaveAttribute(
			'href',
			`/practices/practice-1/settings/plan-templates/care_plan/${careTemplate.fields[0].id}`
		);
		expect(changeLinks().elements()).toHaveLength(careTemplate.fields.length);

		await birthTab.click();
		await expect.element(changeLinks().first()).toHaveAttribute(
			'href',
			`/practices/practice-1/settings/plan-templates/birth_plan/${birthTemplate.fields[0].id}`
		);
	});

	it('has one add, which asks the kind of answer first, and no Save', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Add a question' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/plan-templates/care_plan/kind');
		expect(testPage.getByRole('button', { name: 'Save' }).query()).toBeNull();
	});

	it('says what the save that landed here did', async () => {
		await setup({ outcome: 'Care Plan questions saved.' });

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Care Plan questions saved.' })).toBeVisible();
	});

	it('says so when the plan type cannot be read', async () => {
		const refusing: RouteFixture = {
			...fixture,
			respond: () => jsonResponse({ message: 'The Plan Template could not be read.' }, 500)
		};
		await setup({ subject: refusing });

		await expect.element(testPage.getByText('The Plan Template could not be read.')).toBeVisible();
		expect(changeLinks().query()).toBeNull();
	});
});

/*
 * #865: this screen used to open on a bare field editor, so a Practice met
 * a form with no statement of what it was for and no way to know that
 * editing it is safe. One test per fact the intro has to carry.
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
			.element(intro('Every Care Plan and Birth Plan this Practice fills in starts from the questions set here'))
			.toBeVisible();
	});

	it('says the seeded questions are a starting point meant to be changed', async () => {
		await setup();

		await expect.element(intro('the set DoulaCloud seeded is meant to be changed')).toBeVisible();
	});

	it("says editing it never reaches a Client's completed plan (ADR-0001)", async () => {
		await setup();

		await expect
			.element(
				intro('keeps the questions it was filled in against, so nothing changed here reaches a plan already written')
			)
			.toBeVisible();
	});
});

/*
 * #2193 (MO-G12): stage 2 of the solo doula's journey goes from the Plan
 * Templates to the Contract Template. The way on is drawn only for someone
 * who may change the Contract Template (`MAY_CHANGE`, Owner only).
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
		await setup({ subject: { ...fixture, ...asDoula } });

		await expect.element(changeLinks().first()).toBeVisible();
		expect(nextLink().query()).toBeNull();
	});
});
