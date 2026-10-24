import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { PRIVACY, TERMS, laterVersions, type LegalDocument, type LegalVersion, versionInForce, versionPath } from './legal.js';

/*
 * The version agreement (#1556's third seam). The BFF records which
 * version of each document an Owner agreed to (ADR-0053, #1547), and this
 * site prints the text of that version. The two histories are a mirrored
 * pair, like SITE_ORIGIN and website.SiteBaseURL, so this spec reads the
 * Go source the BFF compiles -- the same move app/'s spelling spec makes
 * -- and fails when the two disagree about any version: its date, its
 * material flag, or its line of what changed.
 */
const goSource = readFileSync(
	fileURLToPath(new URL('../../../api/internal/legal/legal.go', import.meta.url)),
	'utf8'
);

function goHistory(variable: string): LegalVersion[] {
	const block = new RegExp(String.raw`^var ${variable} = Document\{\n([\s\S]*?)^\}`, 'm').exec(goSource);
	if (!block) throw new Error(`api/internal/legal/legal.go declares no ${variable}`);
	return block[1]
		.matchAll(/\{Effective: "([^"]+)", Material: (true|false), Change: "([^"]+)"\}/g)
		.map(([, effective, material, change]) => ({ effective, material: material === 'true', change }))
		.toArray();
}

describe('the version of each document, on the site and in the BFF', () => {
	it.each([
		['Terms', TERMS],
		['Privacy', PRIVACY]
	] as const)('%s: both hold the same history', (variable, legalDocument: LegalDocument) => {
		const bff = goHistory(variable);
		expect(bff.length).toBeGreaterThan(0);
		expect(legalDocument.versions).toEqual(bff);
	});

	it('finds nothing to compare for a variable the Go file does not declare', () => {
		expect(() => goHistory('Nothing')).toThrow('declares no Nothing');
	});
});

// A history with a version in force and a material one still to come:
// the 30 days of notice (#1816).
const HISTORY: LegalDocument = {
	name: 'Terms of Service',
	path: '/terms',
	versions: [
		{ effective: '2026-09-29', material: false, change: 'First version.' },
		{ effective: '2026-10-03', material: false, change: 'Writes the name of the product as one word.' },
		{ effective: '2026-11-01', material: true, change: 'The price of a Credit changes.' }
	]
};

describe('versionInForce', () => {
	it('is the newest version that has started, not the newest version', () => {
		expect(versionInForce(HISTORY, new Date('2026-10-20T12:00:00Z')).effective).toBe('2026-10-03');
	});

	// The reading the BFF gives the same date: midnight UTC, which is the
	// evening before in New York.
	it('starts a version at midnight UTC on its date', () => {
		expect(versionInForce(HISTORY, new Date('2026-10-31T23:59:59.999Z')).effective).toBe('2026-10-03');
		expect(versionInForce(HISTORY, new Date('2026-11-01T00:00:00Z')).effective).toBe('2026-11-01');
		expect(versionInForce(HISTORY, new Date('2026-10-31T21:00:00-04:00')).effective).toBe('2026-11-01');
	});

	it('refuses a date before the first version, which no page can print', () => {
		expect(() => versionInForce(HISTORY, new Date('2026-09-01T00:00:00Z'))).toThrow(
			'no version of the Terms of Service is in force on 2026-09-01'
		);
	});

	it.each([TERMS, PRIVACY])('finds a version of the $name in force today', (legalDocument) => {
		expect(versionInForce(legalDocument, new Date())).toBeDefined();
	});
});

describe('laterVersions', () => {
	it('is every version after the one given, oldest first, whatever the date', () => {
		expect(laterVersions(HISTORY, HISTORY.versions[1]).map(({ effective }) => effective)).toEqual(['2026-11-01']);
		expect(laterVersions(HISTORY, HISTORY.versions[2])).toEqual([]);
	});
});

describe('versionPath', () => {
	// The BFF's email links this address (api/internal/legalnotice).
	it('is the document address and the date the version takes effect', () => {
		expect(versionPath(HISTORY, HISTORY.versions[2])).toBe('/terms/2026-11-01');
	});
});
