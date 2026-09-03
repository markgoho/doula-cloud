import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { cleanup, render } from 'vitest-browser-svelte';
import DataTable from './DataTable.svelte';

/*
 * What one row is allowed to cost -- wayfinder #418, ADR-0020.
 *
 * The brief's "a dense list stays at 60fps" cannot be measured here: a
 * probe on #418 found headless Chromium reports a fixed ~8.3ms frame no
 * matter what it renders, so a frame-rate assertion always passes. What it
 * does report honestly is how many elements a row costs, and how many
 * times the row array itself gets read while mounting -- both exactly
 * deterministic, because both are facts about space rather than time.
 *
 * #489: this file used to also assert a mount-time ratio (800 rows takes
 * under Nx as long as 200), on the reasoning that per-row work which reads
 * the rest of the list -- a sort, a lookup, an `indexOf` -- turns 4x the
 * rows into 16x the time. That assertion was CI's last wall-clock
 * measurement and flaked (6.66, 6.67, 6.57, 7.19 against thresholds of 6
 * then 10, all on commits that touched no front-end code), which is
 * exactly the failure mode ADR-0020 already diagnosed and fixed everywhere
 * else in this file. The same failure it was catching -- a column reading
 * the rest of the list per row -- has a countable signature: it reads the
 * row array more times than there are rows. Wrapping `rows` in a
 * `Proxy` that counts `get` traps reports that count with zero variance
 * (measured at 200/400/800 rows: exactly 2 reads per row, every run), and
 * a deliberately quadratic accessor blows the same bound by two orders of
 * magnitude -- see the last test below. See ADR-0020's amendment for the
 * measured CI spread this replaced.
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
	await render(DataTable<Row>, { columns, rows: caseload(rows), emptyMessage: 'No Clients yet.' });
	// #508: the record view now carries the same text in a <dd>, so two
	// elements match the last row's name -- `.first()` is the <td>, which
	// is what "on screen" means for this measurement's own table below.
	await page.getByText(`Client Number ${rows - 1}`).first().element();

	const table = document.querySelector('table');
	if (!table) throw new Error('DataTable rendered no table');
	const elements = table.querySelectorAll('*').length;

	cleanup();
	return { elements };
}

/**
The elements one extra row adds, isolated from the table's fixed header cost.
*/
async function costPerRow(smaller: number, larger: number) {
	const small = await setup({ rows: smaller });
	const large = await setup({ rows: larger });
	return { elements: (large.elements - small.elements) / (larger - smaller) };
}

/**
 * Wraps a fresh caseload in a `Proxy` that counts every `get` trap --
 * every index read, every `.length` check, anything a column's accessor
 * or `DataTable` itself does to the array. A row's own fields are plain
 * objects underneath, so this counts reads of the *list*, not of a row.
 */
function countedRows(count: number) {
	const source = caseload(count);
	let gets = 0;
	const rows = new Proxy(source, {
		get(target, property, receiver) {
			gets++;
			return Reflect.get(target, property, receiver);
		}
	});
	return { rows, reads: () => gets };
}

/**
 * A column whose accessor reads the rest of the list before returning a
 * value -- the shape this gate exists to catch: a sort, a lookup, an
 * `indexOf` done once per row instead of once per render.
 */
function columnsReadingTheWholeList(rows: Row[]) {
	return [
		{ label: 'Name', accessor: (row: Row) => (rows.indexOf(row), row.name) },
		...columns.slice(1)
	];
}

/**
 * How many times the row array was read while mounting a caseload of this size.
 */
async function readsToRender(rows: number, makeColumns: (rows: Row[]) => typeof columns = () => columns) {
	const { rows: proxied, reads } = countedRows(rows);
	await render(DataTable<Row>, { columns: makeColumns(proxied), rows: proxied, emptyMessage: 'No Clients yet.' });
	await page.getByText(`Client Number ${rows - 1}`).first().element();
	const count = reads();
	cleanup();
	return count;
}

/**
 * The reads one extra row adds, isolated from the table's fixed reads.
 */
async function readsPerRow(smaller: number, larger: number, makeColumns?: (rows: Row[]) => typeof columns) {
	const small = await readsToRender(smaller, makeColumns);
	const large = await readsToRender(larger, makeColumns);
	return (large - small) / (larger - smaller);
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

describe('a row reads the list itself a fixed, small number of times', () => {
	it('reads the row array no more than a small constant number of times per row', async () => {
		const reads = await readsPerRow(200, 800);
		// Measured at exactly 2 per row, with zero variance, at 200/400/800
		// rows: the outer `{#each rows as row}` block indexes into the
		// array once for the table view and once for the record view
		// (ADR-0024). Four leaves headroom for a legitimate extra read
		// without coming near the hundreds a per-row scan of the list
		// would add -- see the last test in this block.
		expect(reads).toBeLessThanOrEqual(4);
	});

	it('reads the array the same number of times whether the list is short or long', async () => {
		const shortList = await readsPerRow(100, 200);
		const longList = await readsPerRow(200, 400);
		// Exact, not approximate, for the same reason as the element count
		// above: reading is one pass per row, so any difference means a
		// read is happening per render rather than per row.
		expect(longList).toBe(shortList);
	});

	it('catches a column whose accessor reads the rest of the list per row', async () => {
		// Proof, not assumption (#489): this is the failure the gate above
		// exists to catch. A column that scans the whole array per row --
		// `indexOf`, `find`, a resort -- reads the list roughly n times per
		// row instead of a constant, which blows the bound above by two
		// orders of magnitude at these sizes.
		const reads = await readsPerRow(200, 800, columnsReadingTheWholeList);
		expect(reads).toBeGreaterThan(50);
	});
});
