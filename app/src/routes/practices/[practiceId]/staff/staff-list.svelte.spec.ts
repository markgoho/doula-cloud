import { page as testPage } from 'vitest/browser';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { findDuplicateIds } from '#lib/duplicateIds.js';
import { gotoWithOutcome } from '#lib/outcome.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#491) also needs the primitives registered, not just
// their CSS: <center-l max="none"> only lifts the default var(--measure) cap
// via the custom element's own attribute handling, and an unregistered
// center-l never runs it, leaving every DataTable narrower than its floor.
import '#lib/styles/app.css';
import Page from './+page.svelte';
import { toPageState } from '../../../routeFixture.js';
import { fixture, roster } from './page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The `page` this route reads comes from its own fixture (#596), so the
 * params this spec installs and the params the continuum sweep installs
 * are one description. `vi.mock` is hoisted above every import, so the
 * object is declared empty here and filled from the fixture once the
 * imports have run.
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
// The real `apiErrorMessage` reads a refusal's `{code, message}` envelope
// (docs/api-design.md section 7); it is imported from the leaf module it
// lives in so the whole of `api.ts`, and Firebase with it, stays out.
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

interface MockOptions {
	roster?: { members: typeof members; invitations: typeof invitations };
	listOk?: boolean;
	revokeResponse?: Response;
}

// The API double is stateful: a successful revoke changes what the next
// roster read returns, so a test can assert on the screen the Owner ends
// up looking at rather than on the request that got her there.
function mockApi({ roster = { members, invitations }, listOk = true, revokeResponse }: MockOptions = {}) {
	const state = structuredClone(roster);
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/revoke')) {
			if (revokeResponse) return Promise.resolve(revokeResponse);
			const invitationId = path.split('/').at(-2);
			state.invitations = state.invitations.filter((invitation) => invitation.invitationId !== invitationId);
			return Promise.resolve(jsonResponse({}));
		}
		return Promise.resolve(
			listOk
				? jsonResponse({ members: state.members, invitations: { items: state.invitations, hasMore: false } })
				: textResponse('Server rejected the Staff list request')
		);
	});
}

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	Object.assign(pageState, toPageState(fixture));
});

/*
 * Scoped to the second .table-view, not an unscoped getByText: DataTable
 * renders rowActions.content once per tree (#508, ADR-0024), so a Badge
 * living inside a row's actions exists twice -- once in the <table>, once
 * in the record view -- and getByText matches DOM text regardless of
 * which tree CSS hides. The same sanctioned querySelector exception
 * DataTable.svelte.spec.ts uses. Members render before Pending
 * invitations, so index order is the DOM's own.
 */
function invitationsTable() {
	return testPage.elementLocator(document.querySelectorAll('.table-view')[1]!);
}

async function setup(options: MockOptions = {}) {
	// Wide enough that every DataTable draws its <table>, which is what
	// the cell and row assertions below are about.
	await testPage.viewport(3200, 800);
	mockApi(options);
	await render(Page, {});
	// The roster arrives from onMount; several tests read the DOM
	// synchronously, so the wait belongs here. Skipped when the load is
	// meant to fail, since then this heading never renders.
	if (options.listOk !== false) {
		await expect.element(testPage.getByRole('heading', { name: 'Pending invitations' })).toBeVisible();
	}
}

// "Revoke" alone doesn't say which invitation it acts on (#515); the
// address is a sibling joined by aria-describedby, so no accessible query
// names it directly.
function describedByText(button: ReturnType<typeof testPage.getByRole>): string {
	const describedBy = button.element().getAttribute('aria-describedby') ?? '';
	return document.querySelector(`#${describedBy}`)?.textContent ?? '';
}

describe('staff screen', () => {
	it('never scrolls the document sideways at 320px (#508)', async () => {
		await setup();
		await testPage.viewport(320, 800);

		await expect.element(testPage.getByRole('heading', { name: 'Members' })).toBeVisible();
		expect(document.documentElement.scrollWidth).toBe(document.documentElement.clientWidth);
	});

	it('lists members with their roles and employment type', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: ownerMember!.email })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Owner, Admin' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'No roles yet' })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'Contractor' }).first()).toBeVisible();
	});

	// #2041: a member row is facts and one link, the name, to that Staff
	// member's own page. Nothing in the row is a control.
	it("links each member's name to that Staff member's page, and puts no control in a member row", async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: contractorMember!.name, exact: true }).first())
			.toHaveAttribute('href', `/practices/practice-1/staff/${contractorMember!.staffId}`);
		const memberRows = testPage
			.getByRole('row')
			.filter({ hasText: ownerMember!.email });
		await expect.element(memberRows.first()).toBeVisible();
		expect(memberRows.first().getByRole('button').elements()).toHaveLength(0);
	});

	// #2041: a removal on a Staff member's page lands here and says so.
	it('says what the removal that led here did, before the title', async () => {
		goto.mockImplementationOnce(async () => {
			await setup();
		});

		await gotoWithOutcome(fixture.url, `${ownerMember!.name} was removed from this Practice.`);

		const message = testPage.getByRole('status').filter({ hasText: 'was removed from this Practice.' });
		await expect.element(message).toBeVisible();
		await expect.element(message).toHaveFocus();
	});

	it('lists pending invitations as their own group, apart from the members (#261)', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('cell', { name: liveInvitation!.address, exact: true }))
			.toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: 'doula' })).toBeVisible();
	});

	it('flags an invitation whose email could not be delivered (#339)', async () => {
		await setup();

		const flags = invitationsTable().getByText('Email could not be delivered');
		await expect.element(flags).toBeVisible();
		expect(flags.elements()).toHaveLength(1);
	});

	it('flags a lapsed invitation instead of hiding it (#291)', async () => {
		await setup();

		await expect.element(invitationsTable().getByText('Expired — invite again or revoke')).toBeVisible();
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

	it('removes a revoked invitation from the pending group', async () => {
		await setup();

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect.element(testPage.getByRole('cell', { name: liveInvitation!.address })).not.toBeInTheDocument();
		await expect
			.element(testPage.getByRole('cell', { name: lapsedInvitation!.address, exact: true }))
			.toBeVisible();
	});

	it('shows a per-row error notice when revoking fails', async () => {
		await setup({ revokeResponse: textResponse('no pending invitation found at this practice') });

		await testPage.getByRole('button', { name: 'Revoke' }).first().click();
		const dialog = testPage.getByRole('dialog', { name: 'Revoke invitation' });
		await dialog.getByRole('button', { name: 'Revoke invitation' }).click();

		await expect.element(invitationsTable().getByText('no pending invitation found at this practice')).toBeVisible();
	});

	it("names each invitation row's Revoke button by its address (#515)", async () => {
		await setup();

		const revoke = testPage.getByRole('button', { name: 'Revoke' });
		expect(describedByText(revoke.first())).toBe(liveInvitation!.address);
		expect(describedByText(revoke.nth(1))).toBe(lapsedInvitation!.address);
	});

	// #666: every id in a row's actions is emitted twice, once per
	// DataTable tree, and `describedByText` resolves the way the browser
	// does -- first match in tree order.
	it('emits no duplicate id across both DataTable trees', async () => {
		await setup();

		expect(document.querySelectorAll('[id]').length).toBeGreaterThan(0);
		expect(findDuplicateIds(document)).toEqual([]);
	});
});
