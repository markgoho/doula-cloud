import type { BeforeNavigate } from '$app/navigation';
import { describe, expect, it, vi } from 'vitest';
import { reloadOnceOnPreloadError, skewReloadHref, VERSION_POLL_MS } from './versionSkew.js';

const to = { url: new URL('https://doula.cloud/portal/clients') } as BeforeNavigate['to'];

describe('skewReloadHref', () => {
	it('names the target when a new version is out and the navigation is in-app', () => {
		expect(skewReloadHref(true, { willUnload: false, to })).toBe('https://doula.cloud/portal/clients');
	});

	it.each([
		['no new version', false, { willUnload: false, to }],
		['the page is already unloading', true, { willUnload: true, to }],
		['there is no destination', true, { willUnload: false, to: undefined as never }]
	])('leaves the navigation alone when %s', (_name, updated, navigation) => {
		expect(skewReloadHref(updated, navigation)).toBeUndefined();
	});

	it('polls once a minute', () => {
		expect(VERSION_POLL_MS).toBe(60_000);
	});
});

function fakeWindow(storage: Storage | undefined) {
	const listeners = new Map<string, (event: Event) => void>();
	const reload = vi.fn();
	const win = {
		addEventListener: (type: string, listener: (event: Event) => void) => listeners.set(type, listener),
		location: { reload } as unknown as Location,
		sessionStorage: storage
	};
	const fire = () => {
		const event = new Event('vite:preloadError', { cancelable: true });
		listeners.get('vite:preloadError')?.(event);
		return event;
	};
	return { win, reload, fire };
}

describe('reloadOnceOnPreloadError', () => {
	it('reloads once, cancels the error, and does not reload again within the guard window', () => {
		const store = new Map<string, string>();
		const storage = {
			getItem: (k: string) => store.get(k) as string,
			setItem: (k: string, v: string) => void store.set(k, v)
		} as Storage;
		const { win, reload, fire } = fakeWindow(storage);
		let clock = 1_000_000;
		reloadOnceOnPreloadError(win, () => clock);

		expect(fire().defaultPrevented).toBe(true);
		expect(reload).toHaveBeenCalledTimes(1);

		clock += 5000;
		fire();
		expect(reload).toHaveBeenCalledTimes(1);

		clock += 60_000;
		fire();
		expect(reload).toHaveBeenCalledTimes(2);
	});

	it('reloads when storage is unavailable on the window', () => {
		const { win, reload, fire } = fakeWindow(undefined);
		reloadOnceOnPreloadError(win);
		fire();
		expect(reload).toHaveBeenCalledTimes(1);
	});

	it('does not reload when storage throws, since the loop cannot be guarded', () => {
		const storage = {
			getItem: () => {
				throw new Error('blocked');
			}
		} as unknown as Storage;
		const { win, reload, fire } = fakeWindow(storage);
		reloadOnceOnPreloadError(win);
		fire();
		expect(reload).not.toHaveBeenCalled();
	});
});
