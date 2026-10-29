/*
 * One Plan Template question on its own page (#2193). It saves the whole
 * template with this one question changed, added or removed, and lands on
 * the list on its own tab with #2194's words.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { captureLanding } from '#lib/testOutcome.js';
import type { Field } from '#lib/planTemplate.js';
import Page from './+page.svelte';
import { toApiResponder, toPageState, type RouteFixture } from '../../../../../../routeFixture.js';
import { careTemplate } from '../../page.fixture.js';
import { asNewQuestion, changedQuestion, fixture } from './page.fixture.js';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, replaceState: vi.fn() }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const templatePath = '/api/practices/practice-1/plan-templates/care_plan';
const savedMessage =
	'Care Plan questions saved. Every Care Plan filled in from now on asks them, and a Care Plan already filled in keeps the questions it had.';

/** The fixture's read, and a PUT that echoes what it is sent -- or
 * refuses it, where `putRefusal` says how. */
// A PUT's own body, sent back as the saved template.
function echo(init: RequestInit): Promise<Response> {
	const sent: unknown = JSON.parse(String(init.body));
	return Promise.resolve(jsonResponse(sent));
}

async function setup(subject: RouteFixture = fixture, putRefusal?: string) {
	Object.assign(pageState, toPageState(subject));
	const read = toApiResponder(subject);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (init?.method === 'PUT') {
			if (putRefusal) return Promise.resolve(jsonResponse({ message: putRefusal }, 400));
			return echo(init);
		}
		return read(path);
	});
	await render(Page, {});
}

function sentFields(): Field[] {
	const put = apiFetchWithSession.mock.calls.find(
		(call: unknown[]) => call[0] === templatePath && (call[1] as RequestInit | undefined)?.method === 'PUT'
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

describe('a Plan Template question (#2193)', () => {
	it('opens the question as it is saved, under its plan type', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Change a question' })).toBeVisible();
		await expect.element(testPage.getByText('Care Plan', { exact: true })).toBeVisible();
		await expect.element(testPage.getByLabelText('Question', { exact: true })).toHaveValue(changedQuestion.label);
		await expect
			.element(testPage.getByLabelText('Answers to choose from'))
			.toHaveValue(changedQuestion.options!.join('\n'));
		await expect.element(testPage.getByLabelText('Place in the list')).toHaveValue('2');
		await expect
			.element(testPage.getByRole('link', { name: 'Back' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/plan-templates');
	});

	it('saves the whole template with this question changed, and lands on the list saying so', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Question', { exact: true }).fill('  Who is with you?  ');
		await testPage.getByLabelText('Answers to choose from').fill('Partner\n\n  Mother ');
		await saveButton().click();

		await expect.poll(() => landing.href).toBe('/practices/practice-1/settings/plan-templates');
		expect(landing.message).toBe(savedMessage);
		expect(sentFields()).toEqual([
			{ ...careTemplate.fields[0], order: 0 },
			{ ...changedQuestion, label: 'Who is with you?', options: ['Partner', 'Mother'], order: 1 }
		]);
	});

	it('moves the question to the place chosen', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Place in the list').selectOptions('1 of 2');
		await saveButton().click();

		await expect.poll(() => landing.href).toBeDefined();
		expect(sentFields().map((field) => [field.id, field.order])).toEqual([
			[changedQuestion.id, 0],
			[careTemplate.fields[0].id, 1]
		]);
	});

	it('drops the answers when the kind of answer stops being a choice', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Kind of answer').selectOptions('Short answer');
		await saveButton().click();

		await expect.poll(() => landing.href).toBeDefined();
		expect(sentFields()[1]).toEqual({ id: changedQuestion.id, type: 'short_text', label: changedQuestion.label, order: 1 });
	});

	it('refuses a question with no words or no answers, and sends nothing', async () => {
		await setup();

		await testPage.getByLabelText('Question', { exact: true }).fill(' ');
		await testPage.getByLabelText('Answers to choose from').fill('\n');
		await saveButton().click();

		await expect.element(testPage.getByRole('link', { name: 'Enter the question' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Enter the answers to choose from, one on each line' }))
			.toBeVisible();
		expect(goto).not.toHaveBeenCalled();
		expect(apiFetchWithSession.mock.calls.some((call: unknown[]) => (call[1] as RequestInit | undefined)?.method === 'PUT')).toBe(false);
	});

	it('says what the boundary refused', async () => {
		await setup(fixture, 'The template could not be saved.');

		await saveButton().click();

		await expect.element(testPage.getByText('The template could not be saved.')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('removes the question once the removal is confirmed', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByRole('button', { name: 'Remove question' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await testPage.getByRole('dialog').getByRole('button', { name: 'Remove question' }).click();

		await expect.poll(() => landing.href).toBe('/practices/practice-1/settings/plan-templates');
		expect(landing.message).toBe(savedMessage);
		expect(sentFields().map((field) => field.id)).toEqual([careTemplate.fields[0].id]);
	});
});

describe('a Plan Template question not yet saved (#2193)', () => {
	const asNew: RouteFixture = { ...fixture, ...asNewQuestion };

	it('opens empty with the kind chosen, no place to choose and nothing to remove', async () => {
		await setup(asNew);

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Add a question' })).toBeVisible();
		await expect.element(testPage.getByLabelText('Question', { exact: true })).toHaveValue('');
		await expect.element(testPage.getByLabelText('Kind of answer')).toHaveValue('single_select');
		expect(testPage.getByLabelText('Place in the list').query()).toBeNull();
		expect(testPage.getByRole('button', { name: 'Remove question' }).query()).toBeNull();
	});

	it('adds the question at the end of the template', async () => {
		await setup(asNew);
		const landing = captureLanding(goto);

		await testPage.getByLabelText('Question', { exact: true }).fill('Hospital transfer plan');
		await testPage.getByLabelText('Answers to choose from').fill('Drive\nAmbulance');
		await saveButton().click();

		await expect.poll(() => landing.href).toBeDefined();
		const sent = sentFields();
		expect(sent.slice(0, 2)).toEqual(careTemplate.fields.map((field, order) => ({ ...field, order })));
		expect(sent[2]).toMatchObject({ type: 'single_select', label: 'Hospital transfer plan', options: ['Drive', 'Ambulance'], order: 2 });
	});

	it('sends a reader with no kind chosen back to choose one', async () => {
		await setup({ ...asNew, url: asNew.url.replace('?kind=single_select', '') });

		await expect.element(testPage.getByText('Choose the kind of answer first, from Add a question.')).toBeVisible();
	});
});

describe('a Plan Template question that cannot be opened (#2193)', () => {
	it('says so for a question the template no longer holds', async () => {
		await setup({ ...fixture, params: { ...fixture.params, questionId: 'gone' } });

		await expect.element(testPage.getByText('This question is not on the Care Plan any more.')).toBeVisible();
	});

	it('says so for a plan type there is not', async () => {
		await setup({ ...fixture, params: { ...fixture.params, planType: 'nursing_plan' } });

		await expect.element(testPage.getByText('There is no plan of that kind.')).toBeVisible();
	});

	it('says so when the template cannot be read', async () => {
		await setup({ ...fixture, respond: () => jsonResponse({ message: 'The Plan Template could not be read.' }, 500) });

		await expect.element(testPage.getByText('The Plan Template could not be read.')).toBeVisible();
	});
});
