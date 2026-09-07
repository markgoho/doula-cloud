import { describe, expect, it, vi } from 'vitest';

import { SectionState } from './sectionState.svelte.js';

describe('SectionState', () => {
	it('starts with the given value, no error, not busy', () => {
		const section = new SectionState('idle');

		expect(section.value).toBe('idle');
		expect(section.error).toBe('');
		expect(section.isBusy).toBe(false);
	});

	describe('load', () => {
		it('stores what the loader resolves to', async () => {
			const section = new SectionState<string | undefined>(undefined);

			const succeeded = await section.load(async () => 'loaded', 'Failed to load');

			expect(succeeded).toBe(true);
			expect(section.value).toBe('loaded');
			expect(section.error).toBe('');
			expect(section.isBusy).toBe(false);
		});

		it("surfaces a thrown Error's own message", async () => {
			const section = new SectionState('kept');

			const succeeded = await section.load(async () => {
				throw new Error('The Engagement was not found');
			}, 'Failed to load');

			expect(succeeded).toBe(false);
			expect(section.error).toBe('The Engagement was not found');
			// A failed load leaves the previous value in place rather than
			// clobbering it with nothing.
			expect(section.value).toBe('kept');
		});

		// So a rejected non-Error never renders an empty error box.
		it('falls back to the failure message when something else is thrown', async () => {
			const section = new SectionState('kept');

			await section.load(async () => {
				throw 'offline';
			}, 'Failed to load the section');

			expect(section.error).toBe('Failed to load the section');
		});

		it('clears a previous error when a new load starts', async () => {
			const section = new SectionState('kept');
			await section.load(async () => {
				throw new Error('temporary');
			}, 'Failed to load');
			expect(section.error).toBe('temporary');

			await section.load(async () => 'recovered', 'Failed to load');

			expect(section.error).toBe('');
			expect(section.value).toBe('recovered');
		});

		it('is busy only while the loader is in flight', async () => {
			const held = Promise.withResolvers<string>();
			const section = new SectionState<string | undefined>(undefined);

			const inFlight = section.load(() => held.promise, 'Failed to load');
			expect(section.isBusy).toBe(true);

			held.resolve('done');
			await inFlight;

			expect(section.isBusy).toBe(false);
		});

		it('does not guard against a second call while one is in flight', async () => {
			const held = Promise.withResolvers<string>();
			const loader = vi.fn().mockReturnValueOnce(held.promise).mockResolvedValueOnce('second');
			const section = new SectionState<string | undefined>(undefined);

			const first = section.load(loader, 'Failed to load');
			await section.load(loader, 'Failed to load');

			expect(loader).toHaveBeenCalledTimes(2);
			expect(section.value).toBe('second');

			held.resolve('first');
			await first;
		});
	});

	describe('mutate', () => {
		it('stores what the mutator resolves to', async () => {
			const section = new SectionState('before');

			const succeeded = await section.mutate(async () => 'after', 'Failed to save');

			expect(succeeded).toBe(true);
			expect(section.value).toBe('after');
		});

		it("surfaces a thrown Error's own message, keeping the previous value", async () => {
			const section = new SectionState('before');

			const succeeded = await section.mutate(async () => {
				throw new Error('The Contract could not be saved');
			}, 'Failed to save');

			expect(succeeded).toBe(false);
			expect(section.error).toBe('The Contract could not be saved');
			expect(section.value).toBe('before');
		});

		it('falls back to the failure message when something else is thrown', async () => {
			const section = new SectionState('before');

			await section.mutate(async () => {
				throw 'offline';
			}, 'Failed to save the section');

			expect(section.error).toBe('Failed to save the section');
		});

		it('is busy only while the mutator is in flight', async () => {
			const held = Promise.withResolvers<string>();
			const section = new SectionState('before');

			const inFlight = section.mutate(() => held.promise, 'Failed to save');
			expect(section.isBusy).toBe(true);

			held.resolve('after');
			await inFlight;

			expect(section.isBusy).toBe(false);
		});

		// A double click on "Save" is one request, not two -- the same
		// guard PaginatedList.loadMore makes.
		it('does nothing and reports failure when one is already in flight', async () => {
			const held = Promise.withResolvers<string>();
			const mutator = vi.fn().mockReturnValue(held.promise);
			const section = new SectionState('before');

			const first = section.mutate(mutator, 'Failed to save');
			const second = await section.mutate(mutator, 'Failed to save');

			expect(second).toBe(false);
			expect(mutator).toHaveBeenCalledTimes(1);

			held.resolve('after');
			await first;
		});

		it('clears a previous error when the next mutate starts', async () => {
			const section = new SectionState('before');
			await section.mutate(async () => {
				throw new Error('temporary');
			}, 'Failed to save');
			expect(section.error).toBe('temporary');

			await section.mutate(async () => 'after', 'Failed to save');

			expect(section.error).toBe('');
			expect(section.value).toBe('after');
		});
	});
});
