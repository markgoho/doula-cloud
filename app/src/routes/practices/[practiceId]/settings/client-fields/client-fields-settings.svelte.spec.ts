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
	apiFetchWithSession
}));

const defaultFields: Field[] = template.fields;
const [firstField] = defaultFields;

interface MockOptions {
	fields?: Field[];
	roles?: string[];
	// Omitted is a session that does not say, which draws no link (#2183).
	hasClients?: boolean;
}

/*
 * #461: read is every Staff member (ADR-0017), write is Owner or Admin
 * (#460's RequireOwnerOrAdmin) -- this mirrors the "load for everyone,
 * gate the write controls" test shape payments-settings.svelte.spec.ts
 * already uses for the same split. The Membership (roles) comes off
 * page.data.session (#835), not a fetch this mock has to answer.
 */
function mockApi({ fields = defaultFields, roles = [], hasClients }: MockOptions = {}) {
	pageState.data = {
		session: {
			practiceId: 'practice-1',
			practiceName: 'Riverside Doula Collective',
			roles,
			isContractor: false,
			hasClients
		}
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

		await expect.element(testPage.getByLabelText('Question', { exact: true }).first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows the editor and a Save button for an Admin', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(testPage.getByLabelText('Question', { exact: true }).first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows a read-only list and no Save button for a Doula', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByText(firstField.label)).toBeVisible();
		await expect.element(testPage.getByLabelText('Question', { exact: true })).not.toBeInTheDocument();
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
 * editing here damage work already done?" -- GLOSSARY.md is explicit that a
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

/*
 * #2194 (MO-G13): the save said "Saved." and nothing else. The status
 * names what was saved and what it changes: a Client's values are read
 * live (GLOSSARY.md, Client Field Template), so the questions are on
 * every Client's record at once, new and existing, and an archived one
 * keeps what was recorded. It is a polite status in place and takes no
 * focus, because Save is still there. A Practice with no Client yet is
 * linked on to her first one.
 */
function saveButton() {
	return testPage.getByRole('button', { name: 'Save' });
}

function firstClientLink() {
	return testPage.getByRole('link', { name: 'Next, add your first Client' });
}

// The editor keeps a status region of its own, so the save's is the one
// that says it saved.
function savedStatus() {
	return testPage.getByRole('status').filter({ hasText: 'Client Fields saved.' });
}

describe('client fields settings screen: what a save says (#2194)', () => {
	it('says what was saved and what it changes from here', async () => {
		await setup({ roles: ['owner'] });

		await saveButton().click();

		await expect
			.element(savedStatus())
			.toHaveTextContent(
				"Client Fields saved. These questions are on every Client's record now, and an archived field keeps what was already recorded."
			);
		await expect.element(saveButton()).toHaveFocus();
	});

	it('links an Admin on to her first Client while the Practice holds none', async () => {
		await setup({ roles: ['admin'], hasClients: false });

		await expect
			.element(firstClientLink())
			.toHaveAttribute('href', '/practices/practice-1/clients/new?from=client-fields');
	});

	it('draws no link once the Practice holds a Client', async () => {
		await setup({ roles: ['owner'], hasClients: true });

		await expect.element(saveButton()).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});

	it('draws no link for a Doula, who cannot change the fields', async () => {
		await setup({ roles: ['doula'], hasClients: false });

		await expect.element(testPage.getByText('Ask a Practice Owner or Admin to edit Client fields.')).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});
});
