import { afterEach, describe, expect, it, vi } from 'vitest';
import { US_TIMEZONES, detectTimezone, timezoneOptions } from './timezones.js';

describe('the zones a person is offered (#1166)', () => {
	it('offers seven US zones, each a real IANA name', () => {
		expect(US_TIMEZONES).toHaveLength(7);
		for (const zone of US_TIMEZONES) {
			// A name the platform's own zone database resolves -- the same
			// database `ianazone` checks against in the BFF, so the list
			// cannot drift into offering something the API refuses.
			expect(() => new Intl.DateTimeFormat('en-US', { timeZone: zone.value })).not.toThrow();
		}
	});

	it('names Arizona separately from the rest of Mountain time', () => {
		// Phoenix does not observe daylight saving, so for most of the year
		// it and Denver disagree about the hour -- which moves the boundary
		// an evening Visit is typed against.
		const values = US_TIMEZONES.map((zone) => zone.value);
		expect(values).toContain('America/Phoenix');
		expect(values).toContain('America/Denver');
	});
});

describe('timezoneOptions', () => {
	it('leaves the list alone for a zone it already carries', () => {
		expect(timezoneOptions('America/Denver')).toEqual(US_TIMEZONES);
	});

	it('leaves the list alone when nothing is chosen yet', () => {
		expect(timezoneOptions('')).toEqual(US_TIMEZONES);
	});

	it('adds the zone a Practice actually holds when the seven do not carry it', () => {
		// Without this, the select renders blank for a Practice stored in
		// Indianapolis and silently rewrites her zone on the next save.
		const options = timezoneOptions('America/Indiana/Indianapolis');

		expect(options).toHaveLength(US_TIMEZONES.length + 1);
		expect(options.at(-1)).toEqual({
			value: 'America/Indiana/Indianapolis',
			label: 'America/Indiana/Indianapolis'
		});
	});
});

describe('detectTimezone', () => {
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('reports the zone the browser resolves', () => {
		expect(detectTimezone()).toBe(new Intl.DateTimeFormat().resolvedOptions().timeZone);
	});

	it('reports nothing rather than throwing when the browser will not say', () => {
		vi.stubGlobal('Intl', {
			DateTimeFormat: class {
				resolvedOptions() {
					return {};
				}
			}
		});

		expect(detectTimezone()).toBe('');
	});
});
