/*
 * #865: the Contract Template screen used to open on a bare prose editor,
 * naming neither what the terms are for nor the fact that makes editing
 * them safe. This is the screen's first spec, so it covers the intro the
 * ticket added and nothing else -- the Owner/non-Owner split (#970) is
 * already realized as a fixture variant and swept there.
 *
 * The first fact is asserted for both callers on purpose. A non-Owner
 * reads this screen and cannot save from it, but "editing here cannot
 * reach a Contract already written" is orientation, not a write control,
 * and a screen that explains itself only to the person who may change it
 * leaves everyone else guessing.
 */
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { practiceSession, toApiResponder, toPageState, type RouteVariant } from '../../../../routeFixture.js';
import { fixture, noClientYet, nonOwner } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession
}));

/*
 * What the screen says it is -- the one fact both sessions are asked for.
 */
const PURPOSE = 'Every Contract this Practice sends starts from the terms written here';

interface SetupOptions {
	/*
	 * The fixture variant to mount. Omitted is the fixture's own session,
	 * which is an Owner.
	 */
	as?: RouteVariant;
}

async function setup({ as }: SetupOptions = {}) {
	Object.assign(pageState, toPageState(as ? { ...fixture, ...as } : fixture));
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	await render(Page, {});
}

/*
 * `intro` matches a substring rather than the whole paragraph: the
 * assertion should fail when a fact goes missing, not when a comma moves.
 */
function intro(fact: string) {
	return testPage.getByText(fact, { exact: false });
}

describe('contract-template settings screen: it introduces itself (#865)', () => {
	it('says what the terms on it are for', async () => {
		await setup();

		await expect.element(intro(PURPOSE)).toBeVisible();
	});

	it('says the seeded terms are a starting point meant to be replaced', async () => {
		await setup();

		await expect
			.element(intro("are meant to be replaced with this Practice's own"))
			.toBeVisible();
	});

	it('says editing the terms never reaches a Contract already written', async () => {
		await setup();

		await expect
			.element(
				intro(
					'keeps the terms it was written from, so nothing changed here reaches a Contract already made'
				)
			)
			.toBeVisible();
	});

	it('introduces itself to a caller who cannot change the terms', async () => {
		await setup({ as: nonOwner });

		await expect.element(intro(PURPOSE)).toBeVisible();
	});
});

/*
 * #2194 (MO-G13): the save said "Saved." and nothing else, which tells
 * her something happened but not what, nor what it changes. The status
 * names what was saved and what it reaches from here; it is a polite
 * status in place, and takes no focus, because Save is still there
 * (docs/design/govuk-alignment.md, Notification banner).
 */
const SAVED =
	'Contract Template saved. Every Contract written from now on uses these terms, and a Contract already written keeps the terms it was written from.';

function saveButton() {
	return testPage.getByRole('button', { name: 'Save' });
}

function firstClientLink() {
	return testPage.getByRole('link', { name: 'Next, add your first Client' });
}

describe('contract-template settings screen: what a save says (#2194)', () => {
	it('says what was saved and what it changes from here', async () => {
		await setup();

		await saveButton().click();

		await expect.element(testPage.getByRole('status')).toHaveTextContent(SAVED);
	});

	it('leaves focus on Save, which is still there', async () => {
		await setup();

		await saveButton().click();

		await expect.element(testPage.getByRole('status')).toBeVisible();
		await expect.element(saveButton()).toHaveFocus();
	});

	it('links an Owner on to her first Client while the Practice holds none', async () => {
		await setup({ as: noClientYet });

		await expect
			.element(firstClientLink())
			.toHaveAttribute('href', '/practices/practice-1/clients/new?from=contract-template');
	});

	it('draws no link once the Practice holds a Client', async () => {
		await setup();

		await expect.element(saveButton()).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});

	it('draws no link for a caller who cannot change the terms', async () => {
		await setup({
			as: { ...nonOwner, pageData: { session: { ...practiceSession(['admin']).session, hasClients: false } } }
		});

		await expect.element(intro(PURPOSE)).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});
});

/*
 * #2244: the PUT is OwnerOnly (#970), and a non-Owner used to get the
 * same editable field an Owner does, with no Save: whatever she typed was
 * thrown away when she left. She reads the terms now, as text, with the
 * merge fields still explained; an Owner's editor is unchanged.
 */
const PROSE_START = 'This Contract is between {{practice_name}} and {{client_name}}';

describe('contract-template settings screen: who gets the editor (#2244)', () => {
	it('gives an Owner the prose to change, and Save', async () => {
		await setup();

		await expect.element(testPage.getByRole('textbox', { name: 'Contract template prose' })).toBeVisible();
		await expect.element(saveButton()).toBeVisible();
	});

	it.each([
		['an Admin', nonOwner],
		['a Doula', { ...nonOwner, pageData: practiceSession(['doula']) }]
	])('gives %s the terms to read, with no field and no Save', async (_who, as) => {
		await setup({ as });

		await expect.element(testPage.getByText(PROSE_START, { exact: false })).toBeVisible();
		await expect.element(testPage.getByText('Only a Practice Owner can change these terms.')).toBeVisible();
		await expect.element(testPage.getByRole('heading', { name: 'Merge-field placeholders' })).toBeVisible();
		expect(testPage.getByRole('textbox').query()).toBeNull();
		expect(saveButton().query()).toBeNull();
	});
});
