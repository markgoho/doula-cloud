import { page as testPage } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import type { PracticeInvoiceListData } from '#lib/invoice.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts. This
// route's ListPage (#1576) also needs the primitives registered, not just
// their CSS -- see staff-list.svelte.spec.ts for why.
import '#lib/styles/app.css';
import { expectOneFramedHeading } from '#lib/components/templates/pageFrame.testing.js';
import Page from './InvoicesScreen.svelte';
import { toPageState } from '../../../routeFixture.js';
import { PRACTICE_SESSION_DEPENDENCY } from '#lib/practiceSession.js';
import { clientsCannotPay, data, fixture } from './page.fixture.js';
import { pageState } from '#testing/routePage.js';
import { afterNavigate, beforeNavigate, replaceState } from '$app/navigation';

/*
 * The screen's content and the `page` it reads both come from the route's
 * own fixture (#596), so what this spec asserts on and what the continuum
 * sweep measures are one description. `vi.mock` is hoisted above every
 * import, so the object is declared empty here and filled from the
 * fixture once the imports have run -- the route reads `page` inside its
 * own functions rather than destructuring it at module scope, so the
 * later write is seen. This is the same installation
 * `route-continuum.svelte.spec.ts` performs, through the same
 * `toPageState`.
 */
vi.mock('$app/state', () => import('#testing/routePage.js'));
if (!customElements.get('center-l')) registerLayoutPrimitives();
Object.assign(pageState, toPageState(fixture));

const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const invalidate = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ invalidate, goto: vi.fn(), afterNavigate: vi.fn(), beforeNavigate: vi.fn(), replaceState: vi.fn() }));

const [openInvoice, paidInvoice] = data.items;

// A cell's name as a screen reader hears it. Chromium puts a space either
// side of a visually hidden comma, which is out of the flow, so the words
// are matched with any spacing between them.
function heard(text: string): RegExp {
	const words = text.replaceAll(/[$.]/g, String.raw`\$&`).split(/,? /);
	return new RegExp(`^${words.join(String.raw`\s*,?\s*`)}$`);
}
const { practiceId } = fixture.params;

beforeEach(() => {
	apiFetchWithSession.mockReset();
	invalidate.mockReset();
});

async function setup(page: PracticeInvoiceListData = data) {
	// Wide enough for DataTable's <table> rather than the <dl> record view
	// its content floor stacks into below 46rem (#508) -- the same call the
	// Billing ledger's spec makes for the same reason. What the list says
	// is asserted here; that it says the same thing in a narrow container
	// is DataTable's own spec's job.
	// Above this route's own table floor (#1945, #1981, its +page.svelte), so the <table> renders.
	await testPage.viewport(1440, 900);
	await render(Page, { data: page });
}

describe('the dates the Practice-wide invoice list prints (#1958, #1965)', () => {
	// The BFF sends paidOn and dueOn as days it has already read in the
	// Practice's own timezone. Printed as calendar days, they hold in any
	// reader zone: the suite runs under TZ=UTC in CI and under zones behind
	// and ahead of UTC locally, and these assertions do not change.
	it('shows a paid Invoice on the day the BFF sent, month first', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: 'Paid Jul 4, 2026', exact: true })).toBeVisible();
	});

	it('shows the due day the BFF sent while it can be paid, and the lateness it counted with the amount', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: 'Payment due Aug 31, 2026', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('$4,500.00, 56 days overdue') })).toBeVisible();
		// The paid row's Raised day, which an instant read in a zone behind
		// UTC would have put on Jun 30.
		const paidRow = testPage.getByRole('row').filter({ hasText: 'DC-0092' });
		await expect.element(paidRow.getByRole('cell', { name: 'Jul 1, 2026', exact: true })).toBeVisible();
	});

	it('says no day for an Invoice nobody will pay', async () => {
		await setup({ ...data, items: [{ ...openInvoice, status: 'void', daysOverdue: 0 }] });

		await expect.element(testPage.getByRole('cell', { name: 'No payment due', exact: true })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('$4,500.00, void') })).toBeVisible();
	});
});

describe('the Practice-wide invoice list (#265)', () => {
	it('answers "who owes us money" with the whole book, not one Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('heading', { name: fixture.readyText }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: paidInvoice.clientName }))
			.toBeVisible();
		// The money said once, its state in the Clients list's words: the
		// open row is past its due day, so it says how late (#2122). A paid
		// row's state is its date cell's lead, so its amount stands alone.
		await expect.element(testPage.getByRole('cell', { name: heard('$4,500.00, 56 days overdue') })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('$2,500.00') })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('Paid Jul 4, 2026') })).toBeVisible();
	});

	it('reads an open Invoice that is not yet late as owed', async () => {
		await setup({ ...data, items: [{ ...openInvoice, daysOverdue: 0 }] });

		await expect.element(testPage.getByRole('cell', { name: heard('$4,500.00, owed') })).toBeVisible();
	});

	// T6: the row link already says the reference, so the table's cell says
	// only how it was billed, and the reference is heard once.
	it('says how each Invoice was billed, beside its reference', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: heard('Billed By hand') })).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('Billed through Stripe') }).first()).toBeVisible();
		await expect.element(testPage.getByRole('cell', { name: heard('Billed By hand') })).toHaveTextContent('INV-0042');
	});

	// T6: the table is named, described by the result line, and two
	// Invoices of one Client are two links a list of links tells apart.
	it('names the table, describes it by the result line, and names each link by Client and reference', async () => {
		await setup({ ...data, items: [openInvoice, { ...openInvoice, id: 'inv-9', reference: 'INV-0043' }] });

		const table = testPage.getByRole('table', { name: 'Invoices' });
		await expect.element(table).toBeVisible();
		await expect.element(table).toHaveAccessibleDescription('Showing 2 Invoices, newest first.');
		await expect.element(testPage.getByRole('link', { name: `${openInvoice.clientName}, INV-0042` })).toBeVisible();
		await expect.element(testPage.getByRole('link', { name: `${openInvoice.clientName}, INV-0043` })).toBeVisible();
	});

	it('names each Client as the way in to that Invoice on her Engagement', async () => {
		await setup();

		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toHaveAttribute(
				'href',
				`/practices/${practiceId}/engagements/${openInvoice.engagementId}#invoice-${openInvoice.id}`
			);
	});

	it('shows what Clients owe across the Practice', async () => {
		await setup();

		// The summary is a description list, so its values are read
		// positionally -- the figures also appear as a row's own amount,
		// which is exactly why an unscoped getByText would not say which is
		// the whole book's.
		const totals = testPage.getByRole('definition');
		await expect.element(totals.nth(0)).toHaveTextContent('$6,300.00 on 2 Invoices');
		await expect.element(totals.nth(2)).toHaveTextContent('$2,500.00');
		await expect.element(totals.nth(3)).toHaveTextContent('$1,250.50');
		await expect.element(testPage.getByRole('term').nth(3)).toHaveTextContent('Of that, refunded');
	});

	// #2225: the BFF says a payment is on its way, and the row says so.
	it('reads a bank payment that is still clearing as on its way', async () => {
		await setup();

		await expect.element(testPage.getByRole('cell', { name: heard('$1,800.00, payment on its way') })).toBeVisible();
	});

	it('says so plainly when nothing has been billed yet', async () => {
		// Not the happy path, so it is this spec's own to declare -- but it
		// is declared as a departure from the fixture rather than as a
		// second description of the same screen.
		await setup({ ...data, items: [], outstandingCents: 0, outstandingCount: 0, paidCents: 0 });

		await expect
			.element(
				testPage.getByText('No invoices yet. One appears here as soon as a contract is billed.')
			)
			.toBeVisible();
		// #2015: an empty book has nothing to total and nothing to narrow.
		await expect.element(testPage.getByText('Clients owe')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('radio', { name: 'Overdue' })).not.toBeInTheDocument();
	});

	it('appends the next page rather than replacing the one already read', async () => {
		// A second page is content the fixture does not hold: the screen the
		// sweep measures is the first page, and this is what arrives after an
		// interaction.
		apiFetchWithSession.mockResolvedValue(
			jsonResponse({
				items: [
					{
						id: 'inv-3',
						engagementId: 'eng-3',
						contractId: 'contract-3',
						clientName: 'Cleo',
						reference: 'INV-3',
						status: 'open',
						amountCents: 100,
						currency: 'usd',
						createdAt: '2026-06-01T00:00:00Z',
						raisedOn: '2026-06-01',
						dueOn: '2026-07-01',
						daysOverdue: 0
					}
				],
				hasMore: false,
				outstandingCents: 450_100,
				outstandingCount: 2,
				paidCents: 250_000
			})
		);

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: openInvoice.clientName }))
			.toBeVisible();
		expect(apiFetchWithSession).toHaveBeenCalledWith(
			`/api/practices/${practiceId}/invoices?cursor=cursor-1`
		);
	});

	it('reports a failed next page in its own words, keeps the rows, and lets Load more try again', async () => {
		apiFetchWithSession.mockResolvedValueOnce(jsonResponse('invalid cursor', 400));

		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect
			.element(testPage.getByRole('alert'))
			.toHaveTextContent('More Invoices did not load. Nothing you saved is lost. Try again.');
		await expect.element(testPage.getByText('invalid cursor')).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: openInvoice.clientName })).toBeVisible();

		apiFetchWithSession.mockResolvedValueOnce(jsonResponse({ ...data, items: [{ ...openInvoice, id: 'inv-9', clientName: 'Cleo' }], hasMore: false }));
		await testPage.getByRole('button', { name: 'Load more' }).click();

		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();
		await expect.element(testPage.getByRole('alert')).not.toBeInTheDocument();
	});
});

// T8 (#2746): a read that may work a second time says so in the screen's
// own words, keeps the view and the search, and "Try again" runs it again.
describe('a read that did not land', () => {
	it('says the Invoices did not load, holds no placeholder, and Try again runs the read again', async () => {
		await testPage.viewport(1440, 900);
		const retried = Promise.withResolvers<void>();
		const onRetry = vi.fn(() => retried.promise);
		await render(Page, { hasReadFailed: true, onRetry });

		await expect.element(testPage.getByRole('alert')).toHaveTextContent('Your Invoices did not load. Nothing you saved is lost.');
		await expect.element(testPage.getByRole('status', { name: 'Loading your Invoices' })).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('radio', { name: 'Every Invoice' })).toBeVisible();

		const tryAgain = testPage.getByRole('button', { name: 'Try again' });
		await tryAgain.click();
		expect(onRetry).toHaveBeenCalledOnce();
		await expect.element(tryAgain).toHaveAttribute('aria-busy', 'true');
		retried.resolve();
		await expect.element(tryAgain).toHaveAttribute('aria-busy', 'false');
	});

	it('draws the list once the read Try again ran lands', async () => {
		await testPage.viewport(1440, 900);
		const screen = await render(Page, { hasReadFailed: true });
		await screen.rerender({ hasReadFailed: false, data });

		await expect.element(testPage.getByRole('link', { name: openInvoice.clientName })).toBeVisible();
		await expect.element(testPage.getByRole('alert')).not.toBeInTheDocument();
	});
});

// T3 (#2746): Back from an Invoice puts her on the same rows, where she was.
describe('coming back from an Invoice', () => {
	afterEach(() => {
		pageState.state = {};
		vi.restoreAllMocks();
	});

	it('keeps how many pages were open and how far down she was when she leaves for another screen', async () => {
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [{ ...openInvoice, id: 'inv-9', clientName: 'Cleo' }], hasMore: false }));
		await setup({ ...data, hasMore: true, nextCursor: 'cursor-1' });
		await testPage.getByRole('button', { name: 'Load more' }).click();
		await expect.element(testPage.getByRole('link', { name: 'Cleo' })).toBeVisible();

		const leave = vi.mocked(beforeNavigate).mock.calls.at(-1)![0];
		const away = (type: string, pathname: string) =>
			leave({ type, to: { url: new URL(`https://example.test${pathname}`) } } as never);
		away('popstate', '/elsewhere');
		away('link', pageState.url.pathname);
		expect(replaceState).not.toHaveBeenCalled();

		away('link', `/practices/${practiceId}/engagements/eng-1`);
		expect(replaceState).toHaveBeenCalledWith('', expect.objectContaining({ invoicesPages: 2, invoicesScrollY: scrollY }));
	});

	it('reads as many pages again, scrolls back, and lands on the row she left from', async () => {
		pageState.state = { invoicesPages: 2, invoicesScrollY: 120 };
		const scrollTo = vi.spyOn(globalThis, 'scrollTo').mockImplementation(() => {});
		apiFetchWithSession.mockResolvedValue(jsonResponse({ ...data, items: [{ ...openInvoice, id: 'inv-9', clientName: 'Cleo' }], hasMore: false }));
		await testPage.viewport(1440, 900);
		const screen = await render(Page, {});
		const arrive = vi.mocked(afterNavigate).mock.calls.at(-1)![0];
		arrive({ from: { url: new URL('https://example.test/practices/p/engagements/e#invoice-inv-9') } } as never);
		await screen.rerender({ data: { ...data, hasMore: true, nextCursor: 'cursor-1' } });

		const left = testPage.getByRole('link', { name: /^Cleo,/ });
		await expect.element(left).toBeVisible();
		await vi.waitFor(() => expect(scrollTo).toHaveBeenCalledWith(0, 120));
		await expect.element(left).toHaveFocus();
		await expect.element(testPage.getByRole('button', { name: 'Load more' })).not.toBeInTheDocument();
	});
});

// #1576: this list sat straight in the layout's <main>, with no gutter,
// until it moved onto ListPage.
describe('the page frame (#1576)', () => {
	it('renders its one heading inside the page frame', async () => {
		await setup();

		await expectOneFramedHeading(fixture.readyText);
	});
});

// #2255: the screen draws what the URL already says before the read lands.
async function setupLoading() {
	await testPage.viewport(1440, 900);
	await render(Page, {});
}

describe('the Practice-wide invoice list while its read runs (#2255)', () => {
	it('draws the totals’ labels and the filter bar, and says what it is loading', async () => {
		await setupLoading();

		await expect.element(testPage.getByText('Clients owe')).toBeVisible();
		await expect.element(testPage.getByRole('radio', { name: 'Every Invoice' })).toBeChecked();
		await expect.element(testPage.getByRole('button', { name: 'Apply' })).toBeVisible();
		await expect.element(testPage.getByRole('status', { name: 'Loading your invoices' })).toBeVisible();
	});

	// H1: the totals hold their place while the figures load, so the filter
	// does not move when they land, at a desk's width or a phone's.
	it.each([1440, 375])('does not move the filter when the figures land, at %ipx', async (width) => {
		await testPage.viewport(width, 900);
		const { rerender } = await render(Page, {});
		const apply = () => testPage.getByRole('button', { name: 'Apply' }).element().getBoundingClientRect().top;
		const before = apply();

		await rerender({ data });
		await expect.element(testPage.getByRole('definition').nth(0)).toHaveTextContent('$6,300.00');

		expect(apply()).toBe(before);
	});
});

// #270, #2293: from the session, so it is above the book on the first frame.
// #2200: the fixture's reader is the Owner, so hers is the ask.
const cannotPayText = 'Clients cannot pay you by card yet. To take card payments, connect Stripe.';

async function setupClientsCanPay(
	canPay: boolean,
	read: PracticeInvoiceListData | undefined,
	session: Record<string, unknown> = {}
) {
	const state = toPageState(canPay ? fixture : { ...fixture, ...clientsCannotPay }).data as {
		session: Record<string, unknown>;
	};
	pageState.data = { ...state, session: { ...state.session, ...session } };
	await testPage.viewport(1440, 900);
	return render(Page, read ? { data: { ...read, clientsCanPay: canPay } } : {});
}

function isBefore(earlier: Element, later: Element): boolean {
	return Boolean(earlier.compareDocumentPosition(later) & Node.DOCUMENT_POSITION_FOLLOWING);
}

describe('whether Clients can pay this Practice (#270, #2293)', () => {
	afterEach(() => {
		pageState.data = toPageState(fixture).data;
	});

	it('shows the Notice above the invoice table', async () => {
		await setupClientsCanPay(false, data);

		const notice = testPage.getByText(cannotPayText);
		await expect.element(notice).toBeVisible();
		await expect.element(testPage.getByRole('table')).toBeVisible();
		expect(isBefore(notice.element(), testPage.getByRole('table').element())).toBe(true);
	});

	it('shows the Notice above the rows’ placeholder while the read runs', async () => {
		await setupClientsCanPay(false, undefined);

		const notice = testPage.getByText(cannotPayText);
		const loading = testPage.getByRole('status', { name: 'Loading your invoices' });
		await expect.element(notice).toBeVisible();
		await expect.element(loading).toBeVisible();
		expect(isBefore(notice.element(), loading.element())).toBe(true);
	});

	it('shows the Notice on an empty book', async () => {
		await setupClientsCanPay(false, { ...data, items: [], outstandingCents: 0, outstandingCount: 0, paidCents: 0 });

		await expect.element(testPage.getByText(cannotPayText)).toBeVisible();
	});

	it('links the Owner to Getting paid to connect Stripe (#2200)', async () => {
		await setupClientsCanPay(false, data);

		await expect
			.element(testPage.getByRole('link', { name: 'Connect Stripe on Getting paid' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/payments');
	});

	it('tells an Admin who connects Stripe, and links her to Getting paid (#2200)', async () => {
		await setupClientsCanPay(false, data, { roles: ['admin'] });

		await expect
			.element(
				testPage.getByText('Clients cannot pay this Practice by card yet. A Practice Owner has to connect Stripe.')
			)
			.toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Go to Getting paid' }))
			.toHaveAttribute('href', '/practices/practice-1/settings/payments');
	});

	// #2200: Clients pay a Practice that bills by hand without Stripe.
	it('says nothing about Stripe at a Practice that bills by hand', async () => {
		await setupClientsCanPay(false, data, { billingMode: 'by_hand' });

		await expect.element(testPage.getByRole('table')).toBeVisible();
		await expect.element(testPage.getByText(/Clients cannot pay/)).not.toBeInTheDocument();
		await expect.element(testPage.getByRole('link', { name: /Getting paid/ })).not.toBeInTheDocument();
	});

	it('shows no Notice when Clients can pay, while the read runs or once it lands', async () => {
		await setupClientsCanPay(true, undefined);
		await expect.element(testPage.getByText(cannotPayText)).not.toBeInTheDocument();

		await setupClientsCanPay(true, data);
		await expect.element(testPage.getByRole('table')).toBeVisible();
		await expect.element(testPage.getByText(cannotPayText)).not.toBeInTheDocument();
	});
});

async function setupReconcile(canPayPerSession: boolean) {
	const { rerender } = await setupClientsCanPay(canPayPerSession, undefined);
	return {
		land: async (canPayPerRead: boolean) => {
			await rerender({ data: { ...data, clientsCanPay: canPayPerRead } });
			await expect.element(testPage.getByRole('table')).toBeVisible();
		}
	};
}

describe('keeping the session in step with the landed read (#2311)', () => {
	afterEach(() => {
		pageState.data = toPageState(fixture).data;
	});

	it.each([
		{ canPayPerSession: false, canPayPerRead: true },
		{ canPayPerSession: true, canPayPerRead: false }
	])(
		'reads the session again once when it says $canPayPerSession and the read says $canPayPerRead',
		async ({ canPayPerSession, canPayPerRead }) => {
			const { land } = await setupReconcile(canPayPerSession);
			expect(invalidate).not.toHaveBeenCalled();

			await land(canPayPerRead);

			expect(invalidate).toHaveBeenCalledExactlyOnceWith(PRACTICE_SESSION_DEPENDENCY);
		}
	);

	it.each([true, false])('leaves the session alone when both say %s', async (canPay) => {
		const { land } = await setupReconcile(canPay);

		await land(canPay);

		expect(invalidate).not.toHaveBeenCalled();
	});
});
