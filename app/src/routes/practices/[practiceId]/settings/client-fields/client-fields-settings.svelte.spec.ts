/*
 * The Client Fields list (#2193): GOV.UK's "Add another" shape on the
 * Plan Templates' component. A row per question still asked, a Change on
 * each to its own page, one "Add a question", and the archived questions
 * as a list of their own with "Collect again". What a question's save did
 * is said here when it lands (#2194, #1710).
 *
 * Read is every Staff member (ADR-0017), write is Owner or Admin (#460).
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { Field } from '#lib/clientFieldTemplate.js';
import Page from './+page.svelte';
import { practiceSession, toPageState } from '../../../../routeFixture.js';
import { fixture, template } from './page.fixture.js';
import { clientFieldsSavedMessage } from './clientFieldPages.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const takeOutcome = vi.hoisted(() => vi.fn());
vi.mock('#lib/outcome.js', () => ({ takeOutcome, gotoWithOutcome: vi.fn() }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({
	apiFetchWithSession
}));

const activeFields = template.fields.filter((field) => !field.archived);
const archivedField = template.fields.find((field) => field.archived)!;

interface SetupOptions {
	fields?: Field[];
	roles?: string[];
	// Omitted is a session that does not say, which draws no link (#2183).
	hasClients?: boolean;
	outcome?: string;
	putRefusal?: string;
}

// A PUT's own body, sent back as the saved template.
function echo(init: RequestInit): Promise<Response> {
	const sent: unknown = JSON.parse(String(init.body));
	return Promise.resolve(jsonResponse(sent));
}

async function setup({ fields = template.fields, roles = ['owner'], hasClients, outcome, putRefusal }: SetupOptions = {}) {
	pageState.data = { session: { ...practiceSession(roles).session, hasClients } };
	takeOutcome.mockReturnValue(outcome);
	apiFetchWithSession.mockImplementation((_path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			if (putRefusal) return Promise.resolve(jsonResponse({ message: putRefusal }, 400));
			return echo(init);
		}
		return Promise.resolve(jsonResponse({ fields }));
	});
	await render(Page, {});
}

function changeLinks() {
	return testPage.getByRole('link', { name: 'Change', exact: true });
}

function addLink() {
	return testPage.getByRole('link', { name: 'Add a question' });
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	takeOutcome.mockReset();
});

describe('client fields settings screen (#2193)', () => {
	it('lists each question still asked with a Change to its own page, for an Owner', async () => {
		await setup();

		expect(changeLinks().elements()).toHaveLength(activeFields.length);
		await expect
			.element(changeLinks().first())
			.toHaveAttribute('href', `/practices/practice-1/settings/client-fields/${activeFields[0].id}`);
		await expect.element(addLink()).toHaveAttribute('href', '/practices/practice-1/settings/client-fields/kind');
		expect(testPage.getByRole('button', { name: 'Save' }).query()).toBeNull();
	});

	it('lets an Admin change it too', async () => {
		await setup({ roles: ['admin'] });

		await expect.element(addLink()).toBeVisible();
	});

	it('shows a Doula the same list, with no way to change it and who can', async () => {
		await setup({ roles: ['doula'] });

		await expect.element(testPage.getByText(activeFields[1].label)).toBeVisible();
		expect(changeLinks().query()).toBeNull();
		expect(addLink().query()).toBeNull();
		expect(testPage.getByRole('button', { name: 'Collect again' }).query()).toBeNull();
		await expect.element(testPage.getByText('Ask a Practice Owner or Admin to change the Client Fields.')).toBeVisible();
	});

	it('says what the save that landed here did', async () => {
		await setup({ outcome: clientFieldsSavedMessage });

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Client Fields saved.' })).toBeVisible();
	});

	it('says so when the template cannot be read', async () => {
		apiFetchWithSession.mockImplementation(() =>
			Promise.resolve(jsonResponse({ message: 'The Client Field Template could not be read.' }, 500))
		);
		pageState.data = { session: practiceSession(['owner']).session };
		await render(Page, {});

		await expect.element(testPage.getByText('The Client Field Template could not be read.')).toBeVisible();
	});

	it('warns once the template asks more than 20 questions', async () => {
		const many: Field[] = Array.from({ length: 21 }, (_, index) => ({
			id: `field-${index}`,
			type: 'short_text',
			label: `Question ${index}`,
			order: index,
			archived: false
		}));
		await setup({ fields: many });

		await expect.element(testPage.getByText('This template asks a Client 21 questions', { exact: false })).toBeVisible();
	});

	it('shows no count warning at 20 questions or fewer', async () => {
		await setup();

		await expect.element(addLink()).toBeVisible();
		expect(testPage.getByText('questions beyond the standard ones', { exact: false }).query()).toBeNull();
	});
});

describe('client fields settings screen: the archived questions (#461, #2193)', () => {
	it('lists them apart, as no longer asked', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Archived questions' })).toBeVisible();
		await expect.element(testPage.getByText('No longer asked')).toBeVisible();
	});

	it('collects one again from here, saving at once and saying so', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Collect again' }).click();

		await expect.element(testPage.getByRole('status').filter({ hasText: 'Client Fields saved.' })).toBeVisible();
		const put = apiFetchWithSession.mock.calls.find((call: unknown[]) => (call[1] as RequestInit | undefined)?.method === 'PUT');
		const sent = (JSON.parse(String((put![1] as RequestInit).body)) as { fields: Field[] }).fields;
		expect(sent.find((field) => field.id === archivedField.id)?.archived).toBe(false);
		expect(testPage.getByRole('heading', { name: 'Archived questions' }).query()).toBeNull();
		expect(changeLinks().elements()).toHaveLength(template.fields.length);
	});

	it('says what the boundary refused', async () => {
		await setup({ putRefusal: 'The template could not be saved.' });

		await testPage.getByRole('button', { name: 'Collect again' }).click();

		await expect.element(testPage.getByText('The template could not be saved.')).toBeVisible();
	});

	it('draws no archived list when nothing is archived', async () => {
		await setup({ fields: activeFields });

		await expect.element(addLink()).toBeVisible();
		expect(testPage.getByRole('heading', { name: 'Archived questions' }).query()).toBeNull();
	});
});

/*
 * #865: the intro, for every caller -- orientation is not a write control.
 * `intro` matches a substring, so it fails when a fact goes missing, not
 * when a comma moves.
 */
function intro(fact: string) {
	return testPage.getByText(fact, { exact: false });
}

describe('client fields settings screen: it introduces itself (#865)', () => {
	it('says what the fields are, for a caller who cannot edit them', async () => {
		await setup({ roles: ['doula'] });

		await expect
			.element(intro('The extra questions this Practice asks about every Client, beyond name, contact details and address'))
			.toBeVisible();
	});

	it('says the list starts empty and that no Client ever sees a field', async () => {
		await setup();

		await expect.element(intro('Nothing is here to start with, no Client ever sees one')).toBeVisible();
	});

	it('says removing a field archives it rather than losing what was recorded', async () => {
		await setup();

		await expect.element(intro('removing a field archives it, so what was already recorded stays')).toBeVisible();
	});
});

/*
 * #2194: a Practice with no Client yet is linked on to her first one.
 */
function firstClientLink() {
	return testPage.getByRole('link', { name: 'Next, add your first Client' });
}

describe('client fields settings screen: the way on (#2194)', () => {
	it('links an Admin on to her first Client while the Practice holds none', async () => {
		await setup({ roles: ['admin'], hasClients: false });

		await expect
			.element(firstClientLink())
			.toHaveAttribute('href', '/practices/practice-1/clients/new?from=client-fields');
	});

	it('draws no link once the Practice holds a Client', async () => {
		await setup({ hasClients: true });

		await expect.element(addLink()).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});

	it('draws no link for a Doula, who cannot change the fields', async () => {
		await setup({ roles: ['doula'], hasClients: false });

		await expect.element(testPage.getByText('Ask a Practice Owner or Admin to change the Client Fields.')).toBeVisible();
		expect(firstClientLink().query()).toBeNull();
	});
});
