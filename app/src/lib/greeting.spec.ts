import { describe, expect, it } from 'vitest';
import { greetingFor } from './greeting';

/*
 * Each `Date` is built from local parts, and `greetingFor` reads local
 * hours, so these hold in Eastern time on a laptop and in UTC on CI alike.
 */
function at(hour: number, minute = 0): Date {
	return new Date(2026, 9, 2, hour, minute);
}

describe('greetingFor', () => {
	it.each([
		[0, 0, 'Up late? Welcome.'],
		[4, 59, 'Up late? Welcome.'],
		[5, 0, 'Good morning.'],
		[11, 59, 'Good morning.'],
		[12, 0, 'Good afternoon.'],
		[16, 59, 'Good afternoon.'],
		[17, 0, 'Good evening.'],
		[21, 59, 'Good evening.'],
		[22, 0, 'Up late? Welcome.'],
		[23, 59, 'Up late? Welcome.']
	] as const)('greets %i:%i as "%s"', (hour, minute, greeting) => {
		expect(greetingFor(at(hour, minute))).toBe(greeting);
	});

	it('reads the device clock when it is given no time', () => {
		expect(greetingFor()).toBe(greetingFor(new Date()));
	});
});
