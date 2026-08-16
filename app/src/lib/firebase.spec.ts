import { describe, it, expect } from 'vitest';
import { emulatorURL } from './firebase';

describe('emulatorURL', () => {
	it('returns undefined when no emulator host is configured', () => {
		expect(emulatorURL(undefined)).toBeUndefined();
	});

	it('builds an http URL from a host:port pair', () => {
		expect(emulatorURL('127.0.0.1:9099')).toBe('http://127.0.0.1:9099');
	});
});
