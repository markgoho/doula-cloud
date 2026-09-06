import { page } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import { mountInFrame, overflowReport, sweep } from '../../../../style-guide/continuum.js';
import Hub from './+page.svelte';
// Rendering `+page.svelte` directly bypasses `+layout.svelte`, the only
// place the real app calls this -- without it a layout primitive sits
// unregistered and inert (clients-list.svelte.spec.ts's own reason for the
// same line). #486's own DataTable assertions below need the real
// table-view/record-view switch.
import '#lib/styles/app.css';
import { toApiResponder, toPageState } from '../../../../routeFixture.js';
import { detail, fixture } from './page.fixture.js';
if (!customElements.get('center-l')) registerLayoutPrimitives();

/*
 * The Engagement this screen shows, and the `page` it reads, both come
 * from the route's own fixture (#596) -- so the screen this spec asserts
 * on and the screen the continuum sweep measures are one description.
 * `vi.mock` is hoisted above every import, so `pageState` is declared
 * empty and filled from the fixture once the imports have run; the hub
 * reads `page.params.engagementId` for the fetch and its two Link hrefs,
 * and `page.data.practiceName` for the bar, inside its own functions
 * rather than at module scope, so the later write is seen. Same
 * installation, through the same `toPageState`, as
 * `route-continuum.svelte.spec.ts`.
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

function jsonResponse(body: unknown) {
	return { ok: true, json: () => Promise.resolve(body) } as Response;
}

// Both the Engagement detail read and #486's own /activity read share this
// one mock, branched by path -- a blanket single response would hand the
// activity loader the detail body instead of a { items, hasMore } page.
function mockFetch(body: unknown, activityItems: unknown[] = []) {
	apiFetchWithSession.mockImplementation((path: string) =>
		Promise.resolve(jsonResponse(path.includes('/activity') ? { items: activityItems, hasMore: false } : body))
	);
}

describe('Client portal Engagement hub', () => {
	// The happy path: the fixture's own detail already carries a due date
	// (2027-03-01), so this is the fixture's content unmodified.
	it("shows the due date under its own label, not 'Created' (#505)", async () => {
		apiFetchWithSession.mockImplementation(toApiResponder(fixture));

		await render(Hub);

		await expect.element(page.getByText('Due date')).toBeVisible();
		await expect.element(page.getByText('Mar 1, 2027')).toBeVisible();
		await expect.element(page.getByText('Created')).not.toBeInTheDocument();
	});

	// ADR-0017: a postpartum-only Engagement has no due date. #505 asks for
	// nothing to show, not a blank row and not a placeholder -- the row is
	// left out of the DescriptionList's own items entirely, so there is no
	// "Due date" label sitting over an empty value. Not the happy path --
	// the fixture's own detail has a due date (see the previous test) --
	// so this is a departure from it, spread rather than restated.
	it('shows nothing for a null due date -- no blank label, no placeholder', async () => {
		mockFetch({ ...detail, dueDate: undefined });

		await render(Hub);

		// #212: the register's own label ("Ongoing" for `active`), not the
		// raw enum value -- this is also this suite's own assertion for
		// #212's AC4 (the portal shows the register's word for a status
		// value).
		await expect.element(page.getByText('Ongoing')).toBeVisible();
		await expect.element(page.getByText(/due date/i)).not.toBeInTheDocument();
	});
});

// #486 AC5: CONTEXT.md's own vocabulary for this to a Client -- "Everything
// that has happened" -- behind a closed disclosure, per the design brief's
// own #433 amendment for the Client portal.
describe('the Activity disclosure (#486)', () => {
	// DataTable renders both a <table> and a record view at once (#508,
	// ADR-0024), so a bare page-wide getByText matches both -- scoped here
	// to `.table-view` the same way DataTable.svelte.spec.ts's own record-
	// view tests do, rather than asserting on ambiguous duplicate text.
	// `.frame` has no accessible signal of its own for open/closed --
	// svelte-tests.md's rule 1, the same exception DataTable's own
	// disclosure test already relies on: `<details>` without `open`
	// removes its content from the accessibility tree entirely rather than
	// merely marking it not-visible, so `getByRole`/`getByText` can never
	// resolve anything inside it to assert against.
	it('renders closed under its own heading, with a descriptive toggle', async () => {
		await page.viewport(1440, 900);
		mockFetch(detail, [
			{
				subjectKind: 'engagement',
				subjectId: detail.engagementId,
				action: 'contract_sent',
				actorKind: 'staff',
				// A generic name, not a person's -- portal.ActivityHandler
				// (Go) already replaces a staff actor's name before this
				// response ever reaches the browser (CONTEXT.md's Activity
				// entry: "never who inside the Practice did what"). This
				// spec only proves the frontend renders whatever name it is
				// given, muted; the redaction itself has its own Go test.
				actorName: 'Your practice',
				createdAt: new Date().toISOString()
			}
		]);

		const { container } = await render(Hub);

		await expect
			.element(page.getByRole('heading', { name: 'Everything that has happened' }))
			.toBeVisible();
		await expect.element(page.getByText('Show what has happened')).toBeVisible();
		expect(getComputedStyle(container.querySelector('.frame')!).display).toBe('none');

		await page.getByText('Show what has happened').click();
		expect(getComputedStyle(container.querySelector('.frame')!).display).not.toBe('none');
		const tableView = page.elementLocator(container.querySelector('.table-view')!);
		await expect.element(tableView.getByText('Contract sent')).toBeVisible();
		await expect.element(tableView.getByText('Your practice')).toBeVisible();
	});

	// ADR-0024/0025: route-continuum.svelte.spec.ts sweeps every route from
	// 320px up, but a closed <details> takes no box at all while closed --
	// its content is unrendered, not merely hidden -- so that sweep can
	// never measure what is actually inside this disclosure, at any width.
	// This test opens it first and reuses the same sweep instrument
	// (continuum.ts), so #486's AC7 ("free of horizontal scroll from
	// 320px up") is checked for the ledger's own open-state layout rather
	// than only asserted in a doc comment.
	it('is free of horizontal overflow from 320px up once opened (ADR-0024/0025)', async () => {
		mockFetch(detail, [
			{
				subjectKind: 'engagement',
				subjectId: detail.engagementId,
				action: 'contract_sent',
				actorKind: 'staff',
				actorName: 'Your practice',
				createdAt: new Date().toISOString()
			}
		]);

		const { run, frame, remove } = await mountInFrame(Hub);
		try {
			await expect
				.element(page.getByRole('heading', { name: 'Everything that has happened' }))
				.toBeVisible();
			await page.getByText('Show what has happened').click();
			// Confirms the disclosure actually opened before sweeping, via a
			// plain DOM read rather than an accessible query: DataTable
			// renders both a table view and a record view for the same row
			// at once (#508), one of them display:none depending on the
			// frame's own width at this point in the test, so a role/text
			// query would either hit a strict-mode multiple match or resolve
			// against whichever tree is currently hidden.
			await expect.poll(() => frame.querySelector('.frame')?.textContent).toContain('Contract sent');

			const found = sweep(frame, run.clientWidth);
			expect(found, found && overflowReport('Client-portal Activity disclosure (open)', found)).toBeUndefined();
		} finally {
			remove();
		}
	});

	it('says so when the ledger cannot be read', async () => {
		apiFetchWithSession.mockImplementation((path: string) =>
			Promise.resolve(
				path.includes('/activity')
					? ({ ok: false, text: () => Promise.resolve('nope') } as Response)
					: jsonResponse(detail)
			)
		);

		await render(Hub);

		await expect.element(page.getByText('nope')).toBeVisible();
	});
});
