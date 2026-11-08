import { describe, expect, it } from 'vitest';
import { type DoorStorage, readLastDoor, rememberDoor } from './lastDoor.js';

function memoryStorage(): DoorStorage {
	const values = new Map<string, string>();
	return {
		getItem: (key) => values.get(key),
		setItem: (key, value) => {
			values.set(key, value);
		}
	};
}

function throwingStorage(): DoorStorage {
	throw new Error('Storage is blocked');
}

describe('lastDoor', () => {
	it('remembers the door and reads it back', () => {
		const storage = memoryStorage();
		rememberDoor('client', () => storage);
		expect(readLastDoor(() => storage)).toBe('client');
	});

	it('lights no door on a device that has used none', () => {
		expect(readLastDoor(() => memoryStorage())).toBeUndefined();
	});

	it('ignores a value it did not write', () => {
		const storage = memoryStorage();
		storage.setItem('doulacloud.lastDoor', 'kitchen');
		expect(readLastDoor(() => storage)).toBeUndefined();
	});

	it('lights no door, and does not throw, where storage is blocked', () => {
		expect(readLastDoor(throwingStorage)).toBeUndefined();
		expect(() => rememberDoor('staff', throwingStorage)).not.toThrow();
	});
});
