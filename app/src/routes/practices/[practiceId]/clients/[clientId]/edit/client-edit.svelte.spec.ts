import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { ClientDetail } from '#lib/clientDetail.js';
import type { CollisionMatch } from '#lib/client.js';
import { editMergeDraft } from '#lib/editMergeDraft.svelte.js';
import Page from './+page.svelte';
import { toPageState } from '../../../../../routeFixture.js';
import { detail as baseDetail, fixture } from './page.fixture.js';

/*
 * The Client this form edits, and the `page` it reads, both come from the
 * route's own fixture (#596) -- so what this spec asserts on and what the
 * continuum sweep measures are one description. `vi.mock` is hoisted
 * above every import, so `pageState` is declared empty and filled from
 * the fixture once the imports have run. Same installation, through the
 * same `toPageState`, as `route-continuum.svelte.spec.ts`.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const { practiceId, clientId } = fixture.params;
const detailHref = `/practices/${practiceId}/clients/${clientId}`;
const editDuplicateHref = `${detailHref}/edit/duplicate`;

const anotherClientMatch: CollisionMatch = {
	id: 'client-2',
	givenName: 'Ada',
	familyName: 'Byron',
	preferredName: '',
	email: 'ada.byron@example.com',
	phone: '',
	addressLine1: '',
	addressLine2: '',
	addressLocality: '',
	addressRegion: '',
	addressPostalCode: '',
	dateOfBirth: '1815-12-10',
	wouldSurvive: false,
	engagements: []
};

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	editMergeDraft.clear();
});

async function setup(overrides: Partial<ClientDetail> = {}) {
	const detail = { ...baseDetail, ...overrides };
	apiFetchWithSession.mockResolvedValueOnce(jsonResponse(detail));
	await render(Page, {});
	await expect.element(testPage.getByLabelText('Given name')).toHaveValue(detail.givenName);
	return { detail };
}

function requestBody(callIndex: number): { override: boolean; dateOfBirth: string } {
	const init = apiFetchWithSession.mock.calls[callIndex][1] as RequestInit;
	return JSON.parse(init.body as string) as { override: boolean; dateOfBirth: string };
}

describe('client edit', () => {
	it('pre-fills the twelve structural columns from her current record', async () => {
		await setup();

		await expect.element(testPage.getByLabelText('Given name')).toHaveValue(baseDetail.givenName);
		await expect.element(testPage.getByLabelText('Family name')).toHaveValue(baseDetail.familyName);
		// The fixture's Client carries no preferred name (#537's vocabulary
		// keeps the free-text fields hostile, not every field non-blank), so
		// this asserts the honest blank rather than a value invented for a
		// tidier assertion.
		await expect.element(testPage.getByLabelText('Preferred name')).toHaveValue(baseDetail.preferredName);
		await expect.element(testPage.getByLabelText('Email')).toHaveValue(baseDetail.email);
		await expect.element(testPage.getByLabelText('Phone')).toHaveValue(baseDetail.phone);
		await expect.element(testPage.getByLabelText('Address line 1')).toHaveValue(baseDetail.addressLine1);
		await expect.element(testPage.getByLabelText('Town or city')).toHaveValue(baseDetail.addressLocality);
		await expect.element(testPage.getByLabelText('State')).toHaveValue(baseDetail.addressRegion);
		await expect.element(testPage.getByLabelText('Postal code')).toHaveValue(baseDetail.addressPostalCode);
		// A memorable date, in three boxes (#807), seeded from the stored
		// "YYYY-MM-DD" rather than shown as one.
		await expect.element(testPage.getByLabelText('Month')).toHaveValue('03');
		await expect.element(testPage.getByLabelText('Day')).toHaveValue('01');
		await expect.element(testPage.getByLabelText('Year')).toHaveValue('1994');
	});

	it('shows an error notice when the Client fails to load', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('client not found', 404));

		await render(Page, {});

		await expect.element(testPage.getByText('client not found')).toBeVisible();
	});

	it('redirects to the survivor rather than rendering a tombstoned record', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...baseDetail, mergedInto: 'client-9' }));

		await render(Page, {});

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith(`/practices/${practiceId}/clients/client-9`);
	});

	it("refuses to save with a blank given name, client-side, before any request", async () => {
		await setup();

		await testPage.getByLabelText('Given name').fill('');
		await testPage.getByRole('button', { name: 'Save' }).click();

		// ErrorSummary renders this refusal as a link to the field it names.
		await expect
			.element(testPage.getByRole('link', { name: "Enter the Client's given name" }))
			.toBeVisible();
		// The load is the only request made -- the refusal never reached the network.
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	/*
	 * #807: a birth date is a memorable date here too, so it is three
	 * boxes, composed on the way to the wire with intake's own tolerance
	 * -- one or two digits for the month, two or four for the year.
	 */
	it('composes the three boxes into the stored "YYYY-MM-DD" on save', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByLabelText('Month').fill('3');
		await testPage.getByLabelText('Day').fill('12');
		await testPage.getByLabelText('Year').fill('88');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(requestBody(1).dateOfBirth).toBe('1988-03-12');
	});

	it('refuses a date that is not one, before any request, linking to the box that has to change', async () => {
		await setup();

		await testPage.getByLabelText('Month').fill('2');
		await testPage.getByLabelText('Day').fill('30');
		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect
			.element(testPage.getByRole('link', { name: 'Date of birth must be a real date' }))
			.toHaveAttribute('href', '#client-edit-date-of-birth-day');
		const group = testPage.getByRole('group', { name: 'Date of birth' });
		await expect.element(group.getByRole('alert')).toHaveTextContent('Date of birth must be a real date');
		// The load is the only request made -- the refusal never reached the network.
		expect(apiFetchWithSession).toHaveBeenCalledTimes(1);
	});

	it('refuses a save that exactly matches a different Client (gate one), naming the match, before writing anything', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true }, 409)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await expect.element(testPage.getByText('Ada Byron', { exact: false })).toBeVisible();
		expect(requestBody(1).override).toBe(false);
		expect(goto).not.toHaveBeenCalled();
	});

	it('saves after the deliberate override, retrying with override: true', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true }, 409)
		);
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();

		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(requestBody(2).override).toBe(true);
		expect(goto).toHaveBeenCalledWith(detailHref);
	});

	/*
	 * The two halves of #1082's decision. Both need an assertion
	 * `toBeVisible()` cannot make: the dialog is a native `<dialog>` held
	 * open by `showModal()`, so it sits in the top layer above a
	 * `::backdrop` and the whole page behind it is inert. A refusal
	 * rendered in the page while the dialog is open still passes
	 * `toBeVisible()` and is unreadable, and `ErrorSummary`'s focus effect
	 * fires against inert content and does nothing. So the refusal is
	 * queried *through* the dialog rather than through the page, which is
	 * the assertion #804 used for the same defect, and the focus half is
	 * read off `document.activeElement`. Each one fails under the
	 * arrangement this ticket replaces.
	 */
	it('keeps a refused override that names no field readable inside the still-open dialog', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true }, 409)
		);
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ message: 'This Practice is not accepting changes.' }, 403)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		// Scoped to the dialog, so this passes only while the refusal is in
		// the top layer with it -- the same query that would have failed
		// with the refusal rendered in the page behind the backdrop.
		const dialog = testPage.getByRole('dialog');
		await expect.element(dialog).toBeVisible();
		await expect
			.element(dialog.getByText('This Practice is not accepting changes.'))
			.toBeVisible();
		// And nothing is left waiting in the page's own summary to appear
		// unannounced the moment she cancels.
		await expect.element(testPage.getByText('There is a problem')).not.toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});

	it('shows every reason when a refused override names fields this form does not map', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true }, 409)
		);
		// `details` keyed on two columns this form has no control for, which
		// is what a BFF refusal naming a field the form has not caught up
		// with looks like. Both entries come back untargeted, and neither
		// may be the one that disappears.
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					message: 'The Client record could not be saved.',
					details: {
						pronouns: 'Enter pronouns of 50 characters or fewer',
						dueDate: 'The due date must be a real date'
					}
				},
				400
			)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		const dialog = testPage.getByRole('dialog');
		await expect
			.element(dialog.getByText('Enter pronouns of 50 characters or fewer', { exact: false }))
			.toBeVisible();
		await expect
			.element(dialog.getByText('The due date must be a real date', { exact: false }))
			.toBeVisible();
	});

	it('hands a refused override that names a field back to the form, with focus that lands', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse({ matches: [anotherClientMatch], substitution: true }, 409)
		);
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{
					message: 'The Client record could not be saved.',
					details: { givenName: 'Enter a given name of 100 characters or fewer' }
				},
				400
			)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();
		await expect.element(testPage.getByRole('dialog')).toBeVisible();
		await testPage.getByRole('button', { name: 'Yes, a different person' }).click();

		// The fix is on the form, so the dialog gets out of the way: the
		// page stops being inert, its summary's own focus effect can reach
		// it, and the entry's fragment link can follow. Under the old
		// arrangement the dialog stayed open and `document.activeElement`
		// was still the confirm button.
		await expect
			.element(testPage.getByRole('link', { name: 'Enter a given name of 100 characters or fewer' }))
			.toBeVisible();
		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		// `document.activeElement` rather than a locator's `toHaveFocus()`:
		// what takes focus is `ErrorSummary`'s own `tabindex="-1"` wrapper,
		// which carries no role on purpose so it does not double up on the
		// `role="alert"` inside it -- the second `querySelector` exception
		// in `.claude/rules/svelte-tests.md`, a deliberately non-accessible
		// element with nothing for an accessible query to find.
		await expect
			.poll(() => document.activeElement?.textContent)
			.toContain('Enter a given name of 100 characters or fewer');
		expect(goto).not.toHaveBeenCalled();
	});

	it('sends a possible duplicate (gate two) to its own question page rather than a dialog', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(
			jsonResponse(
				{ matches: [{ ...anotherClientMatch, wouldSurvive: true }], substitution: false },
				409
			)
		);

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByRole('dialog')).not.toBeInTheDocument();
		expect(goto).toHaveBeenCalledWith(editDuplicateHref);
		expect(editMergeDraft.clientId).toBe(clientId);
		expect(editMergeDraft.matches).toEqual([{ ...anotherClientMatch, wouldSurvive: true }]);
		expect(editMergeDraft.fields.givenName).toBe(baseDetail.givenName);
	});

	it('surfaces the endpoint refusal as an error rather than a silent failure', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('client not found', 404));

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.element(testPage.getByText('client not found')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});

	it('returns to the Client detail hub after a successful save', async () => {
		await setup();
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse(baseDetail));

		await testPage.getByRole('button', { name: 'Save' }).click();

		await expect.poll(() => goto.mock.calls.length).toBeGreaterThan(0);
		expect(goto).toHaveBeenCalledWith(detailHref);
	});

	it('shows that a changed email revokes any pending portal invite', async () => {
		await setup();

		await expect
			.element(testPage.getByText('revokes any pending portal invite', { exact: false }))
			.not.toBeInTheDocument();

		await testPage.getByLabelText('Email').fill('new@example.com');

		await expect
			.element(testPage.getByText('revokes any pending portal invite', { exact: false }))
			.toBeVisible();
	});

	it('exposes Save and Cancel as reachable, labeled controls', async () => {
		await setup();

		await expect.element(testPage.getByRole('button', { name: 'Save' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Cancel' }))
			.toHaveAttribute('href', detailHref);
	});
});
