/*
 * One Client field on its own page (#2193). It saves the whole template
 * with this one field changed, added or archived, keeps a saved field to
 * its kind of answer, refuses a structural fact, and lands on the list
 * with #2194's words.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { captureLanding } from '#lib/testOutcome.js';
import type { Field } from '#lib/clientFieldTemplate.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../../routeFixture.js';
import { template } from '../page.fixture.js';
import { clientFieldsSavedMessage } from '../clientFieldPages.js';
import { asDoula, asNewField, changedField, fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const listHref = '/practices/practice-1/settings/client-fields';

// A PUT's own body, sent back as the saved template.
function echo(init: RequestInit): Promise<Response> {
	const sent: unknown = JSON.parse(String(init.body));
	return Promise.resolve(jsonResponse(sent));
}

async function setup(subject: RouteFixture = fixture) {
	Object.assign(pageState, toPageState(subject));
	const read = toApiResponder(subject);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			return echo(init);
		}
		return read(path);
	});
	await render(Page, {});
}

function sentFields(): Field[] {
	const put = apiFetchWithSession.mock.calls.find(
		(call: unknown[]) => (call[1] as RequestInit | undefined)?.method === 'PUT'
	);
	return (JSON.parse(String((put![1] as RequestInit).body)) as { fields: Field[] }).fields;
}

function saveButton() {
	return testPage.getByRole('button', { name: 'Save question' });
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

describe('a Client field (#2193)', () => {
	it('opens the field as it is saved, its kind of answer kept', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Change a question' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Question', { exact: true })).toHaveValue(changedField.label);
		await expect.element(testPage.getByLabelText('Kind of answer')).toBeDisabled();
		// Its place counts only the questions still asked.
		await expect.element(testPage.getByRole('option', { name: '2 of 2' })).toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: 'Back' })).toHaveAttribute('href', listHref);
	});

	it('saves the whole template with this field changed and moved, and lands on the list saying so', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Question', { exact: true }).fill('How did you find us?');
		await testPage.getByLabelText('Place in the list').selectOptions('1 of 2');
		await saveButton().click();

		await expect.poll(() => landing.href).toBe(listHref);
		expect(landing.message).toBe(clientFieldsSavedMessage);
		expect(sentFields().map((field) => [field.id, field.label, field.archived])).toEqual([
			[changedField.id, 'How did you find us?', false],
			[template.fields[0].id, template.fields[0].label, false],
			[template.fields[2].id, template.fields[2].label, true]
		]);
	});

	it('refuses a question that restates a structural fact, and sends nothing', async () => {
		await setup();

		await testPage.getByLabelText('Question', { exact: true }).fill('Email');
		await saveButton().click();

		await expect
			.element(
				testPage.getByRole('link', {
					name: '"Email" is already on every Client record as the email field, so it is not a field to add here.'
				})
			)
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('archives the field once the archive is confirmed, keeping it in the template', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByRole('button', { name: 'Archive question' }).click();
		await testPage.getByRole('dialog').getByRole('button', { name: 'Archive question' }).click();

		await expect.poll(() => landing.href).toBe(listHref);
		expect(sentFields().find((field) => field.id === changedField.id)?.archived).toBe(true);
		expect(sentFields()).toHaveLength(template.fields.length);
	});
});

describe('a Client field not yet saved (#2193)', () => {
	const asNew: RouteFixture = { ...fixture, ...asNewField };

	it('opens empty with the kind chosen and still open, and nothing to archive', async () => {
		await setup(asNew);

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Add a question' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Kind of answer')).toHaveValue('multi_select');
		await expect.element(testPage.getByLabelText('Kind of answer')).toBeEnabled();
		expect(testPage.getByLabelText('Place in the list').query()).toBeNull();
		expect(testPage.getByRole('button', { name: 'Archive question' }).query()).toBeNull();
	});

	it('adds the field at the end, unarchived', async () => {
		await setup(asNew);
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Question', { exact: true }).fill('Languages spoken at home');
		await testPage.getByLabelText('Answers to choose from').fill('English\nSpanish');
		await saveButton().click();

		await expect.poll(() => landing.href).toBe(listHref);
		expect(sentFields().at(-1)).toMatchObject({
			type: 'multi_select',
			label: 'Languages spoken at home',
			options: ['English', 'Spanish'],
			archived: false,
			order: template.fields.length
		});
	});

	it('sends a reader with no kind chosen back to choose one', async () => {
		await setup({ ...asNew, url: 'https://example.test/practices/practice-1/settings/client-fields/new' });

		await expect.element(testPage.getByText('Choose the kind of answer first, from Add a question.')).toBeVisible();
	});

	it('saves a field with no answers to choose from as it is', async () => {
		await setup({ ...asNew, url: `${asNew.url.split('?', 1)[0]}?kind=checkbox` });
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Question', { exact: true }).fill('Has a car seat');
		await saveButton().click();

		await expect.poll(() => landing.href).toBe(listHref);
		expect(sentFields().at(-1)).not.toHaveProperty('options');
	});
});

describe('a Client field that cannot be changed here (#2193)', () => {
	it('tells a Doula who may change it', async () => {
		await setup({ ...fixture, ...asDoula });

		await expect.element(testPage.getByText('Only a Practice Owner or Admin can change the Client Fields.')).toBeVisible();
		expect(saveButton().query()).toBeNull();
	});

	it('says so for an archived field, which is collected again from the list', async () => {
		await setup({ ...fixture, params: { ...fixture.params, questionId: template.fields[2].id } });

		await expect.element(testPage.getByText('This question is not on the Client Fields any more.')).toBeVisible();
	});

	it('says so when the template cannot be read', async () => {
		await setup({
			...fixture,
			respond: () => jsonResponse({ message: 'The Client Field Template could not be read.' }, 500)
		});

		await expect.element(testPage.getByText('The Client Field Template could not be read.')).toBeVisible();
	});
});
