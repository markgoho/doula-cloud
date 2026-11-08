/*
 * The six Practice screens whose `load` hands its read over unawaited
 * (#2249): each `+page.svelte` draws a loading state while the read runs,
 * the page `practices/+error.svelte` draws when it fails, and its loaded
 * screen once it lands. Each loaded screen has a spec of its own, so here
 * it is a stand-in.
 *
 * Schedule, On call and Invoices draw their screen from the first frame
 * (#2255): it draws what the URL already says, and its own placeholders
 * until the read lands, so here the stand-in is drawn with no read yet.
 */
import type { Component } from 'svelte';
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { StreamedRead } from '#lib/streamedRead.svelte.js';
import { gotoWithOutcome } from '#lib/outcome.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import type { PracticeSession } from './+layout.js';
import Credits from './billing/+page.svelte';
import OnCall from './on-call/+page.svelte';
import Invoices from './invoices/+page.svelte';
import Contracts from './contracts/+page.svelte';
import Schedule from './schedule/+page.svelte';
import Engagement from './engagements/[engagementId]/+page.svelte';
import { data as balance } from './billing/page.fixture.js';
import { data as onCallPage } from './on-call/page.fixture.js';
import { data as invoicesPage } from './invoices/page.fixture.js';
import { data as contractsPage } from './contracts/page.fixture.js';
import { data as schedulePage } from './schedule/page.fixture.js';
import { detail as engagementSummary } from './engagements/[engagementId]/page.fixture.js';

if (!customElements.get('center-l')) registerLayoutPrimitives();

const pageState = vi.hoisted(() => ({
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' } as Record<string, string>,
	url: new URL('https://example.test/practices/practice-1'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto, afterNavigate: vi.fn() }));

const didFollowSessionRefusal = vi.hoisted(() => vi.fn(async () => false));
vi.mock('#lib/api.js', async (importOriginal) => ({
	...(await importOriginal<typeof import('#lib/api.js')>()),
	apiFetch: vi.fn(),
	didFollowSessionRefusal
}));

vi.mock('./billing/CreditsScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./on-call/OnCallScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./invoices/InvoicesScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./contracts/ContractsScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./schedule/ScheduleScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./engagements/[engagementId]/EngagementScreen.svelte', async () => import('#lib/TestLoaded.svelte'));

// Each page's `data` also carries the Practice layout's session. Only the
// Engagement reads it, for the shape of the summary it holds while loading
// (#2251), so it is a Doula's.
const session: PracticeSession = {
	practiceId: 'practice-1',
	staffId: 'staff-1',
	practiceName: 'Riverside Doula Collective',
	roles: ['doula'],
	isContractor: false
};

type Landing = { isLoaded: true } | { isLoaded: false; response?: Response };

interface StreamingRow {
	name: string;
	mount: (landing: Promise<Landing>) => Promise<unknown>;
	loadedKey: string;
}

// `propertiesFor` is checked against the page's own `PageProps`, and `value`
// against what its `load` reads, so a page that asks for more than this
// hands it fails the type check.
function row<Value extends object, Properties extends object>(
	name: string,
	page: Component<Properties>,
	value: Value,
	propertiesFor: (read: Promise<StreamedRead<Value>>) => Properties
): StreamingRow {
	async function read(landing: Promise<Landing>): Promise<StreamedRead<Value>> {
		const settled = await landing;
		return settled.isLoaded ? { isLoaded: true, value } : settled;
	}
	return {
		name,
		loadedKey: `"${Object.keys(value)[0]}":`,
		mount: (landing) => render(page, propertiesFor(read(landing)))
	};
}

const credits = row('Credits', Credits, balance, (read) => ({
	params: { practiceId: 'practice-1' },
	data: { read, session }
}));
const contracts = row('Contracts', Contracts, contractsPage, (read) => ({
	params: { practiceId: 'practice-1' },
	data: { read, session }
}));
const engagement = row('the Engagement', Engagement, engagementSummary, (read) => ({
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' },
	data: { read, session }
}));
const onCall = row('On call', OnCall, onCallPage, (read) => ({
	params: { practiceId: 'practice-1' },
	data: { read, session }
}));
const invoices = row('Invoices', Invoices, invoicesPage, (read) => ({
	params: { practiceId: 'practice-1' },
	data: { read, session }
}));
const schedule = row('Schedule', Schedule, schedulePage, (read) => ({
	params: { practiceId: 'practice-1' },
	data: { read, session }
}));

const templateLoading = [
	{ screen: credits, known: 'Credits' },
	{ screen: contracts, known: 'Contracts' },
	{ screen: engagement, known: 'Clients' }
];

const screenLoading = [onCall, invoices, schedule];

const screens = [credits, contracts, engagement, onCall, invoices, schedule];

const running = () => new Promise<Landing>(() => {});

describe.each(templateLoading)('$screen.name', ({ screen, known }) => {
	it('draws what it knows while the read runs', async () => {
		await screen.mount(running());

		await expect.element(testPage.getByRole('status')).toBeVisible();
		await expect.element(testPage.getByText(known, { exact: true }).first()).toBeVisible();
	});
});

describe.each(screenLoading)('$name', (screen) => {
	it('draws its screen, with no read yet, while the read runs', async () => {
		await screen.mount(running());

		await expect.element(testPage.getByText('Loaded')).toHaveAttribute('title', '{}');
	});
});

describe.each(screens)('$name', (screen) => {
	it('draws the refusal practices/+error.svelte would', async () => {
		await screen.mount(Promise.resolve({ isLoaded: false, response: new Response('', { status: 403 }) }));

		await expect.element(testPage.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Go to your Practice overview' }))
			.toHaveAttribute('href', '/practices/practice-1');
	});

	it('draws the loaded screen once the read lands', async () => {
		await screen.mount(Promise.resolve({ isLoaded: true }));

		await expect
			.element(testPage.getByText('Loaded'))
			.toHaveAttribute('title', expect.stringContaining(screen.loadedKey));
	});
});

// #2270: a contractor whose Attachment ended reads that the work ended,
// in the BFF's words, in place of "Page not found".
it('says the work ended where the Engagement was', async () => {
	await engagement.mount(
		Promise.resolve({
			isLoaded: false,
			response: Response.json(
				{ code: 'ATTACHMENT_ENDED', message: 'Your work on the birth due Feb 2027 ended on Oct 31, 2026.' },
				{ status: 410 }
			)
		})
	);

	await expect.element(testPage.getByRole('heading', { name: 'Your work has ended' })).toBeVisible();
	await expect.element(testPage.getByText('Your work on the birth due Feb 2027 ended on Oct 31, 2026.')).toBeVisible();
	await expect
		.element(testPage.getByRole('link', { name: 'Your offers' }))
		.toHaveAttribute('href', '/practices/practice-1/offers');
	await expect.element(testPage.getByText('Page not found')).not.toBeInTheDocument();
});

// #1710: a save hands its message over only while its own navigation
// runs, which ends once this page draws its loading state and before the
// read lands. The page takes it then, and the loaded screen says it.
it('hands the Engagement screen what the save that led here did', async () => {
	const engagementPath = '/practices/practice-1/engagements/engagement-1';
	pageState.url = new URL(`https://example.test${engagementPath}`);
	const landing = Promise.withResolvers<Landing>();
	goto.mockImplementationOnce(async () => {
		await engagement.mount(landing.promise);
	});

	await gotoWithOutcome(engagementPath, 'Work with Pat started.');
	landing.resolve({ isLoaded: true });

	await expect
		.element(testPage.getByText('Loaded'))
		.toHaveAttribute('title', expect.stringContaining('"outcome":"Work with Pat started."'));
});
