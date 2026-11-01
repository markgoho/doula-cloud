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
import { page as testPage } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { Component } from 'svelte';
import type { StreamedRead } from '#lib/streamedRead.svelte.js';
import { gotoWithOutcome } from '#lib/outcome.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import Credits from './billing/+page.svelte';
import OnCall from './on-call/+page.svelte';
import Invoices from './invoices/+page.svelte';
import Contracts from './contracts/+page.svelte';
import Schedule from './schedule/+page.svelte';
import Engagement from './engagements/[engagementId]/+page.svelte';

if (!customElements.get('center-l')) registerLayoutPrimitives();

const pageState = vi.hoisted(() => ({
	params: { practiceId: 'practice-1', engagementId: 'engagement-1' } as Record<string, string>,
	url: new URL('https://example.test/practices/practice-1'),
	data: {} as Record<string, unknown>
}));
vi.mock('$app/state', () => ({ page: pageState }));

const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));

const didFollowSessionRefusal = vi.hoisted(() => vi.fn(async () => false));
vi.mock('#lib/api.js', () => ({ apiFetch: vi.fn(), didFollowSessionRefusal }));

vi.mock('./billing/CreditsScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./on-call/OnCallScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./invoices/InvoicesScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./contracts/ContractsScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./schedule/ScheduleScreen.svelte', async () => import('#lib/TestLoaded.svelte'));
vi.mock('./engagements/[engagementId]/EngagementScreen.svelte', async () => import('#lib/TestLoaded.svelte'));

// Each page's `data` also carries the Practice layout's session. Only the
// Engagement reads it, for the shape of the summary it holds while loading
// (#2251), so the spec hands it a Doula's and the others the read alone.
type StreamingPage = Component<{ data: { read: Promise<StreamedRead<unknown>> } }>;

const templateLoading: [string, StreamingPage, string][] = [
	['Credits', Credits as unknown as StreamingPage, 'Credits'],
	['Contracts', Contracts as unknown as StreamingPage, 'Contracts'],
	['the Engagement', Engagement as unknown as StreamingPage, 'Clients']
];

const screenLoading: [string, StreamingPage][] = [
	['On call', OnCall as unknown as StreamingPage],
	['Invoices', Invoices as unknown as StreamingPage],
	['Schedule', Schedule as unknown as StreamingPage]
];

const screens: [string, StreamingPage][] = [...templateLoading, ...screenLoading].map(([name, screen]) => [
	name,
	screen
]);

async function setup(screen: StreamingPage, read: Promise<StreamedRead<unknown>>) {
	const session = screen === (Engagement as unknown as StreamingPage) ? { session: { roles: ['doula'] } } : {};
	await render(screen, { data: { read, ...session } });
}

describe.each(templateLoading)('%s', (_name, screen, known) => {
	it('draws what it knows while the read runs', async () => {
		await setup(screen, new Promise(() => {}));

		await expect.element(testPage.getByRole('status')).toBeVisible();
		await expect.element(testPage.getByText(known, { exact: true }).first()).toBeVisible();
	});
});

describe.each(screenLoading)('%s', (_name, screen) => {
	it('draws its screen, with no read yet, while the read runs', async () => {
		await setup(screen, new Promise(() => {}));

		await expect.element(testPage.getByText('Loaded')).toHaveAttribute('title', '{}');
	});
});

describe.each(screens)('%s', (_name, screen) => {
	it('draws the refusal practices/+error.svelte would', async () => {
		await setup(screen, Promise.resolve({ isLoaded: false, response: new Response('', { status: 403 }) }));

		await expect.element(testPage.getByRole('heading', { name: 'You cannot view this' })).toBeVisible();
		await expect
			.element(testPage.getByRole('link', { name: 'Go to your Practice overview' }))
			.toHaveAttribute('href', '/practices/practice-1');
	});

	it('draws the loaded screen once the read lands', async () => {
		await setup(screen, Promise.resolve({ isLoaded: true, value: { landed: true } }));

		await expect.element(testPage.getByText('Loaded')).toHaveAttribute('title', expect.stringContaining('"landed":true'));
	});
});

// #2270: a contractor whose Attachment ended reads that the work ended,
// in the BFF's words, in place of "Page not found".
it('says the work ended where the Engagement was', async () => {
	await setup(
		Engagement as unknown as StreamingPage,
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
	const read = Promise.withResolvers<StreamedRead<unknown>>();
	goto.mockImplementationOnce(async () => {
		await setup(Engagement as unknown as StreamingPage, read.promise);
	});

	await gotoWithOutcome(engagementPath, 'Work with Pat started.');
	read.resolve({ isLoaded: true, value: { landed: true } });

	await expect
		.element(testPage.getByText('Loaded'))
		.toHaveAttribute('title', expect.stringContaining('"outcome":"Work with Pat started."'));
});
