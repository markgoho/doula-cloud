import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { PRIVACY, TERMS, currentVersion, type LegalDocument, type LegalVersion } from './legal.js';

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

describe('currentVersion', () => {
	it('is the newest version', () => {
		const legalDocument: LegalDocument = {
			name: 'Terms of Service',
			path: '/terms',
			versions: [
				{ effective: '2026-09-29', material: false, change: 'First version.' },
				{ effective: '2026-11-01', material: true, change: 'The price of a Credit changes.' }
			]
		};
		expect(currentVersion(legalDocument).effective).toBe('2026-11-01');
	});
});
