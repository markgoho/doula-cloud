/*
 * #1540's setting: an Owner states her Practice's name again. The screen
 * opens on the name the session already holds, says where Clients see it
 * and what does not move before the press, and sends the new one.
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { asDoula, fixture } from './page.fixture.js';
import '#lib/styles/app.css';

const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const invalidateAll = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ invalidateAll }));

const control = () => testPage.getByRole('textbox', { name: 'Practice name' });
const save = () => testPage.getByRole('button', { name: 'Save' }).click();

beforeEach(() => {
	apiFetchWithSession.mockReset();
	invalidateAll.mockReset();
	Object.assign(pageState, toPageState(fixture));
});

async function setup() {
	await render(Page, {});
}

describe('the Practice name screen', () => {
	it('opens on the name the Practice holds', async () => {
		await setup();

		await expect
			.element(control())
			.toHaveValue(
				'Riverside Birth, Postpartum and Lactation Support Collective of the Finger Lakes Region'
			);
		expect(apiFetchWithSession).not.toHaveBeenCalled();
	});

	it('says where Clients see the name, and what does not change, before the press', async () => {
		await setup();

		await expect.element(testPage.getByText(/Your Clients see this name on their portal/)).toBeVisible();
		await expect
			.element(testPage.getByText(/already sent or signed keeps the name it was sent with/))
			.toBeVisible();
		await expect
			.element(testPage.getByText(/name on your Stripe statement does not change/))
			.toBeVisible();
	});

	it('sends the new name, confirms it, and refreshes the Practice session', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ name: 'Willow Birth Services' }));

		await control().fill('Willow Birth Services');
		await save();

		await expect
			.element(
				testPage.getByText('Practice name changed to Willow Birth Services. Your Clients see it from now on.')
			)
			.toBeVisible();
		// Editing again withdraws a confirmation that no longer holds.
		await control().fill('Willow Birth Services LLC');
		await expect
			.element(testPage.getByText(/Practice name changed to/))
			.not.toBeInTheDocument();
		const [path, init] = apiFetchWithSession.mock.calls[0];
		expect(path).toBe('/api/practices/practice-1/name');
		expect(init.method).toBe('PUT');
		expect(JSON.parse(init.body)).toEqual({ name: 'Willow Birth Services' });
		expect(invalidateAll).toHaveBeenCalledOnce();
	});

	it("shows the BFF's sentence for an empty name, linked to the control", async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					code: 'INVALID_ARGUMENT',
					message: 'name is required',
					details: { name: 'Enter the name of your Practice' }
				},
				400
			)
		);

		await control().fill('');
		await save();

		await expect
			.element(testPage.getByRole('link', { name: 'Enter the name of your Practice' }))
			.toBeVisible();
		await expect.element(testPage.getByText(/Practice name changed to/)).not.toBeInTheDocument();
		expect(invalidateAll).not.toHaveBeenCalled();
	});

	it('shows anyone but an Owner the name as a fact, with no Save button', async () => {
		Object.assign(pageState, toPageState({ ...fixture, ...asDoula }));
		await setup();

		await expect
			.element(testPage.getByText('Only a Practice Owner can change the Practice name.'))
			.toBeVisible();
		await expect.element(testPage.getByRole('button', { name: 'Save' })).not.toBeInTheDocument();
		await expect.element(control()).not.toBeInTheDocument();
	});
});
