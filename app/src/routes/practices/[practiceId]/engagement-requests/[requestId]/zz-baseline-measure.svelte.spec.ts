/*
 * THROWAWAY measurement harness, kept only as a record. Wayfinder #521,
 * the baseline run against #502. It is not part of the suite: it lives on
 * the branch `research/baseline-521-harness` and was deleted from trunk
 * the moment it had produced its numbers.
 *
 * What it does: renders the real route in real Chromium with the real
 * stylesheet, then sweeps the space the page is given from 1400px down to
 * 280px one pixel at a time, asserting on every element that nothing
 * reaches past the frame's edge. It reports the *first* width at which
 * each element breaks, so the output is a list of onset points rather than
 * a snapshot at a width somebody chose.
 *
 * Three techniques in here are worth keeping, because each one took a
 * try or two and whoever builds the real continuum check (the map's
 * "automated verification check", waiting on #527) needs all three:
 *
 * 1. Nothing in `vite.config.ts` sets up the app's CSS or its custom
 *    elements for a component test -- the root `+layout.svelte` does that
 *    at runtime. So a harness must import `#lib/styles/app.css` and call
 *    `registerLayoutPrimitives()` itself, guarded on
 *    `customElements.get`, because the registry survives between tests in
 *    a file and a second `define` throws.
 * 2. Browser-mode `console.log` does not reach the terminal. The report
 *    comes back as a deliberately failing `expect(report).toBe(...)`,
 *    which prints it in the diff.
 * 3. A harness that finds nothing is worthless unless it is shown to be
 *    able to find something. `harness sanity` plants a 700px-min-width
 *    element and asserts the sweep catches it -- it caught 39 elements.
 *
 * Run: `bunx vitest --run --project client <this file>`
 */
import { page as testPage } from 'vitest/browser';
import { beforeEach, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { jsonResponse } from '#lib/testResponse.js';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import type { ApprovalDetail } from '#lib/engagementRequest.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({
	page: { params: { practiceId: 'practice-1', requestId: 'request-1' } }
}));
const goto = vi.hoisted(() => vi.fn());
vi.mock('$app/navigation', () => ({ goto }));
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

const modest: ApprovalDetail = {
	requestId: 'request-1', state: 'pending', kind: 'birth', dueDate: '2027-03-01',
	note: 'Referred by the hospital', requestedBy: 'staff-1', requestedByName: 'Ada Doula',
	requestedAt: '2026-08-01T10:00:00Z',
	client: { clientId: 'client-1', givenName: 'Mara', familyName: 'Quinn', preferredName: '', isNewToPractice: true },
	creditCost: 1, balance: 3, balanceAfter: 2, engagements: []
};

// Longer, but every token still breakable -- this is the "reasonable" content
// a fixture author invents, and it never overflowed at any width.
const real: ApprovalDetail = {
	...modest,
	note: 'She was referred by the midwifery group at Highland Hospital and would like someone who has supported a VBAC before. Her partner works nights until the end of February.',
	requestedByName: 'Anastasia Wolstenholme-Fitzgerald',
	client: { clientId: 'client-1', givenName: 'Persephone', familyName: 'Vandenberghe-Okonkwo', preferredName: '', isNewToPractice: false },
	balance: 0, balanceAfter: -1,
	engagements: [
		{ engagementId: 'e1', kind: 'postpartum', status: 'complete', createdAt: '2025-04-02T10:00:00Z' },
		{ engagementId: 'e2', kind: 'birth', status: 'active', createdAt: '2026-01-17T10:00:00Z' }
	] as ApprovalDetail['engagements']
};

// The variants that isolate which token actually breaks the page: only the
// URL does. A browser breaks on `-` and `@`, so neither the long name nor
// the long email is hostile at all.
const hostile: ApprovalDetail = {
	...real,
	note: 'See https://portal.highland-midwifery-group.example.org/referrals/2027/persephone-vandenberghe-okonkwo?source=intake',
	requestedByName: 'Anastasia.Wolstenholme-Fitzgerald@highland-midwifery-group.example.org',
	client: { clientId: 'client-1', givenName: 'Persephone', familyName: 'Vandenberghe-Okonkwo-Featherstonehaugh', preferredName: '', isNewToPractice: false }
};
const longNameOnly: ApprovalDetail = { ...real, client: { ...real.client, familyName: 'Vandenberghe-Okonkwo-Featherstonehaugh' } };
const longEmailOnly: ApprovalDetail = { ...real, requestedByName: 'Anastasia.Wolstenholme-Fitzgerald@highland-midwifery-group.example.org' };
const longUrlOnly: ApprovalDetail = { ...real, note: 'See https://portal.highland-midwifery-group.example.org/referrals/2027/persephone?source=intake' };

beforeEach(() => {
	apiFetchWithSession.mockReset();
	goto.mockReset();
	sessionStorage.clear();
});

function describeElement(el: Element): string {
	const tag = el.tagName.toLowerCase();
	const cls = typeof el.className === 'string' && el.className ? '.' + el.className.trim().split(/\s+/).join('.') : '';
	const text = (el.textContent ?? '').trim().slice(0, 40).replace(/\s+/g, ' ');
	return `${tag}${cls} "${text}"`;
}

async function mount(detail: ApprovalDetail) {
	if (!customElements.get('stack-l')) registerLayoutPrimitives();
	document.body.innerHTML = '';
	apiFetchWithSession.mockImplementation((path: string) => {
		if (path.endsWith('/approve')) return Promise.resolve(jsonResponse({ requestId: 'r', engagementId: 'e', state: 'approved' }, 200));
		if (path.endsWith('/refuse')) return Promise.resolve(jsonResponse({ requestId: 'r', state: 'refused' }, 200));
		return Promise.resolve(jsonResponse(detail, 200));
	});
	await render(Page, {});
	await expect.element(testPage.getByRole('heading', { level: 1 })).toBeVisible();
	document.body.style.margin = '0';
}

/** The continuum sweep: 1400px to 280px, one pixel at a time. */
async function sweep(detail: ApprovalDetail, label: string) {
	await mount(detail);
	const body = document.body;
	const firstSeen = new Map<string, number>();
	const lines: string[] = [];
	for (let w = 1400; w >= 280; w--) {
		body.style.width = `${w}px`;
		void body.offsetWidth;
		const bodyLeft = body.getBoundingClientRect().left;
		for (const el of body.querySelectorAll('*')) {
			const rect = el.getBoundingClientRect();
			const rightOverflow = rect.right - bodyLeft - w;
			const scrollOverflow = el.scrollWidth - el.clientWidth;
			if (rightOverflow > 1 || scrollOverflow > 1) {
				const key = describeElement(el);
				if (!firstSeen.has(key)) {
					firstSeen.set(key, w);
					lines.push(`  ${w}px  overflow=${Math.max(rightOverflow, scrollOverflow).toFixed(0)}px  ${key}`);
				}
			}
		}
	}
	const report = `### ${label}\n` + (lines.length === 0 ? '  no element overflowed at any width from 1400 down to 280' : lines.slice(0, 40).join('\n'));
	expect(report).toBe('REPORT');
}

/** How far past the edge, at the widths a person would actually ask about. */
async function magnitudes(detail: ApprovalDetail, label: string) {
	await mount(detail);
	const out: Record<string, string> = {};
	for (const w of [400, 360, 320, 300, 280]) {
		document.body.style.width = `${w}px`;
		void document.body.offsetWidth;
		const bodyLeft = document.body.getBoundingClientRect().left;
		let worst = 0;
		let worstElement = 'none';
		for (const el of document.body.querySelectorAll('*')) {
			const over = el.getBoundingClientRect().right - bodyLeft - w;
			if (over > worst) { worst = over; worstElement = describeElement(el); }
		}
		out[`${w}px`] = `${worst.toFixed(0)}px past the edge -- ${worstElement}`;
	}
	expect({ label, ...out }).toBe('MAGNITUDES');
}

it('sweeps modest content', async () => { await sweep(modest, 'modest fixture content'); });
it('sweeps realistic content', async () => { await sweep(real, 'realistic longer content'); });
it('sweeps hostile unbroken tokens', async () => { await sweep(hostile, 'hostile unbroken tokens (URL, email, long name)'); });

it('magnitudes: realistic', async () => { await magnitudes(real, 'realistic longer content'); });
it('magnitudes: hostile', async () => { await magnitudes(hostile, 'hostile unbroken tokens'); });
it('magnitudes: long name only', async () => { await magnitudes(longNameOnly, 'long client name only'); });
it('magnitudes: long email only', async () => { await magnitudes(longEmailOnly, 'long requester email only'); });
it('magnitudes: long url only', async () => { await magnitudes(longUrlOnly, 'long URL in the note only'); });

/** The wide end: does the layout ever take a second configuration? */
it('wide-end shape', async () => {
	await mount(real);
	const out: Record<string, string> = {};
	for (const w of [1600, 1280, 900, 600, 400, 320]) {
		document.body.style.width = `${w}px`;
		void document.body.offsetWidth;
		const dl = document.querySelector('dl')!;
		const dd = dl.querySelector('dd')!;
		const textarea = document.querySelector('textarea')!;
		out[`${w}px`] = `dl=${Math.round(dl.getBoundingClientRect().width)} cols=${getComputedStyle(dl).gridTemplateColumns} dd=${Math.round(dd.getBoundingClientRect().width)} textarea=${Math.round(textarea.getBoundingClientRect().width)}`;
	}
	expect(out).toBe('WIDE');
});

/** Proves the sweep can see a break, and that the CSS and primitives are live. */
it('harness sanity: CSS applied, primitives live, and a real break is detected', async () => {
	await mount(real);
	document.body.style.width = '320px';
	const probe = document.createElement('div');
	probe.style.minWidth = '700px';
	probe.textContent = 'probe';
	document.querySelector('section')!.append(probe);
	void document.body.offsetWidth;
	const found: string[] = [];
	const bodyLeft = document.body.getBoundingClientRect().left;
	for (const el of document.body.querySelectorAll('*')) {
		const r = el.getBoundingClientRect();
		if (r.right - bodyLeft - 320 > 1 || el.scrollWidth - el.clientWidth > 1) found.push(describeElement(el));
	}
	expect({
		dlDisplay: getComputedStyle(document.querySelector('dl')!).display,
		dlColumns: getComputedStyle(document.querySelector('dl')!).gridTemplateColumns,
		centreWidth: getComputedStyle(document.querySelector('center-l')!).maxInlineSize,
		containerType: getComputedStyle(document.querySelector('container-l')!).containerType,
		probeDetected: found.length
	}).toBe('DIAG');
});
