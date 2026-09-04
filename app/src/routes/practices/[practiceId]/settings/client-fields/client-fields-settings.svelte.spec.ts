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
	sessionOk?: boolean;
}

/*
 * #461: read is every Staff member (ADR-0017), write is Owner or Admin
 * (#460's RequireOwnerOrAdmin) -- this mirrors the "load for everyone,
 * gate the write controls" test shape payments-settings.svelte.spec.ts
 * already uses for the same split.
 */
function mockApi({ fields = defaultFields, roles = [], sessionOk = true }: MockOptions = {}) {
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/session')) {
			return Promise.resolve(jsonResponse({ roles }, sessionOk ? 200 : 401));
		}
		return Promise.resolve(jsonResponse({ fields }));
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

describe('client fields settings screen', () => {
	it('shows the editor and a Save button for an Owner', async () => {
		mockApi({ roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByLabelText('Field label').first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows the editor and a Save button for an Admin', async () => {
		mockApi({ roles: ['admin'] });
		await render(Page, {});

		await expect.element(testPage.getByLabelText('Field label').first()).toHaveValue(firstField.label);
		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
	});

	it('shows a read-only list and no Save button for a Doula', async () => {
		mockApi({ roles: ['doula'] });
		await render(Page, {});

		await expect.element(testPage.getByText(firstField.label)).toBeVisible();
		await expect.element(testPage.getByLabelText('Field label')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText('Ask a Practice Owner or Admin to edit Client fields.'))
			.toBeVisible();
	});

	it('marks an archived field in the read-only list', async () => {
		mockApi({
			roles: ['doula'],
			fields: [{ id: 'a', type: 'short_text', label: 'Old note', order: 0, archived: true }]
		});
		await render(Page, {});

		await expect.element(testPage.getByText('Old note (archived)')).toBeVisible();
	});

	it('falls back to the read-only view if the session roles fetch fails, even for an actual Owner', async () => {
		mockApi({ roles: [], sessionOk: false });
		await render(Page, {});

		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByText('Ask a Practice Owner or Admin to edit Client fields.'))
			.toBeVisible();
	});

	it('warns once the template asks more than 20 questions', async () => {
		const manyFields: Field[] = Array.from({ length: 21 }, (_, index) => ({
			id: `f${index}`,
			type: 'short_text',
			label: `Field ${index}`,
			order: index,
			archived: false
		}));
		mockApi({ roles: ['owner'], fields: manyFields });
		await render(Page, {});

		await expect.element(testPage.getByText('21 questions', { exact: false })).toBeVisible();
	});

	it('shows no count warning at 20 questions or fewer', async () => {
		mockApi({ roles: ['owner'] });
		await render(Page, {});

		await expect.element(testPage.getByText('questions beyond the standard ones', { exact: false })).not.toBeInTheDocument();
	});
});
