import { describe, expect, it } from 'vitest';
import { isErased, isLive, isTombstone, type ClientStanding } from './clientStanding.js';

describe('the standing questions', () => {
	it.each<[ClientStanding, boolean, boolean, boolean]>([
		['live', true, false, false],
		['merged', false, false, true],
		['erased', false, true, false],
		['merged_then_erased', false, true, true]
	])('answers %s as live %s, erased %s, tombstone %s', (standing, live, erased, tombstone) => {
		expect(isLive({ standing })).toBe(live);
		expect(isErased({ standing })).toBe(erased);
		expect(isTombstone({ standing })).toBe(tombstone);
	});

	it('answers a record not yet read as none of them', () => {
		expect(isLive(undefined)).toBe(false);
		expect(isErased(undefined)).toBe(false);
		expect(isTombstone(undefined)).toBe(false);
	});
});
