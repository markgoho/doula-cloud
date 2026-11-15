import { describe, expect, it } from 'vitest';
import { headingOrder, type ColumnOrder } from './headingOrder.js';

const orders: ColumnOrder[] = [
	{ order: '', column: 'Date', direction: 'ascending' },
	{ order: 'name', column: 'Name', direction: 'ascending' },
	{ order: 'name-desc', column: 'Name', direction: 'descending' },
	{ order: 'owed', column: 'Money', direction: 'descending' }
];

describe('headingOrder', () => {
	it("asks for the column's first order when the rows are in order by another column", () => {
		expect(headingOrder(orders, { column: 'Date', direction: 'ascending' }, 'Name')).toBe('name');
		expect(headingOrder(orders, undefined, 'Money')).toBe('owed');
	});

	it('asks for the opposite direction when the rows are already in order by the column', () => {
		expect(headingOrder(orders, { column: 'Name', direction: 'ascending' }, 'Name')).toBe('name-desc');
		expect(headingOrder(orders, { column: 'Name', direction: 'descending' }, 'Name')).toBe('name');
	});

	it('asks for nothing when the column has no order to move to', () => {
		expect(headingOrder(orders, { column: 'Money', direction: 'descending' }, 'Money')).toBeUndefined();
		expect(headingOrder(orders, { column: 'Date', direction: 'ascending' }, 'Status')).toBeUndefined();
	});

	it("keeps the empty order, the list's own, distinct from none", () => {
		expect(headingOrder(orders, { column: 'Name', direction: 'ascending' }, 'Date')).toBe('');
	});
});
