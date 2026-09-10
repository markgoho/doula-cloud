/*
 * #865: the Contract Template screen used to open on a bare prose editor,
 * naming neither what the terms are for nor the fact that makes editing
 * them safe. This is the screen's first spec, so it covers the intro the
 * ticket added and nothing else -- the Owner/non-Owner split (#970) is
 * already realized as a fixture variant and swept there.
 *
 * The reassurance is asserted for both callers on purpose. A non-Owner
 * reads this screen and cannot save from it, but "editing here cannot
 * reach a Contract already written" is orientation, not a write control,
 * and a screen that explains itself only to the person who may change it
 * leaves everyone else guessing.
 */
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
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

interface SetupOptions {
	/*
	 * The fixture variant to mount, defaulting to the base Owner.
	 */
	as?: typeof nonOwner;
}

async function setup({ as }: SetupOptions = {}) {
	Object.assign(pageState, toPageState(as ? { ...fixture, ...as } : fixture));
	apiFetchWithSession.mockReset();
	apiFetchWithSession.mockImplementation(toApiResponder(fixture));
	await render(Page, {});
}

describe('contract-template settings screen: it introduces itself (#865)', () => {
	it('says what the terms on it are for', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText(
					'Every Contract this Practice sends starts from the terms written here',
					{ exact: false }
				)
			)
			.toBeVisible();
	});

	it('says the seeded terms are a starting point meant to be replaced', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText("it is meant to be replaced with this Practice's own", { exact: false })
			)
			.toBeVisible();
	});

	it('says editing the terms never reaches a Contract already written', async () => {
		await setup();

		await expect
			.element(
				testPage.getByText(
					'keeps the terms it was written from, so nothing changed here reaches a Contract already made',
					{ exact: false }
				)
			)
			.toBeVisible();
	});

	it('introduces itself to a caller who cannot change the terms', async () => {
		await setup({ as: nonOwner });

		await expect
			.element(
				testPage.getByText(
					'Every Contract this Practice sends starts from the terms written here',
					{ exact: false }
				)
			)
			.toBeVisible();
	});
});
