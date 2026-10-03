import { beforeEach, describe, expect, it, vi } from 'vitest';
import { goto } from '$app/navigation';
import { gotoWithOutcome, takeOutcome } from './outcome.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

const ORIGIN = 'http://localhost';

function at(path: string): URL {
	return new URL(path, ORIGIN);
}

// #1710: a save that leaves the screen says what it did on the screen it
// lands on. The destination reads the message while it renders, which is
// while `goto` is still running.
describe('gotoWithOutcome and takeOutcome (#1710)', () => {
	beforeEach(() => {
		vi.mocked(goto).mockReset();
	});

	it('navigates to the href', async () => {
		await gotoWithOutcome('/practices/p/clients/c', 'Sam saved.');

		expect(goto).toHaveBeenCalledWith('/practices/p/clients/c');
	});

	it('gives the message to the screen it lands on, during the navigation', async () => {
		let arrived: string | undefined;
		vi.mocked(goto).mockImplementation(async () => {
			arrived = takeOutcome(at('/practices/p/clients/c'));
		});

		await gotoWithOutcome('/practices/p/clients/c', 'Sam saved.');

		expect(arrived).toBe('Sam saved.');
	});

	it('reads the path only, so a query on either side does not matter', async () => {
		let arrived: string | undefined;
		vi.mocked(goto).mockImplementation(async () => {
			arrived = takeOutcome(at('/practices/p/clients/c?tab=1'));
		});

		await gotoWithOutcome('/practices/p/clients/c?from=edit', 'Sam saved.');

		expect(arrived).toBe('Sam saved.');
	});

	it('gives the message once: a second read during the same navigation is empty', async () => {
		let second: string | undefined = 'unread';
		vi.mocked(goto).mockImplementation(async () => {
			takeOutcome(at('/practices/p/clients/c'));
			second = takeOutcome(at('/practices/p/clients/c'));
		});

		await gotoWithOutcome('/practices/p/clients/c', 'Sam saved.');

		expect(second).toBeUndefined();
	});

	it('gives nothing to a screen the save was not sent to', async () => {
		let elsewhere: string | undefined = 'unread';
		vi.mocked(goto).mockImplementation(async () => {
			elsewhere = takeOutcome(at('/practices/p/clients/other'));
		});

		await gotoWithOutcome('/practices/p/clients/c', 'Sam saved.');

		expect(elsewhere).toBeUndefined();
	});

	it('keeps nothing once the navigation ends, so a later visit is silent', async () => {
		await gotoWithOutcome('/practices/p/clients/c', 'Sam saved.');

		expect(takeOutcome(at('/practices/p/clients/c'))).toBeUndefined();
	});

	it('keeps nothing when the navigation fails', async () => {
		vi.mocked(goto).mockRejectedValue(new Error('navigation aborted'));

		await expect(gotoWithOutcome('/practices/p/clients/c', 'Sam saved.')).rejects.toThrow('navigation aborted');

		expect(takeOutcome(at('/practices/p/clients/c'))).toBeUndefined();
	});

	it('gives nothing on a visit no save sent', () => {
		expect(takeOutcome(at('/practices/p/clients/c'))).toBeUndefined();
	});
});
