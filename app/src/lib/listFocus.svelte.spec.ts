import { flushSync } from 'svelte';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { focusResultAfterChange } from './listFocus.svelte.js';

const navigation = vi.hoisted(() => ({ afterNavigate: vi.fn<(callback: () => void) => void>() }));
vi.mock('$app/navigation', () => navigation);

function setup(options: { isOwnChange?: boolean } = {}) {
	const state = $state({ key: 'a', isLanded: true, isOwnChange: options.isOwnChange ?? false });
	const seed = vi.fn();
	const focus = vi.fn();
	const cleanup = $effect.root(() => {
		focusResultAfterChange({
			seed,
			wantedKey: () => state.key,
			isLanded: () => state.isLanded,
			isOwnChange: () => state.isOwnChange,
			focus
		});
	});
	return { state, seed, focus, navigate, cleanup };
}

const navigate = () => navigation.afterNavigate.mock.calls[0]![0]();

describe('focusing the result line after a change of narrowing', () => {
	beforeEach(() => navigation.afterNavigate.mockReset());

	it('seeds the controls after every navigation and never focuses on first arrival', () => {
		const { seed, focus, navigate, cleanup } = setup();

		navigate();
		flushSync();

		expect(seed).toHaveBeenCalledOnce();
		expect(focus).not.toHaveBeenCalled();
		cleanup();
	});

	it('focuses once the list for the new narrowing has landed', () => {
		const { state, focus, navigate, cleanup } = setup();
		navigate();

		state.key = 'b';
		state.isLanded = false;
		navigate();
		flushSync();
		expect(focus).not.toHaveBeenCalled();

		state.isLanded = true;
		flushSync();
		expect(focus).toHaveBeenCalledOnce();
		cleanup();
	});

	it('leaves focus on the control when the reader\'s own change caused the move', () => {
		const { state, focus, navigate, cleanup } = setup({ isOwnChange: true });
		navigate();

		state.key = 'b';
		navigate();
		flushSync();

		expect(focus).not.toHaveBeenCalled();
		cleanup();
	});

	it('does not focus when the narrowing is the same', () => {
		const { focus, navigate, cleanup } = setup();
		navigate();

		navigate();
		flushSync();

		expect(focus).not.toHaveBeenCalled();
		cleanup();
	});
});
