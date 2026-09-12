import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Field } from '#lib/clientFieldTemplate.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { fixture, template } from './page.fixture.js';

/*
 * The Template this screen edits, and the `page` it reads, both come from
 * the route's own fixture (#596) -- so the screen this spec asserts on
 * and the screen the continuum sweep measures are one description.
 * `vi.mock` is hoisted above every import, so `pageState` is declared
 * empty and filled from the fixture once the imports have run.
 */
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

const defaultFields: Field[] = template.fields;
const [firstField] = defaultFields;

interface MockOptions {
	fields?: Field[];
	roles?: string[];
}

/*
 * #461: read is every Staff member (ADR-0017), write is Owner or Admin
 * (#460's RequireOwnerOrAdmin) -- this mirrors the "load for everyone,
 * gate the write controls" test shape payments-settings.svelte.spec.ts
 * already uses for the same split. The Membership (roles) comes off
 * page.data.session (#835), not a fetch this mock has to answer.
 */
function mockApi({ fields = defaultFields, roles = [] }: MockOptions = {}) {
	pageState.data = {
		session: { practiceId: 'practice-1', practiceName: 'Riverside Doula Collective', roles, isContractor: false }
	};
	apiFetchWithSession.mockImplementation(() => Promise.resolve(jsonResponse({ fields })));
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

async function setup(options: MockOptions = {}) {
	mockApi(options);
	await render(Page, {});
}

describe('client fields settings screen', () => {
	it('shows the editor and a Save button for an Owner', async () => {
		await setup({ roles: ['owner'] });

		await expect.element(testPage.getByLabelText('Field label').first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows the editor and a Save button for an Admin', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByLabelText('Field label').first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows a read-only list and no Save button for a Doula', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByText(firstField.label)).toBeVisible();
		await expect.element(testPage.getByLabelText('Field label')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText('Ask a Practice Owner or Admin to edit Client fields.'))
			.toBeVisible();
	});

	it('marks an archived field in the read-only list', async () => {
		await setup({
			roles: ['doula'],
			fields: [{ id: 'a', type: 'short_text', label: 'Old note', order: 0, archived: true }]
		});

		await expect.element(testPage.getByText('Old note (archived)')).toBeVisible();
	});

	it('warns once the template asks more than 20 questions', async () => {
		const manyFields: Field[] = Array.from({ length: 21 }, (_, index) => ({
			id: `f${index}`,
			type: 'short_text',
			label: `Field ${index}`,
			order: index,
			archived: false
		}));
		await setup({ roles: ['owner'], fields: manyFields });

		await expect.element(testPage.getByText('21 questions', { exact: false })).toBeVisible();
	});

	it('shows no count warning at 20 questions or fewer', async () => {
		await setup({ roles: ['owner'] });

		await expect.element(testPage.getByText('questions beyond the standard ones', { exact: false })).not.toBeInTheDocument();
	});
});

/*
 * #865 gave all three template-editing settings screens one voice. This
 * screen already had an intro; the ticket aligned its wording and added
 * the archive reassurance, which is this screen's own answer to "can
 * editing here damage work already done?" -- CONTEXT.md is explicit that a
 * Client's values are read live, never snapshotted, so archiving is the
 * only thing protecting a recorded fact. Asserted for every caller, not
 * only an Owner: orientation is not a write control, and a Doula who can
 * read the list needs to know what it is as much as the Owner who edits
 * it.
 */
/*
 * `intro` matches a substring rather than the whole paragraph: the
 * assertion should fail when a fact goes missing, not when a comma moves.
 */
function intro(fact: string) {
	return testPage.getByText(fact, { exact: false });
}

describe('client fields settings screen: it introduces itself (#865)', () => {
	it('says what the fields are, for a caller who cannot edit them', async () => {
		await setup({ roles: ['doula'] });

		await expect
			.element(
				intro(
					'The extra questions this Practice asks about every Client, beyond name, contact details and address'
				)
			)
			.toBeVisible();
	});

	it('says the list starts empty and that no Client ever sees a field', async () => {
		await setup();

		await expect
			.element(intro('Nothing is here to start with, no Client ever sees one'))
			.toBeVisible();
	});

	it('says removing a field archives it rather than losing what was recorded', async () => {
		await setup();

		await expect
			.element(intro('removing a field archives it, so what was already recorded stays'))
			.toBeVisible();
	});
});
