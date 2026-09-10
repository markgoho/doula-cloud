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
import { toApiResponder, toPageState, type RouteVariant } from '../../../../routeFixture.js';
import { fixture, nonOwner } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession,
	apiErrorMessage: (response: Response) => response.text()
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
