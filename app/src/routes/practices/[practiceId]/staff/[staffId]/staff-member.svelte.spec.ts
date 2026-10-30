import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { findDuplicateIds } from '#lib/duplicateIds.js';
import { gotoWithOutcome } from '#lib/outcome.js';
import { captureLanding } from '#lib/testOutcome.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// RecordDetail and DataTable lay out through the primitives and the
// tokens, which the real app loads in its root layout.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../../routeFixture.js';
import { asContractor, fixture, respond } from './page.fixture.js';
import { roster } from '../page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The page, the roster and both histories come from the fixture (#596).
 * This spec owns what is not the happy path -- a read that fails, a
 * second page, an act that is refused -- each written as a departure from
 * the fixture at the test that needs it.
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
// The real `apiErrorMessage`, from its leaf module, for the reason the
// roster's own spec gives.
vi.mock('#lib/api.js', async () => ({
	apiFetchWithSession,
	...(await vi.importActual<typeof import('#lib/apiErrorMessage.js')>('#lib/apiErrorMessage.js'))
}));

function refusal(message: string, status = 409): Response {
	return jsonResponse({ code: 'FAILED_PRECONDITION', message }, status);
}

const [owner, contractor] = roster.members;
const { session } = fixture.pageData as { session: Record<string, unknown> };

/**
 * A departure from the fixture: the answer to one request, or undefined
 * to let the fixture answer it.
 */
type Answer = (path: string, init?: RequestInit) => Response | undefined;

interface Options {
	answer?: Answer;
	asAdmin?: boolean;
	variant?: typeof asContractor;
}

function isRosterRead(path: string): boolean {
	return path.endsWith('/staff');
}

function deleteOf(segment: string): (path: string, init?: RequestInit) => boolean {
	return (path, init) => path.endsWith(segment) && init?.method === 'DELETE';
}

const isEndSessions = deleteOf('/sessions');
const isRemove = deleteOf('/membership');

// No departure: the fixture answers every request.
function fromFixture(): undefined {}

async function setup({ answer = fromFixture, asAdmin = false, variant }: Options = {}) {
	Object.assign(pageState, toPageState(variant ? { ...fixture, ...variant } : fixture));
	if (asAdmin) pageState.data = { session: { ...session, roles: ['admin'] } };
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) =>
		Promise.resolve(answer(path, init) ?? (init?.method === 'DELETE' ? jsonResponse({}) : respond(path)))
	);
	await testPage.viewport(1440, 900);
	await render(Page, {});
}

function requests(piece: string): unknown[][] {
	return apiFetchWithSession.mock.calls.filter((call: unknown[]) => String(call[0]).includes(piece));
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
});

describe("a Staff member's page", () => {
	it('names the person, links back to the roster, and shows the facts', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { level: 1, name: owner!.name })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: 'Staff' }).first()).toHaveAttribute('href', '/practices/practice-1/staff');
		await expect.element(testPage.getByText(owner!.email)).toBeVisible();
		await expect.element(testPage.getByText('Owner, Admin', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText('Employee', { exact: true })).toBeVisible();
		await expect.element(testPage.getByText(/^New York, self-reported /)).toBeVisible();
	});

	// GOV.UK's Summary list row action: one Change per fact an Owner may
	// change, each to its own question page, never a form in the row.
	it('offers an Owner one Change for roles and one for employment type, each to its own question page', async () => {
		await setup();

		const changes = testPage.getByRole('link', { name: 'Change' });
		await expect.element(changes.first()).toHaveAttribute('href', `/practices/practice-1/staff/${owner!.staffId}/roles`);
		await expect.element(changes.nth(1)).toHaveAttribute('href', `/practices/practice-1/staff/${owner!.staffId}/employment-type`);
		expect(changes.elements()).toHaveLength(2);
		expect(testPage.getByRole('group').elements()).toHaveLength(0);
	});

	it('offers an Owner the way in to vouching', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: 'Send a recovery code' }))
			.toHaveAttribute('href', `/practices/practice-1/staff/${owner!.staffId}/mfa-recovery`);
	});

	// The endpoints are Owner-only (staffauth/mount.go): an Admin reads the
	// facts and the histories, and is offered nothing that would 403.
	it('offers an Admin the facts and the histories, and none of the Owner-only acts', async () => {
		await setup({ asAdmin: true });

		await expect.element(testPage.getByRole('heading', { name: 'Membership history' })).toBeVisible();
		await expect.element(testPage.getByText('Changed from District of Columbia to New York').first()).toBeVisible();
		expect(testPage.getByRole('link', { name: 'Change' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('link', { name: 'Send a recovery code' }).elements()).toHaveLength(0);
		expect(testPage.getByRole('heading', { name: 'Access' }).elements()).toHaveLength(0);
	});

	it('says when the roster no longer holds the person', async () => {
		await setup({ answer: (path) => (isRosterRead(path) ? jsonResponse({ ...roster, members: [contractor] }) : undefined) });

		await expect.element(testPage.getByText('That Staff member is not on this practice roster.')).toBeVisible();
		// Never just "— DoulaCloud", which is what a move here announced (#2246).
		expect(document.title).toBe('Staff member — DoulaCloud');
	});

	it('says why the roster could not be read', async () => {
		await setup({
			answer: (path) => (isRosterRead(path) ? refusal('Server rejected the Staff list request', 403) : undefined)
		});

		await expect.element(testPage.getByText('Server rejected the Staff list request')).toBeVisible();
	});

	it('says what the change that led here did, before the title', async () => {
		Object.assign(pageState, toPageState(fixture));
		goto.mockImplementationOnce(async () => {
			await setup();
		});

		await gotoWithOutcome(fixture.url, `${owner!.name}'s roles are now Owner, Admin.`);

		const message = testPage.getByRole('status').filter({ hasText: 'roles are now Owner, Admin.' });
		await expect.element(message).toBeVisible();
		await expect.element(message).toHaveFocus();
	});

	// #666: DataTable draws each history twice, once per tree.
	it('emits no duplicate id', async () => {
		await setup();
		await expect.element(testPage.getByText('Joined as Owner, Admin, Doula (Employee)').first()).toBeVisible();

		expect(document.querySelectorAll('[id]').length).toBeGreaterThan(0);
		expect(findDuplicateIds(document)).toEqual([]);
	});
});

/*
 * The two histories, read with the page rather than behind a disclosure
 * (#459, #872). DataTable draws every row into both of its trees, so text
 * inside one is matched with `.first()`: the wide viewport shows the
 * <table>, which comes first.
 */
describe("a Staff member's histories", () => {
	it('reads both histories with the page, once each', async () => {
		await setup();

		await expect.element(testPage.getByText('Changed from District of Columbia to New York').first()).toBeVisible();
		await expect.element(testPage.getByText('Employment type changed from Employee to Contractor').first()).toBeVisible();
		expect(requests('/work-state-history')).toHaveLength(1);
		expect(requests('/membership-history')).toHaveLength(1);
	});

	// A first assertion has no previous value (migration 00043 leaves it
	// NULL), and printing it as a change would invent one never made.
	it('prints a first assertion as a report, and marks one made before joining this practice', async () => {
		await setup();

		await expect.element(testPage.getByText('Reported North Carolina (before joining this practice)').first()).toBeVisible();
		await expect.element(testPage.getByText(/^Changed from North Carolina to District of Columbia$/).first()).toBeVisible();
	});

	it('names who made each Membership change, which is half of what the audit trail is for', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: 'Renata Alvarez' }).first()).toBeVisible();
	});

	it('shows older entries when asked, after the ones already there', async () => {
		await setup({
			answer: (path) =>
				path.includes('/membership-history?cursor=')
					? jsonResponse({
							items: [
								{
									eventId: 'membership-event-0',
									action: 'removed',
									actorName: 'Renata Alvarez',
									previousRoles: ['doula'],
									previousEmploymentType: 'employee',
									createdAt: '2026-07-01T00:00:00Z'
								}
							],
							hasMore: false
						})
					: undefined
		});

		await expect.element(testPage.getByText('Joined as Owner, Admin, Doula (Employee)').first()).toBeVisible();
		// The work state history has more too, so the second Load more is
		// the Membership history's.
		await testPage.getByRole('button', { name: 'Load more' }).nth(1).click();

		await vi.waitFor(() => expect(requests('/membership-history')).toHaveLength(2));
		await expect.element(testPage.getByText('Joined as Owner, Admin, Doula (Employee)').first()).toBeVisible();
	});

	it("says when a history could not be read, without taking the page's other content away", async () => {
		await setup({
			answer: (path) => (path.includes('/work-state-history') ? refusal('Failed to load work state history', 500) : undefined)
		});

		await expect.element(testPage.getByText('Failed to load work state history')).toBeVisible();
		await expect.element(testPage.getByText('Joined as Owner, Admin, Doula (Employee)').first()).toBeVisible();
	});

	it('says when the Membership history could not be read', async () => {
		await setup({
			answer: (path) => (path.includes('/membership-history') ? refusal('Failed to load membership history', 500) : undefined)
		});

		await expect.element(testPage.getByText('Failed to load membership history')).toBeVisible();
	});

	it('says when nothing is recorded', async () => {
		await setup({
			answer: (path) =>
				path.includes('/membership-history') ? jsonResponse({ items: [], hasMore: false }) : undefined
		});

		await expect.element(testPage.getByText('Nothing recorded.')).toBeVisible();
	});

	it("draws a contractor's page with no roles yet", async () => {
		await setup({ variant: asContractor });

		await expect.element(testPage.getByRole('heading', { level: 1, name: contractor!.name })).toBeVisible();
		await expect.element(testPage.getByText('No roles yet')).toBeVisible();
		await expect.element(testPage.getByText('Reported California (before joining this practice)').first()).toBeVisible();
	});
});

/*
 * The Access section: the two acts that end something, last on the page,
 * apart from each other, each behind its own ConfirmDialog.
 */
describe("a Staff member's Access section", () => {
	it('states what each act does before its button', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Access' })).toBeVisible();
		await expect.element(testPage.getByText(`${owner!.name} is signed out on every device at once.`, { exact: false })).toBeVisible();
		await expect
			.element(testPage.getByText(`${owner!.name} loses access to this Practice's Clients immediately. The account`, { exact: false }))
			.toBeVisible();
	});

	it('ends every session the person holds, and says so', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'End sessions everywhere' }).click();
		await testPage.getByRole('dialog').getByRole('button', { name: 'End sessions everywhere' }).click();

		await expect.element(testPage.getByText('Sessions ended.')).toBeVisible();
		expect(requests('/sessions')).toHaveLength(1);
	});

	it('keeps the dialog open with the reason when ending sessions is refused', async () => {
		await setup({ answer: (path, init) => (isEndSessions(path, init) ? refusal('only a Practice Owner can do that', 403) : undefined) });

		await testPage.getByRole('button', { name: 'End sessions everywhere' }).click();
		const dialog = testPage.getByRole('dialog');
		await dialog.getByRole('button', { name: 'End sessions everywhere' }).click();

		await expect.element(dialog.getByText('only a Practice Owner can do that')).toBeVisible();
	});

	it('removes the person and lands on the roster, which says so', async () => {
		await setup();
		const landing = captureLanding(goto);

		await testPage.getByRole('button', { name: 'Remove from this Practice' }).click();
		await testPage.getByRole('dialog').getByRole('button', { name: 'Remove from this Practice' }).click();

		await vi.waitFor(() => expect(landing.href).toBe('/practices/practice-1/staff'));
		expect(landing.message).toBe(`${owner!.name} was removed from this Practice.`);
	});

	it('keeps the dialog open with the reason when removing is refused', async () => {
		await setup({ answer: (path, init) => (isRemove(path, init) ? refusal('a practice must keep at least one Owner') : undefined) });

		await testPage.getByRole('button', { name: 'Remove from this Practice' }).click();
		const dialog = testPage.getByRole('dialog');
		await dialog.getByRole('button', { name: 'Remove from this Practice' }).click();

		await expect.element(dialog.getByText('a practice must keep at least one Owner')).toBeVisible();
		expect(goto).not.toHaveBeenCalled();
	});
});
