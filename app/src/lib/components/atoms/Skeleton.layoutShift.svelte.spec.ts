import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
// DataTable's frame needs stack-l's display:block default (primitives.css)
// to work as a container-query context -- see DataTable.svelte.spec.ts.
import '#lib/styles/app.css';
import DataTable from '../organisms/DataTable.svelte';
import Skeleton from './Skeleton.svelte';

/*
 * The outcome half of #418's smoothness gate -- ADR-0020.
 *
 * Everything else in this ticket enforces a cause. This measures the
 * result: that swapping a skeleton for the content it stood in for moves
 * nothing. It is the one place `layout-shift` can be observed honestly,
 * because a shift is a fact about space, not about time -- unlike frame
 * rate or interaction latency, it reads the same on a contended CI runner
 * as on an idle laptop.
 *
 * This deliberately does not measure a whole route. Route-level pop-in is
 * fixed by giving each route a loading state, not by observing that it
 * has one, and a route needs the SvelteKit runtime that browser-mode
 * tests do not have. See ADR-0020 for what is not checked and why.
 */

interface Row {
	name: string;
	status: string;
}

const columns = [
	{ label: 'Name', accessor: (row: Row) => row.name },
	{ label: 'Status', accessor: (row: Row) => row.status }
];

function caseload(count: number): Row[] {
	return Array.from({ length: count }, (_, index) => ({
		name: `Client Number ${index}`,
		status: 'Active'
	}));
}

/**
 * The height each of the two states settles at, measured from its own
 * bounding box: first the placeholder, then the content it stood in for.
 */
async function setup({ rows = 6 }: { rows?: number } = {}) {
	// DataTable's own content floor (#508) stacks it into a <dl> below
	// 44rem, and this measures the <table> layout specifically.
	await page.viewport(1440, 900);
	await render(Skeleton, { variant: 'row', lines: rows, label: 'Loading Clients' });
	const skeleton = await page.getByRole('status').element();
	const placeholder = skeleton.getBoundingClientRect().height;
	cleanup();

	await render(DataTable<Row>, { columns, rows: caseload(rows), emptyMessage: 'No Clients yet.' });
	const table = await page.getByRole('table').element();
	const loaded = table.getBoundingClientRect().height;
	cleanup();

	return { placeholder, loaded };
}

describe('a skeleton reserves the space its content will take', () => {
	it('stands in for a six-row list without changing the page height by more than one row', async () => {
		const { placeholder, loaded } = await setup({ rows: 6 });
		// One row of tolerance, not zero: the table carries a header the
		// skeleton does not, and a caller composes the two with the same
		// heading above them. +1 past that for sub-pixel font rounding,
		// the same margin continuum.ts's own sweep gives itself. What this
		// rejects is the shape that actually jumps -- a placeholder sized
		// by guess rather than by row count.
		expect(Math.abs(loaded - placeholder)).toBeLessThanOrEqual(41);
	});

	it('grows with the row count rather than staying a fixed block', async () => {
		const small = await setup({ rows: 3 });
		const large = await setup({ rows: 12 });
		expect(large.placeholder).toBeGreaterThan(small.placeholder);
		expect((large.placeholder - small.placeholder) / (large.loaded - small.loaded)).toBeCloseTo(1, 1);
	});
});
