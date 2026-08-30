import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientMatch } from '#lib/client.js';
import Page from './+page.svelte';

const searchParameters = vi.hoisted(() => new URLSearchParams());
vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1' }, url: { searchParams: searchParameters } }
}));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const savedRecord = {
	id: 'client-1',
	givenName: 'Ada',
	familyName: '',
	preferredName: '',
	email: '',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: ''
};

const anotherClientMatch: ClientMatch = {
	id: 'client-2',
	givenName: 'Ada',
	familyName: 'Byron',
	preferredName: '',
	email: 'ada.byron@example.com',
	phone: '555-0100',
	addressLine1: '1 Analytical Engine Way',
	addressLine2: '',
	addressLocality: 'London',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '1815-12-10',
	fieldValues: { pronouns: 'she/her' },
	engagements: [{ engagementId: 'engagement-1', kind: 'birth', status: 'active', createdAt: '2024-01-01' }]
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	for (const key of searchParameters.keys()) searchParameters.delete(key);
});

async function setup() {
	await render(Page, {});
}

async function fillNameAndContinue(given = 'Ada') {
	await testPage.getByLabelText('Given name').fill(given);
	await testPage.getByRole('button', { name: 'Add contact details' }).click();
}

async function continueThroughContact(name = 'Ada') {
	await testPage.getByRole('button', { name: `Add ${name}'s date of birth` }).click();
}

function requestBody(callIndex: number): Record<string, unknown> {
	const init = apiFetchWithSession.mock.calls[callIndex][1] as RequestInit;
	return JSON.parse(init.body as string) as Record<string, unknown>;
}

describe('client intake (#497)', () => {
	// The `<h1>` itself is never the focus target -- it sits inside a
	// `tabindex="-1"` wrapper, the same shape ErrorSummary's own `.summary`
	// takes, so `document.activeElement` is asserted against that wrapper's
	// stable id rather than against the heading's accessible role.
	it('is three pages in sequence, each taking focus on its own h1 as it appears', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { level: 1, name: "What is the Client's name?" }))
			.toBeVisible();
		expect(document.activeElement?.id).toBe('intake-heading');

		await fillNameAndContinue();
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: 'How do you contact Ada?' }))
			.toBeVisible();
		expect(document.activeElement?.id).toBe('intake-heading');

		await continueThroughContact();
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: "What is Ada's date of birth?" }))
			.toBeVisible();
		expect(document.activeElement?.id).toBe('intake-heading');
	});

	it('demands only a given name, refusing a blank submit and moving focus to the error region', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Add contact details' }).click();

		await expect
			.element(testPage.getByRole('link', { name: "Enter the Client's given name" }))
			.toBeVisible();
		// ErrorSummary takes focus on its own container -- asserted by
		// content rather than by its internal class name, which belongs to
		// that component's own spec.
		expect(document.activeElement?.textContent).toContain('There is a problem');
	});

	it('names what every button does next, never a bare Next or Submit', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Add contact details' })).toBeVisible();
		await fillNameAndContinue();
		await expect.element(testPage.getByRole('button', { name: "Add Ada's date of birth" })).toBeVisible();
		await continueThroughContact();
		await expect.element(testPage.getByRole('button', { name: "Save Ada's record" })).toBeVisible();
	});

	it('keeps every field about the Client rather than the signed-in staff member (#469)', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Given name')).toHaveAttribute('autocomplete', 'off');
		await expect.element(testPage.getByLabelText('Family name')).toHaveAttribute('autocomplete', 'off');
		await expect.element(testPage.getByLabelText('Preferred name')).toHaveAttribute('autocomplete', 'off');
		await fillNameAndContinue();
		await expect.element(testPage.getByLabelText('Phone')).toHaveAttribute('autocomplete', 'off');
		await expect.element(testPage.getByLabelText('Email')).toHaveAttribute('autocomplete', 'off');
		await continueThroughContact();
		await expect.element(testPage.getByLabelText('Date of birth')).toHaveAttribute('autocomplete', 'off');
	});

	it('prefills the given name from a query param, for the search that will front intake (#498)', async () => {
		searchParameters.set('name', 'Grace');

		await setup();

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue('Grace');
	});

	it('saves with only a given name filled, spending no Credit, and lands on the Client detail hub', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(savedRecord, 201));
		await setup();

		await fillNameAndContinue();
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();

		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
		const body = requestBody(0);
		expect(body.override).toBe(false);
		expect(body.givenName).toBe('Ada');
		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-1');
	});

	it('shows the save-time match prompt before writing anything, naming the match', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));
		await setup();

		await fillNameAndContinue();
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();

		await expect.element(testPage.getByRole('heading', { level: 1, name: 'Before this is saved' })).toBeVisible();
		expect(document.activeElement?.id).toBe('intake-heading');
		await expect.element(testPage.getByRole('heading', { level: 2, name: 'Ada Byron' })).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('"No, a different person" writes a new Client with override set', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(savedRecord, 201));
		await setup();

		await fillNameAndContinue();
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();
		await testPage.getByRole('button', { name: 'No, a different person' }).click();

		expect(requestBody(1).override).toBe(true);
		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-1');
	});

	it('"This is <name>" lists the proposed edit before applying it, then edits the existing record', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...anotherClientMatch, email: 'ada@newaddress.example' }));
		await setup();

		await fillNameAndContinue();
		await testPage.getByLabelText('Email').fill('ada@newaddress.example');
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();
		await testPage.getByRole('button', { name: 'This is Ada Byron' }).click();

		// The review screen -- a check-answers shape, not a sentence in a dialog.
		await expect
			.element(testPage.getByRole('heading', { level: 1, name: "Save changes to Ada Byron's record" }))
			.toBeVisible();
		expect(document.activeElement?.id).toBe('intake-heading');
		await expect.element(testPage.getByText('ada.byron@example.com → ada@newaddress.example')).toBeVisible();

		await testPage.getByRole('button', { name: "Save changes to Ada Byron's record" }).click();

		const editCall = apiFetchWithSession.mock.calls[1];
		expect(editCall[0]).toBe('/api/practices/practice-1/clients/client-2');
		const body = JSON.parse((editCall[1] as RequestInit).body as string) as {
			email: string;
			addressLine1: string;
			fieldValues: unknown;
			override: boolean;
		};
		expect(body.email).toBe('ada@newaddress.example');
		// The matched Client's own address and Practice-defined values ride
		// through unchanged (#495's hazard).
		expect(body.addressLine1).toBe('1 Analytical Engine Way');
		expect(body.fieldValues).toEqual({ pronouns: 'she/her' });
		expect(body.override).toBe(false);
		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-2');
	});

	it('"This is <name>" goes straight to her record when nothing typed differs from what is on file', async () => {
		const exactMatch: ClientMatch = { ...anotherClientMatch, givenName: 'Ada', familyName: 'Byron' };
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [exactMatch] }, 409));
		await setup();

		await testPage.getByLabelText('Given name').fill('Ada');
		await testPage.getByLabelText('Family name').fill('Byron');
		await testPage.getByRole('button', { name: 'Add contact details' }).click();
		await continueThroughContact('Ada');
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();
		await testPage.getByRole('button', { name: 'This is Ada Byron' }).click();

		// No edit call -- there was nothing to propose.
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-2');
	});

	it("reuses #495's override dialog when the reviewed edit itself matches a third Client", async () => {
		const thirdMatch: ClientMatch = { ...anotherClientMatch, id: 'client-3', givenName: 'Nadia', familyName: 'Haddad', engagements: [] };
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [anotherClientMatch] }, 409));
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ matches: [thirdMatch] }, 409));
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...anotherClientMatch, email: 'ada@newaddress.example' }));
		await setup();

		await fillNameAndContinue();
		await testPage.getByLabelText('Email').fill('ada@newaddress.example');
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();
		await testPage.getByRole('button', { name: 'This is Ada Byron' }).click();
		await testPage.getByRole('button', { name: "Save changes to Ada Byron's record" }).click();

		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await expect.element(testPage.getByText('Nadia Haddad', { exact: false })).toBeVisible();

		await testPage.getByRole('button', { name: 'Save as a different person' }).click();

		expect(JSON.parse((apiFetchWithSession.mock.calls[2][1] as RequestInit).body as string).override).toBe(true);
		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith('/practices/practice-1/clients/client-2');
	});

	it('surfaces a refused save as an error rather than a silent failure, without moving on', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('a contractor doula does not create clients', 403));
		await setup();

		await fillNameAndContinue();
		await continueThroughContact();
		await testPage.getByRole('button', { name: "Save Ada's record" }).click();

		await expect.element(testPage.getByText('a contractor doula does not create clients')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
