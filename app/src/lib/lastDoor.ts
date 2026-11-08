/*
 * Which door on the signed-out landing this device used last: the lamp in
 * the surface brief for `/`. It lives on the device, not on an account,
 * because nobody is signed in on that page.
 *
 * Storage can be absent or throw (a private window, blocked site data), so
 * every read and write is guarded and a failure means no door is lit. The
 * route reads it once, before the first paint, as it reads the greeting, so
 * a door never lights up after the page has drawn.
 */
export type Door = 'staff' | 'client';

// The two calls this module makes, so anything that answers them will do:
// the device's `localStorage`, or a spec's own map.
export interface DoorStorage {
	getItem(key: string): string | null | undefined;
	setItem(key: string, value: string): void;
}

const KEY = 'doulacloud.lastDoor';

function isDoor(value: string | null | undefined): value is Door {
	return value === 'staff' || value === 'client';
}

export function readLastDoor(storage: () => DoorStorage = () => localStorage): Door | undefined {
	try {
		const value = storage().getItem(KEY);
		return isDoor(value) ? value : undefined;
	} catch {
		return undefined;
	}
}

export function rememberDoor(door: Door, storage: () => DoorStorage = () => localStorage): void {
	try {
		storage().setItem(KEY, door);
	} catch {
		// A device that cannot store simply lights no door next time.
	}
}
