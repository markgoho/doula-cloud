import type { BeforeNavigate } from '$app/navigation';

const RELOAD_GUARD_KEY = 'dc-skew-reload-at';
const RELOAD_GUARD_MS = 30_000;

export const VERSION_POLL_MS = 60_000;

export function skewReloadHref(
	isUpdated: boolean,
	navigation: Pick<BeforeNavigate, 'willUnload' | 'to'>
): string | undefined {
	if (!isUpdated || navigation.willUnload || !navigation.to) return undefined;
	return navigation.to.url.href;
}

export function reloadOnceOnPreloadError(
	win: Pick<Window, 'addEventListener'> & {
		location: Pick<Location, 'reload'>;
		sessionStorage?: Pick<Storage, 'getItem' | 'setItem'>;
	},
	now: () => number = Date.now
): void {
	win.addEventListener('vite:preloadError', (event) => {
		event.preventDefault();
		try {
			const last = Number(win.sessionStorage?.getItem(RELOAD_GUARD_KEY) ?? 0);
			if (now() - last < RELOAD_GUARD_MS) return;
			win.sessionStorage?.setItem(RELOAD_GUARD_KEY, String(now()));
		} catch {
			return;
		}
		win.location.reload();
	});
}
