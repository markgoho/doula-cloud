/*
 * THROWAWAY measurement harness, wayfinder #551 (the reach trial), against
 * the screen a fresh session shipped for #265. It is #521's harness with
 * only the route and the fixtures changed: same 1px sweep, same
 * getBoundingClientRect/scrollWidth overflow test, same
 * report-by-failing-assertion, same planted-700px sanity case.
 *
 * Run: bunx vitest --run --project client <this file>
 */
import { beforeEach, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import type { PracticeInvoicePage, PracticeInvoice } from '#lib/invoice.js';
import Page from './+page.svelte';

vi.mock('$app/state', () => ({ page: { params: { practiceId: 'practice-1' } } }));
const apiFetchWithSession = vi.hoisted(() => vi.fn());
vi.mock('#lib/api.js', () => ({ apiFetchWithSession }));

function row(over: Partial<PracticeInvoice>): PracticeInvoice {
	return {
		id: 'in_1', contractId: 'c1', engagementId: 'e1', clientName: 'Mara Quinn',
		status: 'open', amountCents: 120000, currency: 'usd',
		createdAt: '2026-08-01T10:00:00Z', ...over
	};
}

const modest: PracticeInvoicePage = {
	items: [row({}), row({ id: 'in_2', clientName: 'Ada Reyes', status: 'paid', paidAt: '2026-08-09T10:00:00Z' })],
	hasMore: false, outstandingCents: 120000, outstandingCount: 1, paidCents: 90000
};

// #537's vocabulary: the longest realistic value a Practice actually types.
const realistic: PracticeInvoicePage = {
	...modest,
	items: [
		row({ clientName: 'Persephone Vandenberghe-Okonkwo' }),
		row({ id: 'in_2', clientName: 'Anastasia Wolstenholme-Fitzgerald', status: 'uncollectible', amountCents: 485000 })
	],
	outstandingCents: 485000, outstandingCount: 2, paidCents: 1260000
};

const longNameOnly: PracticeInvoicePage = {
	...modest,
	items: [row({ clientName: 'Persephone Vandenberghe-Okonkwo-Featherstonehaugh' })]
};

function describeElement(el: Element): string {
	const tag = el.tagName.toLowerCase();
	const cls = typeof el.className === 'string' && el.className ? '.' + el.className.trim().split(/\s+/).join('.') : '';
	const text = (el.textContent ?? '').trim().slice(0, 40).replace(/\s+/g, ' ');
	return `${tag}${cls} "${text}"`;
}

async function mount(data: PracticeInvoicePage, waitForFonts: boolean) {
	if (!customElements.get('stack-l')) registerLayoutPrimitives();
	document.body.innerHTML = '';
	await render(Page, { data });
	if (waitForFonts) await document.fonts.ready;
	document.body.style.margin = '0';
}

async function sweep(data: PracticeInvoicePage, label: string, waitForFonts = false) {
	await mount(data, waitForFonts);
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
	expect(`### ${label}\n` + (lines.length === 0 ? '  no element overflowed at any width from 1400 down to 280' : lines.slice(0, 40).join('\n'))).toBe('REPORT');
}

async function magnitudes(data: PracticeInvoicePage, label: string, waitForFonts = false) {
	await mount(data, waitForFonts);
	const out: Record<string, string> = {};
	for (const w of [400, 360, 320, 300, 280]) {
		document.body.style.width = `${w}px`;
		void document.body.offsetWidth;
		const bodyLeft = document.body.getBoundingClientRect().left;
		let worst = 0;
		let worstElement = 'none';
		for (const el of document.body.querySelectorAll('*')) {
			const over = Math.max(el.getBoundingClientRect().right - bodyLeft - w, el.scrollWidth - el.clientWidth);
			if (over > worst) { worst = over; worstElement = describeElement(el); }
		}
		out[`${w}px`] = `${worst.toFixed(0)}px past the edge -- ${worstElement}`;
	}
	expect({ label, ...out }).toBe('MAGNITUDES');
}

beforeEach(() => { apiFetchWithSession.mockReset(); });

it('sweeps modest content', async () => { await sweep(modest, 'modest fixture content'); });
it('sweeps realistic content', async () => { await sweep(realistic, 'realistic longer content'); });
it('sweeps long client name', async () => { await sweep(longNameOnly, 'longest realistic client name'); });
it('sweeps realistic content, fonts ready (#550)', async () => { await sweep(realistic, 'realistic, after document.fonts.ready', true); });

it('magnitudes: realistic', async () => { await magnitudes(realistic, 'realistic longer content'); });
it('magnitudes: realistic, fonts ready', async () => { await magnitudes(realistic, 'realistic, fonts ready', true); });

it('wide-end shape', async () => {
	await mount(realistic, true);
	const out: Record<string, string> = {};
	for (const w of [1600, 1280, 900, 600, 400, 320]) {
		document.body.style.width = `${w}px`;
		void document.body.offsetWidth;
		const dl = document.querySelector('dl');
		const table = document.querySelector('table');
		out[`${w}px`] = `dl=${dl ? Math.round(dl.getBoundingClientRect().width) : 'none'} dlCols=${dl ? getComputedStyle(dl).gridTemplateColumns : 'none'} table=${table ? Math.round(table.getBoundingClientRect().width) : 'none'} tableVisible=${table ? getComputedStyle(table).display : 'none'}`;
	}
	expect(out).toBe('WIDE');
});

it('harness sanity: CSS applied, primitives live, and a real break is detected', async () => {
	await mount(realistic, true);
	document.body.style.width = '320px';
	const probe = document.createElement('div');
	probe.style.minWidth = '700px';
	probe.textContent = 'probe';
	document.body.firstElementChild!.append(probe);
	void document.body.offsetWidth;
	const found: string[] = [];
	const bodyLeft = document.body.getBoundingClientRect().left;
	for (const el of document.body.querySelectorAll('*')) {
		const r = el.getBoundingClientRect();
		if (r.right - bodyLeft - 320 > 1 || el.scrollWidth - el.clientWidth > 1) found.push(describeElement(el));
	}
	expect({
		dlPresent: Boolean(document.querySelector('dl')),
		dlDisplay: document.querySelector('dl') ? getComputedStyle(document.querySelector('dl')!).display : 'none',
		dlColumns: document.querySelector('dl') ? getComputedStyle(document.querySelector('dl')!).gridTemplateColumns : 'none',
		bodyFont: getComputedStyle(document.body).fontFamily,
		probeDetected: found.length
	}).toBe('DIAG');
});

/* Is the screen's safety the componentry, or the fixture being polite?
 * #537's own hostile vocabulary: the one value a browser will not break. */
const hostile: PracticeInvoicePage = {
	...modest,
	items: [
		row({ clientName: 'https://portal.highland-midwifery-group.example.org/clients/persephone-vandenberghe-okonkwo' }),
		row({ id: 'in_2', clientName: 'Vandenberghevanderveldenokonkwofeatherstonehaugh', status: 'paid', paidAt: '2026-08-09T10:00:00Z' })
	]
};
it('sweeps hostile unbroken token', async () => { await sweep(hostile, 'hostile: an unbreakable token in the only free-text column', true); });
it('magnitudes: hostile', async () => { await magnitudes(hostile, 'hostile unbroken token', true); });
