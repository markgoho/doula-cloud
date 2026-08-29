import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import DataTable from './DataTable.svelte';

/*
 * What one row is allowed to cost -- wayfinder #418, ADR-0020.
 *
 * The brief's "a dense list stays at 60fps" cannot be measured here: a
 * probe on #418 found headless Chromium reports a fixed ~8.3ms frame no
 * matter what it renders, so a frame-rate assertion always passes. Two
 * things it does report honestly are how many elements a row costs, which
 * is exactly deterministic, and how mount time *scales* with row count,
 * which is machine-independent in a way an absolute millisecond budget on
 * a shared CI runner is not.
 *
 * `DataTable.usage.spec.ts` holds the other half -- that no route hands
 * this component an unbounded number of rows in the first place.
 */

interface Row {
	name: string;
	status: string;
	due: string;
	owner: string;
}

interface SetupOptions {
	rows?: number;
}

const columns = [
	{ label: 'Name', accessor: (row: Row) => row.name },
	{ label: 'Status', accessor: (row: Row) => row.status },
	{ label: 'Due', accessor: (row: Row) => row.due },
	{ label: 'Owner', accessor: (row: Row) => row.owner }
];

function caseload(count: number): Row[] {
	return Array.from({ length: count }, (_, index) => ({
		name: `Client Number ${index}`,
		status: index % 3 === 0 ? 'Active' : 'Intake',
		due: `2026-0${(index % 9) + 1}-14`,
		owner: `Doula ${index % 14}`
	}));
}

/**
 * Renders a caseload and reports what it cost, once the last row is on
 * screen. Elements are counted inside the table itself, not the document:
 * a measurement that counted the page would fold in the previous mount
 * and the harness's own chrome, and stop being a per-row number at all.
 */
async function setup({ rows = 350 }: SetupOptions = {}) {
	const started = performance.now();
	await render(DataTable<Row>, { columns, rows: caseload(rows), emptyMessage: 'No Clients yet.' });
	await page.getByText(`Client Number ${rows - 1}`).element();
	const milliseconds = performance.now() - started;

	const table = document.querySelector('table');
	if (!table) throw new Error('DataTable rendered no table');
	const elements = table.querySelectorAll('*').length;

	cleanup();
	return { milliseconds, elements };
}

/**
The elements one extra row adds, isolated from the table's fixed header cost.
*/
async function costPerRow(smaller: number, larger: number) {
	const small = await setup({ rows: smaller });
	const large = await setup({ rows: larger });
	return {
		elements: (large.elements - small.elements) / (larger - smaller),
		slowdown: large.milliseconds / small.milliseconds
	};
}

describe('a row costs a fixed, small amount of page', () => {
	it('adds no more than six elements per row', async () => {
		const { elements } = await costPerRow(200, 800);
		expect(elements).toBeLessThanOrEqual(6);
	});

	it('adds the same number of elements whether the list is short or long', async () => {
		const shortList = await costPerRow(100, 200);
		const longList = await costPerRow(200, 400);
		// Exact, not approximate. Rendering is one pass per row, so any
		// difference means something is emitted per render rather than per
		// row -- the shape that stops scaling.
		expect(longList.elements).toBe(shortList.elements);
	});
});

describe('mount cost stays linear in the number of rows', () => {
	it('takes under six times as long for four times the rows', async () => {
		const { slowdown } = await costPerRow(200, 800);
		// Deliberately not a millisecond budget: a constant-factor slowdown
		// is the element budget's job, and an absolute threshold measures
		// whatever else is on the runner. What this catches is per-row work
		// that reads the rest of the list -- a sort, a lookup, an indexOf --
		// which turns 4x the rows into 16x the time and is what actually
		// makes a long list stutter.
		expect(slowdown).toBeLessThan(6);
	});
});
