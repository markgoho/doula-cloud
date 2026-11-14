import { describe, expect, it } from 'vitest';
import { rowLinkLabeler } from './rowLinkLabel';

const name = (row: { who: string; at: string }) => row.who;
const detail = (row: { who: string; at: string }) => row.at;

describe('rowLinkLabeler', () => {
	it('names a row whose name no other row shares by the name alone', () => {
		const rows = [
			{ who: 'Quinn Dunmore', at: 'Sat 2pm' },
			{ who: 'Avery Estrada', at: 'Sun 9am' }
		];
		const label = rowLinkLabeler(rows, name, detail);
		expect(rows.map((row) => label(row))).toEqual(['Quinn Dunmore', 'Avery Estrada']);
	});

	it('adds the detail to every row of a name more than one row shares', () => {
		const rows = [
			{ who: 'Quinn Dunmore', at: 'Sat 2pm' },
			{ who: 'Avery Estrada', at: 'Sun 9am' },
			{ who: 'Quinn Dunmore', at: 'Mon 10am' }
		];
		const label = rowLinkLabeler(rows, name, detail);
		expect(rows.map((row) => label(row))).toEqual(['Quinn Dunmore, Sat 2pm', 'Avery Estrada', 'Quinn Dunmore, Mon 10am']);
	});

	it('gains the detail when a later row brings a second row of the name', () => {
		const first = { who: 'Quinn Dunmore', at: 'Sat 2pm' };
		const more = { who: 'Quinn Dunmore', at: 'Mon 10am' };
		expect(rowLinkLabeler([first], name, detail)(first)).toBe('Quinn Dunmore');
		expect(rowLinkLabeler([first, more], name, detail)(first)).toBe('Quinn Dunmore, Sat 2pm');
	});
});
