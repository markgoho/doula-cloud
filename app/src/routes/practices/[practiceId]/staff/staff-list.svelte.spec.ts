import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { findDuplicateIds } from '#lib/duplicateIds.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import { revealDisclosures } from '../../../style-guide/continuum.js';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture, membershipHistories, roster, workStateHistories } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
 *
 * The roster itself comes from the fixture too (#596). It holds two
 * Members and two pending Invitations because that is the roster a real
 * Practice has, which is also what this file needs: nearly every test
 * here targets a specific row among two-or-more (comparing
 * roles/employment side by side, revoking one Invitation while
 * asserting the other survives, `describedByText` against a second
 * row).
 */
const pageState = vi.hoisted(() => ({
	params: {} as Record<string, string>,
	url: new URL('https://example.test/'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
// The page no longer imports apiErrorMessage itself (#840) -- it calls
// #lib/staff.js, which reads a refusal through the real apiErrorMessage.
// The envelope shape below (`{code, message}`, docs/api-design.md
// section 7) is what that real reader expects; a bare JSON-encoded
// string would parse to a non-object and fall through to the quoted raw
// text instead of `message`.
// The real `apiErrorMessage` comes along because #872 puts
// `#lib/activityLedger.js` in this page's import graph -- the Membership
// history's fallback sentence for an action this build has no words for
// is that module's `describeActivityAction`, and the module reads a
// refusal through `#lib/api.js`'s re-export. It is imported from the leaf
// module it actually lives in rather than through `importOriginal`, which
// would drag the whole of `api.ts` (and Firebase with it) into a spec
// that exists to keep both out.
vi.mock('#lib/api.js', async () => ({
	apiFetchWithSession,
	...(await vi.importActual<typeof import('#lib/apiErrorMessage.js')>('#lib/apiErrorMessage.js'))
}));

function textResponse(message: string): Response {
	return jsonResponse({ code: 'FAILED_PRECONDITION', message }, 403);
}

const { members } = roster;
const invitations = roster.invitations.items;
const [ownerMember, contractorMember] = members;
const [liveInvitation, lapsedInvitation] = invitations;

// #459's work state history and #872's Membership history both come from
// the fixture beside this route now (#596, #1126): the check opens each
// disclosure in preparation and waits for what it loads, so the content
// the sweep measures and the content asserted on below are one object
// rather than two that drift. This spec still owns what is not the happy
// path -- a read that fails, a second page -- and each of those is written
// as a departure from the fixture at the test that needs it.

interface MockOptions {
	roster?: { members: typeof members; invitations: typeof invitations };
	listOk?: boolean;
	sessionsResponse?: Response;
	membershipResponse?: Response;
	revokeResponse?: Response;
	removeResponse?: Response;
	historyResponse?: Response;
	membershipHistoryResponse?: Response;
}

// The API double is stateful: a successful PATCH or revoke changes what
// the next roster read returns, so a test can assert on the screen the
// Owner ends up looking at rather than on the request that got her there.
function mockApi({
	roster = { members, invitations },
	listOk = true,
	sessionsResponse,
	membershipResponse,
	revokeResponse,
	removeResponse,
	historyResponse,
	membershipHistoryResponse
}: MockOptions = {}) {
	const state = structuredClone(roster);
	apiFetchWithSession.mockImplementation((path: string, init?: RequestInit) => {
		if (path.includes('/work-state-history')) {
			if (historyResponse) {
				return Promise.resolve(historyResponse);
			}
			const staffId = String(path.split('/').at(-2));
			return Promise.resolve(jsonResponse(workStateHistories[staffId]));
		}
		if (path.includes('/membership-history')) {
			if (membershipHistoryResponse) {
				return Promise.resolve(membershipHistoryResponse);
			}
			const staffId = String(path.split('/').at(-2));
			return Promise.resolve(jsonResponse(membershipHistories[staffId]));
		}
		if (path.endsWith('/sessions') && init?.method === 'DELETE') {
			return Promise.resolve(sessionsResponse ?? jsonResponse({}));
		}
		if (path.endsWith('/membership') && init?.method === 'DELETE') {
			if (removeResponse) {
				return Promise.resolve(removeResponse);
			}
			const staffId = path.split('/').at(-2);
			state.members = state.members.filter((member) => member.staffId !== staffId);
			return Promise.resolve(jsonResponse({}));
		}
		if (path.endsWith('/membership')) {
			if (membershipResponse) {
				return Promise.resolve(membershipResponse);
			}
			const staffId = path.split('/').at(-2);
			const patch = JSON.parse(String(init?.body));
			state.members = state.members.map((member) =>
				member.staffId === staffId
					? { ...member, roles: patch.roles, employmentType: patch.employmentType }
					: member
			);
			return Promise.resolve(jsonResponse({}));
		}
		if (path.endsWith('/revoke')) {
			if (revokeResponse) {
				return Promise.resolve(revokeResponse);
			}
			const invitationId = path.split('/').at(-2);
			state.invitations = state.invitations.filter(
				(invitation) => invitation.invitationId !== invitationId
			);
			return Promise.resolve(jsonResponse({}));
		}
		return Promise.resolve(
			listOk
				? jsonResponse({
						members: state.members,
						invitations: { items: state.invitations, hasMore: false }
					})
				: textResponse('Server rejected the Staff list request')
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	// #694 gave this screen a role-gated row action, so a test that
	// installs another session must not leave it installed -- the fixture's
	// own Owner is the default every other test here reads.
	Object.assign(pageState, toPageState(fixture));
});

/*
 * Scoped to a specific .table-view, not an unscoped getByText/getByRole:
 * DataTable renders rowActions.content once per tree (#508, ADR-0024), so
 * a Badge or Notice living inside a row's actions exists twice -- once in
 * the <table>, once in the record view's <dl> -- and getByText matches
 * DOM text regardless of which tree CSS hides. This is the same sanctioned
 * querySelector exception DataTable.svelte.spec.ts uses for the same
 * reason. The page renders Members before Pending invitations, so index
 * order is the same source of truth the DOM itself uses.
 */
function membersTable() {
	return testPage.elementLocator(document.querySelector('.table-view')!);
}

function invitationsTable() {
	return testPage.elementLocator(document.querySelectorAll('.table-view')[1]!);
}

async function setup(options: MockOptions = {}) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 46rem, and this file's assertions are about the <table> specifically.
	await testPage.viewport(1024, 800);
	mockApi(options);
	await render(Page, {});
	// The roster arrives from onMount, so `render` returning is not the
	// same as the screen being there. Several tests below read the DOM
	// synchronously -- describedByText and the two table helpers all call
	// .element()/querySelector rather than an awaited locator -- so the
	// wait belongs here rather than in each of them. Skipped when the load
	// is meant to fail, since then this heading never renders.
	if (options.listOk !== false) {
		await expect.element(testPage.getByRole('heading', { name: 'Pending invitations' })).toBeVisible();
	}
}

// "Revoke" (or Edit membership/End sessions everywhere/Remove from
// practice/Show older changes) alone doesn't say which member or invitation
// it acts on (#515); the distinguishing name is a sibling joined by
// aria-describedby, the same pattern the Edit link fix (#513) and
// CheckAnswers' Change links use, so no accessible query names it directly.
function describedByText(button: ReturnType<typeof testPage.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return document.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('staff screen', () => {
	// #508: the actual regression this ticket fixes, checked on the real
	// route rather than only on the style-guide demo that mirrors its
	// shape -- a document-level sweep would duplicate continuum.ts, so
	// this asserts the one number ADR-0024's 320px commitment cares
	// about, directly, the same way the ticket's own AC states it.
	it('never scrolls the document sideways at 320px (#508)', async () => {
		await setup();
		await testPage.viewport(320, 800);

		await expect.element(testPage.getByRole('heading', { name: 'Members' })).toBeVisible();
		expect(document.documentElement.scrollWidth).toBe(document.documentElement.clientWidth);
	});

	it('lists members with their roles and employment type', async () => {
		await setup();

		await expect.element(testPage.getByRole('heading', { name: 'Members' })).toBeVisible();
		// exact: true -- the Actions cell's own name now also contains the
		// member's name (#515's hidden siblings naming its per-row buttons).
		await expect
			.element(testPage.getByRole('cell', { name: ownerMember.name, exact: true }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: ownerMember.email })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Owner, Admin' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'no roles yet' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Contractor' }).first()).toBeVisible();
	});

	// #261: a pending invitation must be tellable apart from a member who
	// holds no roles -- which is why they are two groups, not one list.
	it('lists pending invitations as their own group, apart from the members', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: 'Pending invitations' }))
			.toBeVisible();
		// exact: true -- the Actions cell's own name now also contains the
		// address (#515's hidden sibling naming its Revoke button).
		await expect
			.element(testPage.getByRole('cell', { name: liveInvitation.address, exact: true }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'doula' })).toBeVisible();
		await expect
			.element(testPage.getByRole('button', { name: 'Revoke' }).first())
			.toBeVisible();
	});

	// #339's dead-lettered send has no other read surface.
	it('flags an invitation whose email could not be delivered', async () => {
		await setup();

		const flags = invitationsTable().getByText('Email could not be delivered');
		await expect.element(flags).toBeVisible();
		expect(flags.elements()).toHaveLength(1);
	});

	it('shows both empty messages when nobody is here and nobody is invited', async () => {
		await setup({ roster: { members: [], invitations: [] } });

		await expect.element(testPage.getByText('No Staff yet.')).toBeVisible();
		await expect.element(testPage.getByText('No pending invitations.')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	it('shows an error notice when the roster fails to load', async () => {
		await setup({ listOk: false });

		await expect.element(testPage.getByText('Server rejected the Staff list request')).toBeVisible();
		await expect.element(testPage.getByRole('table')).not.toBeInTheDocument();
	});

	// RA-G2 (#261): roles and employment type on one form, one change.
	it('edits a membership roles and employment type together', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).toBeVisible();
		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await testPage.getByRole('radio', { name: 'Contractor' }).click();
		await testPage.getByRole('button', { name: 'Save membership' }).click();

		// The form closes and the first member's row now reads the edited
		// Membership -- both halves, from one save.
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('cell', { name: 'Owner, Admin, Doula' }))
			.toBeVisible();
		const contractorCells = testPage.getByRole('cell', { name: 'Contractor' });
		await expect.element(contractorCells.first()).toBeVisible();
		expect(contractorCells.elements()).toHaveLength(3);
	});

	it('shows an error notice and keeps the form open when saving a membership fails', async () => {
		await setup({ membershipResponse: textResponse('a practice must keep at least one Owner') });

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await testPage.getByRole('button', { name: 'Save membership' }).click();

		await expect
			.element(membersTable().getByText('a practice must keep at least one Owner'))
			.toBeVisible();
		await expect.element(testPage.getByRole('group', { name: 'Roles' })).toBeVisible();
	});

	it('closes the edit form without changing the membership when canceled', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Edit membership' }).first().click();
		await testPage.getByRole('checkbox', { name: 'Doula' }).click();
		await testPage.getByRole('button', { name: 'Cancel' }).click();

		await expect.element(testPage.getByRole('group', { name: 'Roles' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('cell', { name: 'Owner, Admin' })).toBeVisible();
	});

	it('removes a revoked invitation from the pending group', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect
			.element(testPage.getByRole('cell', { name: liveInvitation.address }))
			.not.toBeInTheDocument();
		// exact: true -- the Actions cell's own name now also contains the
		// address (#515's hidden sibling naming Revoke).
		await expect
			.element(testPage.getByRole('cell', { name: lapsedInvitation.address, exact: true }))
			.toBeVisible();
	});

	// #291: a lapsed Invitation still holds its address slot, so it stays
	// on the screen with something the Owner can do about it.
	it('flags a lapsed invitation instead of hiding it', async () => {
		await setup();

		await expect
			.element(invitationsTable().getByText('Expired -- invite again or revoke'))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('cell', { name: lapsedInvitation.address, exact: true }))
			.toBeVisible();
	});

	// #291: the route that was missing -- without it a roster row nobody
	// wants can never be taken off.
	it('removes a membership from the practice', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Remove from practice' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Remove from Practice' });
		await dialog.getByRole('button', { name: 'Remove from Practice' }).click();

		await expect
			.element(testPage.getByRole('cell', { name: ownerMember.name }))
			.not.toBeInTheDocument();
		// exact: true -- the Actions cell's own name now also contains the
		// member's name (#515's hidden siblings naming its per-row buttons).
		await expect
			.element(testPage.getByRole('cell', { name: contractorMember.name, exact: true }))
			.toBeVisible();
	});

	it('shows a per-row error notice when removing a membership fails', async () => {
		await setup({ removeResponse: textResponse('a practice must keep at least one Owner') });

		await testPage.getByRole('button', { name: 'Remove from practice' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Remove from Practice' });
		await dialog.getByRole('button', { name: 'Remove from Practice' }).click();

		await expect
			.element(membersTable().getByText('a practice must keep at least one Owner'))
			.toBeVisible();
	});

	it('shows a per-row error notice when revoking fails', async () => {
		await setup({ revokeResponse: textResponse('no pending invitation found at this practice') });

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect
			.element(invitationsTable().getByText('no pending invitation found at this practice'))
			.toBeVisible();
	});

	it('ends sessions for a Staff member and shows a success notice for that row only', async () => {
		await setup();

		const buttons = testPage.getByRole('button', { name: 'End sessions everywhere' });
		await buttons.nth(1).click();
		const dialog = testPage.getByRole('dialog', { name: 'End sessions everywhere' });
		await dialog.getByRole('button', { name: 'End sessions everywhere' }).click();

		const successNotices = membersTable().getByText('Sessions ended.');
		await expect.element(successNotices).toBeVisible();
		expect(successNotices.elements()).toHaveLength(1);

		const rows = testPage.getByRole('row');
		await expect.element(rows.nth(1)).not.toHaveTextContent('Sessions ended.');
	});

	it('shows a per-row error notice when ending sessions fails', async () => {
		await setup({ sessionsResponse: textResponse('Failed to end sessions') });

		const buttons = testPage.getByRole('button', { name: 'End sessions everywhere' });
		await buttons.nth(0).click();
		const dialog = testPage.getByRole('dialog', { name: 'End sessions everywhere' });
		await dialog.getByRole('button', { name: 'End sessions everywhere' }).click();

		await expect.element(membersTable().getByText('Failed to end sessions')).toBeVisible();
	});

	// #459: the roster column shows the current value and the day it was
	// asserted, which answers "how did this get set?" only while the value
	// has never moved. These four hold the four things it had to decide.
	/*
	 * DataTable renders rowActions.content once per tree (#508, ADR-0024),
	 * so "Work state history"'s disclosure exists twice -- once in the
	 * <table> row, once in the record view's <dl>. A closed <details> is
	 * still in the DOM (only CSS-hidden), and getByText matches DOM text
	 * regardless of visibility, so a plain getByText for the disclosure's
	 * revealed content is ambiguous between the two copies. Scoping to
	 * .table-view -- the tree setup()'s wide viewport actually shows --
	 * is the same sanctioned querySelector exception DataTable.svelte.spec.ts
	 * uses for the same reason.
	 */
	describe('work state history', () => {
		it('is closed until it is opened, and then names both sides of a move', async () => {
			await setup();
			const tableView = membersTable();

			const disclosures = tableView.getByText('Work state history');
			await expect.element(disclosures.first()).toBeVisible();
			// Nothing is fetched until she asks for it: the roster read is
			// the only request so far.
			expect(
				apiFetchWithSession.mock.calls.filter((call: unknown[]) =>
					String(call[0]).includes('/work-state-history')
				)
			).toHaveLength(0);

			await disclosures.first().click();

			await expect
				.element(tableView.getByText('Changed from District of Columbia to New York'))
				.toBeVisible();
		});

		// A first assertion has no previous value (migration 00043 leaves it
		// NULL), and printing it as a change would invent one she never made.
		it('prints a first assertion as a report, not as a change', async () => {
			await setup();
			const tableView = membersTable();

			await tableView.getByText('Work state history').first().click();

			await expect.element(tableView.getByText('Reported North Carolina')).toBeVisible();
		});

		// A contractor doula who asserted her work state at another Practice
		// carries that row into this one, and the screen must not read as
		// though she said it here.
		it('marks an assertion made before she joined this practice', async () => {
			await setup();
			const tableView = membersTable();

			await tableView.getByText('Work state history').nth(1).click();

			await expect.element(tableView.getByText('Reported California')).toBeVisible();
			await expect.element(tableView.getByText('(before joining this practice)')).toBeVisible();
		});

		/*
		 * What the continuum check measures on this row (#1126). The check
		 * never clicks: it reveals every closed disclosure while it prepares
		 * the subject and waits for what that loads, which is this route's
		 * own `revealDisclosures` call and not a second procedure written
		 * here. Before that it measured the word `Loading...` at 320px and
		 * reported a screen that fits, and the fixture beside this file
		 * answered the roster for the history path, so what it would have
		 * laid out once it waited was an error notice.
		 *
		 * This asserts the entry sentence rather than the request, because
		 * what the sweep needs is content in the frame -- an answered fetch
		 * that never rendered would pass a call-count assertion and measure
		 * nothing.
		 */
		it('is already showing its entries by the time the check measures the screen', async () => {
			await setup();
			const tableView = membersTable();

			await revealDisclosures(document.querySelector('.table-view')!, 'The Staff roster');

			await expect
				.element(tableView.getByText('Changed from District of Columbia to New York'))
				.toBeVisible();
			await expect
				.element(tableView.getByRole('button', { name: 'Show older changes' }).first())
				.toBeVisible();
		});

		it('shows a per-row error notice when the history fails to load', async () => {
			await setup({ historyResponse: textResponse('Failed to load work state history') });
			const tableView = membersTable();

			await tableView.getByText('Work state history').first().click();

			await expect.element(tableView.getByText('Failed to load work state history')).toBeVisible();
		});

		it('names the Show older changes button by its member when there is more history', async () => {
			await setup({
				historyResponse: jsonResponse({
					memberSince: '2026-08-01T00:00:00Z',
					items: [{ eventId: 'event-1', workState: 'NY', createdAt: '2026-08-28T12:00:00Z' }],
					hasMore: true,
					nextCursor: 'cursor-1'
				})
			});
			const tableView = membersTable();

			await tableView.getByText('Work state history').first().click();

			expect(
				describedByText(testPage.getByRole('button', { name: 'Show older changes' }))
			).toBe(ownerMember.name);
		});

		/*
		 * #667: the sibling of #515's Buttons. Without this every row's
		 * disclosure announces the same three words, so a screen-reader user
		 * tabbing the roster -- or reading the rotor's list of controls --
		 * hears "Work state history" once per Member with nothing telling the
		 * rows apart. A <summary> computes its accessible name from its own
		 * content (HTML-AAM name-from-content), so the text this query
		 * matches is the name itself -- which is also why the fix needs no id
		 * and no aria-describedby, and so never meets #666's duplicate ids.
		 */
		it('names each row disclosure by the member it belongs to', async () => {
			await setup();
			const tableView = membersTable();

			await expect
				.element(tableView.getByText(`Work state history for ${ownerMember.name}`))
				.toBeVisible();
			await expect
				.element(tableView.getByText(`Work state history for ${contractorMember.name}`))
				.toBeVisible();
		});
	});

	/*
	 * #872: the roster shows what a person is today; this is what is
	 * behind it. Scoped to `.table-view` for the same reason the work
	 * state history block above is -- DataTable renders every row's
	 * snippet into both of its trees, so a disclosure's revealed content
	 * exists twice in the DOM.
	 */
	describe('membership history', () => {
		it('is closed until it is opened, and then names both sides of a change in the team’s words', async () => {
			await setup();
			const tableView = membersTable();

			const disclosures = tableView.getByText('Membership history');
			await expect.element(disclosures.first()).toBeVisible();
			// Nothing is fetched until she asks for it: the roster read is
			// the only request so far. This is the AC "the history is
			// fetched only when it is asked for, never as part of loading
			// the roster".
			expect(
				apiFetchWithSession.mock.calls.filter((call: unknown[]) =>
					String(call[0]).includes('/membership-history')
				)
			).toHaveLength(0);

			await disclosures.first().click();

			// The fixture stores `employee` and `contractor`; the words on
			// screen are the mapping's (#262), which is this ticket's fourth
			// acceptance criterion.
			await expect
				.element(tableView.getByText('Employment type changed from Employee to Contractor'))
				.toBeVisible();
		});

		// #316: the founding Owner gets the same 'joined' record everybody
		// else does and is named as her own actor, so "how did this person
		// come to hold these roles?" has an answer for her too.
		it('names who did it, and what a person joined as', async () => {
			await setup();
			const tableView = membersTable();

			await tableView.getByText('Membership history').first().click();

			await expect
				.element(tableView.getByText('Joined as Owner, Admin, Doula (Employee)'))
				.toBeVisible();
			await expect.element(tableView.getByText('by Renata Alvarez').first()).toBeVisible();
		});

		it('names both sides of a role change', async () => {
			await setup();
			const tableView = membersTable();

			await tableView.getByText('Membership history').nth(1).click();

			await expect
				.element(tableView.getByText('Roles changed from Doula to Admin, Doula'))
				.toBeVisible();
		});

		it('shows a per-row error notice when the history fails to load', async () => {
			await setup({
				membershipHistoryResponse: textResponse('Failed to load membership history')
			});
			const tableView = membersTable();

			await tableView.getByText('Membership history').first().click();

			await expect.element(tableView.getByText('Failed to load membership history')).toBeVisible();
		});

		it('names the Show older membership changes button by its member', async () => {
			await setup({
				membershipHistoryResponse: jsonResponse({
					items: [
						{
							eventId: 'membership-event-1',
							action: 'sessions_ended',
							actorName: 'Renata Alvarez',
							createdAt: '2027-02-02T12:00:00Z'
						}
					],
					hasMore: true,
					nextCursor: 'cursor-1'
				})
			});
			const tableView = membersTable();

			await tableView.getByText('Membership history').first().click();

			expect(
				describedByText(
					testPage.getByRole('button', { name: 'Show older membership changes' })
				)
			).toBe(ownerMember.name);
		});

		// #667, the same rule the work state disclosure carries: every row
		// has one of these, so the bare words name them all alike.
		it('names each row disclosure by the member it belongs to', async () => {
			await setup();
			const tableView = membersTable();

			await expect
				.element(tableView.getByText(`Membership history for ${ownerMember.name}`))
				.toBeVisible();
			await expect
				.element(tableView.getByText(`Membership history for ${contractorMember.name}`))
				.toBeVisible();
		});
	});

	// #515: a screen-reader user tabbing through the roster hears the same
	// bare word once per member/invitation row without this.
	it("names each member row's Edit membership, End sessions everywhere and Remove from practice button", async () => {
		await setup();

		const edit = testPage.getByRole('button', { name: 'Edit membership' });
		expect(describedByText(edit.first())).toBe(ownerMember.name);
		expect(describedByText(edit.nth(1))).toBe(contractorMember.name);

		const endSessions = testPage.getByRole('button', { name: 'End sessions everywhere' });
		expect(describedByText(endSessions.first())).toBe(ownerMember.name);
		expect(describedByText(endSessions.nth(1))).toBe(contractorMember.name);

		const remove = testPage.getByRole('button', { name: 'Remove from practice' });
		expect(describedByText(remove.first())).toBe(ownerMember.name);
		expect(describedByText(remove.nth(1))).toBe(contractorMember.name);
	});

	it("names each invitation row's Revoke button by its address", async () => {
		await setup();

		const revoke = testPage.getByRole('button', { name: 'Revoke' });
		expect(describedByText(revoke.first())).toBe(liveInvitation.address);
		expect(describedByText(revoke.nth(1))).toBe(lapsedInvitation.address);
	});

	/*
	 * #666: every id above is emitted twice, once per DataTable tree, and
	 * `describedByText` resolves the way the browser does -- first match in
	 * tree order. Two rosters' worth of rows, with the work state history
	 * disclosure opened so its own Show older changes id is in the document
	 * too, is the whole set of ids this route can produce at once.
	 */
	it('emits no duplicate id, across both DataTable trees and both rosters', async () => {
		await setup({
			historyResponse: jsonResponse({
				memberSince: '2026-08-01T00:00:00Z',
				items: [{ eventId: 'event-1', workState: 'NY', createdAt: '2026-08-28T12:00:00Z' }],
				hasMore: true,
				nextCursor: 'cursor-1'
			})
		});
		await membersTable().getByText('Work state history').first().click();
		await expect
			.element(membersTable().getByRole('button', { name: 'Show older changes' }))
			.toBeVisible();

		expect(findDuplicateIds(document)).toEqual([]);
	});

	/*
	 * #694: the way in to Owner vouching. Drawing only -- the endpoint
	 * refuses an Admin regardless (roles.ts) -- but an Admin offered a
	 * link that can only ever 403 is a screen lying about what she can do.
	 */
	it('offers each member a way to send a recovery code, named by whose it is', async () => {
		await setup();

		const links = membersTable().getByRole('link', { name: 'Send a recovery code' });
		await expect.element(links.first()).toHaveAttribute(
			'href',
			`/practices/practice-1/staff/${ownerMember.staffId}/mfa-recovery`
		);
		expect(describedByText(links.first())).toBe(ownerMember.name);
		expect(describedByText(links.nth(1))).toBe(contractorMember.name);
	});

	it('offers an Admin no such link', async () => {
		// A spread of the fixture's own session, never a second one written
		// out here (svelte-tests.md): a `practiceId` that drifted from the
		// fixture's would make every `respond(path)` match silently miss.
		const { session } = fixture.pageData as { session: Record<string, unknown> };
		pageState.data = { session: { ...session, roles: ['admin'] } };
		await setup();

		expect(testPage.getByRole('link', { name: 'Send a recovery code' }).elements()).toHaveLength(0);
	});
});
