/*
 * #1093's section on a birth's own page: who is on call for it, each
 * doula's own days, and the times somebody cannot be reached. The
 * roster answers "who is on call tonight" across the Practice; this
 * answers it for one birth, and is where a person changes it.
 */
import type { ComponentProps } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { jsonResponse } from '#lib/testResponse.js';
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { asDoula, detail as fixtureDetail, fixture, onCallPanel, session } from './page.fixture.js';

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
	apiBaseURL: () => ''
}));

if (!customElements.get('center-l')) registerLayoutPrimitives();

const gapsPath = '/api/practices/practice-1/engagements/engagement-1/coverage-gaps';

beforeEach(() => {
	apiFetchWithSession.mockReset();
});

/**
 * Renders the page with the fixture answering every read, and `extra`
 * answering the writes this section makes on top of them.
 */
async function setupOnCall(
	extra: (path: string, init?: RequestInit) => Response | undefined = () => {},
	data?: ComponentProps<typeof Page>['data']
) {
	await testPage.viewport(1440, 900);
	const respond = toApiResponder(fixture);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		const answer = extra(path, init);
		return answer ? Promise.resolve(answer) : respond(path);
	});
	await render(Page, { data: data ?? { ...fixtureDetail, session }, params: fixture.params });
}

// The summary card above has Changes of its own, so the section's are
// asked for inside its region.
const onCallChanges = () =>
	testPage.getByRole('region', { name: 'On call' }).getByRole('button', { name: 'Change', exact: true });
const recordGapOpener = () => testPage.getByRole('button', { name: 'Record a time a doula cannot be reached' });

/**
 * The page as setupOnCall renders it, with the gap form opened (#2042).
 */
async function setupGapForm(extra?: (path: string, init?: RequestInit) => Response | undefined) {
	await setupOnCall(extra);
	await recordGapOpener().click();
}

const gapStaffField = () => testPage.getByRole('combobox', { name: 'Doula who cannot be reached' });
const coveringField = () => testPage.getByRole('combobox', { name: 'Who is covering' });
const optionNames = (select: ReturnType<typeof testPage.getByRole>) =>
	select
		.getByRole('option')
		.elements()
		.map((option) => option.textContent.trim());
// The error summary's links, as [words, href], in the order it lists them.
const summaryLinks = () =>
	testPage
		.getByRole('alert')
		.getByRole('link')
		.elements()
		.map((link) => [link.textContent.trim(), link.getAttribute('href')]);

describe('the on-call section on a birth (#1093)', () => {
	it('says when the window runs, and who is on call for which part of it', async () => {
		await setupOnCall();

		await expect.element(testPage.getByText(/On call from .* to /)).toBeVisible();
		await expect.element(testPage.getByText(/On call Oct 31, 2026 to Nov 13, 2026/)).toBeVisible();
	});

	it('shows a gap nobody is covering as the hole it is, beside one somebody is covering', async () => {
		await setupOnCall();

		await expect.element(testPage.getByText('Nobody covering')).toBeVisible();
		await expect
			.element(testPage.getByText('Persephone Vandermeulen-Achterberg, CD(DONA) covering'))
			.toBeVisible();
	});

	it('records a gap for a doula on the birth, and reads the section back', async () => {
		const posted: RequestInit[] = [];
		await setupGapForm((path, init) => {
			if (path === gapsPath && init?.method === 'POST') {
				posted.push(init);
				return jsonResponse({ id: 'gap-3' }, 201);
			}
			return;
		});

		await gapStaffField().selectOptions('Bo Ng');
		await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
		await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
		await testPage.getByLabelText('Reason', { exact: true }).fill('Away overnight');
		await testPage.getByRole('button', { name: 'Record the gap' }).click();

		await vi.waitFor(() => expect(posted).toHaveLength(1));
		const body = JSON.parse(String(posted[0]!.body));
		expect(body.staffId).toBe(onCallPanel.doulas[1]!.staffId);
		expect(body.reason).toBe('Away overnight');
		expect(body.coveringStaffId).toBeUndefined();
		// Back to the rows with what it did, and blank again for the next
		// gap, with nobody chosen.
		await expect.element(testPage.getByText('The time Bo Ng cannot be reached is recorded.')).toBeVisible();
		await recordGapOpener().click();
		await expect.element(gapStaffField()).toHaveValue('');
	});

	// #1916, ADR-0055: which Doula cannot be reached is a fact about
	// someone else, and every Owner and Admin is told of the gap, so the
	// question opens with nobody chosen.
	describe('Doula who cannot be reached', () => {
		it('opens with nobody chosen', async () => {
			await setupGapForm();

			await expect.element(gapStaffField()).toHaveValue('');
			await expect.element(gapStaffField().getByRole('option', { name: 'Choose a Doula' })).toBeDisabled();
		});

		it('refuses a gap with no Doula chosen, in the summary and against the field, and sends nothing', async () => {
			const posted: RequestInit[] = [];
			await setupGapForm((path, init) => {
				if (path === gapsPath && init?.method === 'POST') {
					posted.push(init);
					return jsonResponse({ id: 'gap-3' }, 201);
				}
				return;
			});

			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect
				.element(testPage.getByRole('link', { name: 'Select the Doula who cannot be reached' }))
				.toHaveAttribute('href', '#coverage-gap-staff');
			await expect.element(gapStaffField()).toHaveAttribute('aria-invalid', 'true');
			await expect.element(testPage.getByText('Select the Doula who cannot be reached').last()).toBeVisible();
			expect(posted).toHaveLength(0);
		});

		it('puts the BFF refusal of the Doula against the field', async () => {
			await setupGapForm((path, init) =>
				path === gapsPath && init?.method === 'POST'
					? jsonResponse(
							{
								code: 'invalid_argument',
								message: 'The coverage gap could not be saved.',
								details: { staffId: 'Choose a doula who is on this birth.' }
							},
							400
						)
					: undefined
			);

			await gapStaffField().selectOptions('Bo Ng');
			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect
				.element(testPage.getByRole('link', { name: 'Choose a doula who is on this birth.' }))
				.toHaveAttribute('href', '#coverage-gap-staff');
		});

		it('lists every on-call Doula under Who is covering while none is chosen, and leaves out the one chosen', async () => {
			await setupGapForm();

			// An option in a closed <select> is not drawn, so it is counted
			// rather than asked whether it is visible.
			await expect.element(coveringField()).toBeVisible();
			expect(optionNames(coveringField())).toEqual([
				'Nobody yet',
				'Persephone Vandermeulen-Achterberg, CD(DONA)',
				'Bo Ng'
			]);

			await gapStaffField().selectOptions('Bo Ng');

			await vi.waitFor(() =>
				expect(optionNames(coveringField())).toEqual(['Nobody yet', 'Persephone Vandermeulen-Achterberg, CD(DONA)'])
			);
		});
	});

	// #1969: an empty From or Until reached `new Date('').toISOString()`,
	// and the browser's own "Invalid time value" was the gap's error.
	describe('From and Until', () => {
		it('refuses every empty question in one summary, in field order, against each field, and sends nothing', async () => {
			const posted: RequestInit[] = [];
			await setupGapForm((path, init) => {
				if (path === gapsPath && init?.method === 'POST') {
					posted.push(init);
					return jsonResponse({ id: 'gap-3' }, 201);
				}
				return;
			});

			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect.element(testPage.getByRole('link', { name: 'Enter when the gap starts' })).toBeVisible();
			expect(summaryLinks()).toEqual([
				['Select the Doula who cannot be reached', '#coverage-gap-staff'],
				['Enter when the gap starts', '#coverage-gap-starts'],
				['Enter when the gap ends', '#coverage-gap-ends']
			]);
			await expect
				.element(testPage.getByLabelText('From', { exact: true }))
				.toHaveAttribute('aria-invalid', 'true');
			await expect
				.element(testPage.getByLabelText('Until', { exact: true }))
				.toHaveAttribute('aria-invalid', 'true');
			await expect.element(testPage.getByText('Enter when the gap ends').last()).toBeVisible();
			expect(testPage.getByText('Invalid time value').query()).toBeNull();
			expect(posted).toHaveLength(0);
		});

		it('refuses an empty Until alone, with the Doula and From answered', async () => {
			await setupGapForm();

			await gapStaffField().selectOptions('Bo Ng');
			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect.element(testPage.getByRole('link', { name: 'Enter when the gap ends' })).toBeVisible();
			expect(summaryLinks()).toEqual([['Enter when the gap ends', '#coverage-gap-ends']]);
			expect(testPage.getByText('Invalid time value').query()).toBeNull();
		});

		it('refuses an Until that is not after From, against Until, and sends nothing', async () => {
			const posted: RequestInit[] = [];
			await setupGapForm((path, init) => {
				if (path === gapsPath && init?.method === 'POST') {
					posted.push(init);
					return jsonResponse({ id: 'gap-3' }, 201);
				}
				return;
			});

			await gapStaffField().selectOptions('Bo Ng');
			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-21T08:00');
			await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect
				.element(testPage.getByRole('link', { name: 'The gap must end after it starts' }))
				.toHaveAttribute('href', '#coverage-gap-ends');
			await expect
				.element(testPage.getByLabelText('Until', { exact: true }))
				.toHaveAttribute('aria-invalid', 'true');
			expect(posted).toHaveLength(0);
		});

		it('puts each field the BFF refuses against its own field, in field order', async () => {
			await setupGapForm((path, init) =>
				path === gapsPath && init?.method === 'POST'
					? jsonResponse(
							{
								code: 'invalid_argument',
								message: 'The coverage gap could not be saved.',
								// Go writes a map's keys sorted, which is not the page's order.
								details: {
									endsAt: 'The gap must end after it starts',
									reason: 'Keep the reason to 500 characters or fewer.',
									startsAt: "The gap must fall inside this birth's on-call window."
								}
							},
							400
						)
					: undefined
			);

			await gapStaffField().selectOptions('Bo Ng');
			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect.element(testPage.getByRole('link', { name: 'The gap must end after it starts' })).toBeVisible();
			expect(summaryLinks()).toEqual([
				["The gap must fall inside this birth's on-call window.", '#coverage-gap-starts'],
				['The gap must end after it starts', '#coverage-gap-ends'],
				['Keep the reason to 500 characters or fewer.', '#coverage-gap-reason']
			]);
		});

		it('says a refusal that names no field in the summary, with no link', async () => {
			await setupGapForm((path, init) =>
				path === gapsPath && init?.method === 'POST'
					? jsonResponse(
							{
								code: 'failed_precondition',
								message: 'Care has ended on this birth, so there is no on-call window to record a gap in.'
							},
							409
						)
					: undefined
			);

			await gapStaffField().selectOptions('Bo Ng');
			await testPage.getByLabelText('From', { exact: true }).fill('2026-10-20T22:00');
			await testPage.getByLabelText('Until', { exact: true }).fill('2026-10-21T08:00');
			await testPage.getByRole('button', { name: 'Record the gap' }).click();

			await expect
				.element(
					testPage
						.getByRole('alert')
						.getByText('Care has ended on this birth, so there is no on-call window to record a gap in.')
				)
				.toBeVisible();
			expect(summaryLinks()).toEqual([]);
		});
	});

	it('clears a gap when the doula can be reached again', async () => {
		const cleared: string[] = [];
		await setupOnCall((path, init) => {
			if (init?.method === 'DELETE') {
				cleared.push(path);
				return jsonResponse(undefined, 204);
			}
			return;
		});

		// Each Clear is described by whose gap it ends; the fixture's first
		// gap is Persephone's.
		const clear = testPage.getByRole('button', { name: 'Clear', exact: true }).first();
		await expect.element(clear).toHaveAccessibleDescription(/the gap for Persephone/);
		await clear.click();

		await vi.waitFor(() => expect(cleared).toEqual([`${gapsPath}/gap-1`]));
		await expect
			.element(testPage.getByText('The gap for Persephone Vandermeulen-Achterberg, CD(DONA) is cleared.'))
			.toBeVisible();
	});

	it('says why a gap was not cleared', async () => {
		await setupOnCall((path, init) =>
			init?.method === 'DELETE'
				? jsonResponse({ code: 'permission_denied', message: 'You cannot clear this gap.' }, 403)
				: undefined
		);

		await testPage.getByRole('button', { name: 'Clear', exact: true }).first().click();

		await expect.element(testPage.getByText('You cannot clear this gap.')).toBeVisible();
	});

	// #2042: the section is rows with one action each, and a form only
	// when asked, so it never holds more than one primary.
	describe('one action per row', () => {
		it('opens with no form and no primary, only the rows and their actions', async () => {
			await setupOnCall();

			await expect.element(recordGapOpener()).toBeVisible();
			expect(testPage.getByRole('button', { name: 'Record the gap' }).query()).toBeNull();
			expect(testPage.getByLabelText('First day on call').query()).toBeNull();
			await expect
				.element(onCallChanges().first())
				.toHaveAccessibleDescription(/on-call days for Persephone/);
		});

		it('opens one Doula’s two dates under her row, and closes them again', async () => {
			await setupOnCall();
			const change = onCallChanges().nth(1);

			await change.click();

			await expect.element(change).toHaveAttribute('aria-expanded', 'true');
			await expect.element(testPage.getByLabelText('First day on call')).toHaveValue('2026-10-31');
			await expect.element(testPage.getByLabelText('Last day on call')).toHaveValue('');

			await change.click();

			expect(testPage.getByLabelText('First day on call').query()).toBeNull();
		});

		it('closes the gap form when a Change opens, so one form is open at a time', async () => {
			await setupGapForm();

			await onCallChanges().first().click();

			expect(testPage.getByRole('button', { name: 'Record the gap' }).query()).toBeNull();
			await expect.element(recordGapOpener()).toBeVisible();
		});

		it('closes the gap form blank on Cancel', async () => {
			await setupGapForm();
			await gapStaffField().selectOptions('Bo Ng');

			await testPage.getByRole('button', { name: 'Cancel' }).click();
			await recordGapOpener().click();

			await expect.element(gapStaffField()).toHaveValue('');
		});

		it('offers the gap form from the empty state where nobody has recorded a gap', async () => {
			await setupOnCall((path) =>
				path.endsWith('/on-call') ? jsonResponse({ ...onCallPanel, gaps: [] }) : undefined
			);

			await expect
				.element(testPage.getByText('Nobody has recorded a time they cannot be reached.'))
				.toBeVisible();
			await recordGapOpener().click();

			await expect.element(gapStaffField()).toBeVisible();
		});
	});

	it('states one doula’s own on-call days, sends only hers, and says it is saved', async () => {
		const puts: { path: string; body: unknown }[] = [];
		await setupOnCall((path, init) => {
			if (path.includes('/attachments/') && init?.method === 'PUT') {
				puts.push({ path, body: JSON.parse(String(init.body)) });
				return jsonResponse({ from: '2026-10-20' });
			}
			return;
		});

		await onCallChanges().nth(1).click();
		await testPage.getByLabelText('First day on call').fill('2026-10-20');
		await testPage.getByRole('button', { name: 'Save on-call days' }).click();

		await vi.waitFor(() => expect(puts).toHaveLength(1));
		expect(puts[0]!.path).toContain('/attachments/staff-2/on-call');
		expect(puts[0]!.body).toEqual({ from: '2026-10-20', to: undefined });
		await expect.element(testPage.getByText('The on-call days for Bo Ng are saved.')).toBeVisible();
		expect(testPage.getByLabelText('First day on call').query()).toBeNull();
	});

	it('keeps the dates open beside the refusal when the days are not saved', async () => {
		await setupOnCall((path, init) =>
			path.includes('/attachments/') && init?.method === 'PUT'
				? jsonResponse({ code: 'invalid_argument', message: 'The first day must be inside the window.' }, 400)
				: undefined
		);

		await onCallChanges().nth(1).click();
		await testPage.getByRole('button', { name: 'Save on-call days' }).click();

		await expect.element(testPage.getByText('The first day must be inside the window.')).toBeVisible();
		await expect.element(testPage.getByLabelText('First day on call')).toBeVisible();
	});

	it('draws no Change for a reader who is not an Owner or an Admin', async () => {
		await setupOnCall(undefined, asDoula.props!.data as ComponentProps<typeof Page>['data']);

		await expect.element(testPage.getByText(/On call Oct 31, 2026 to Nov 13, 2026/)).toBeVisible();
		expect(onCallChanges().elements()).toHaveLength(0);
	});

	it('says plainly why a birth has no window, rather than drawing an empty one', async () => {
		await setupOnCall((path) =>
			path.endsWith('/on-call') && !path.includes('coverage')
				? jsonResponse({
						noWindowReason: 'no_due_date',
						rule: { startRule: 'gestational_week', startWeek: 37, graceDays: 14, overridden: false },
						doulas: [],
						gaps: []
					})
				: undefined
		);

		await expect
			.element(testPage.getByText('No due date recorded, so there is no window to work out.'))
			.toBeVisible();
	});

	it('reports a section it could not read, rather than rendering nothing', async () => {
		await setupOnCall((path) =>
			path.endsWith('/on-call') ? jsonResponse('engagement not found', 404) : undefined
		);

		await expect.element(testPage.getByText('engagement not found')).toBeVisible();
	});
});
