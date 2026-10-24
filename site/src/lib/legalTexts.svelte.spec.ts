import { describe, expect, it } from 'vitest';
import { PRIVACY, TERMS, type LegalDocument, versionInForce } from './legal.js';
import { textDates, versionWithText, versionsWithText, textOf } from './legalTexts.js';

describe('the text of each version (#1816)', () => {
	it.each([TERMS, PRIVACY])('the $name: every text is a version in the history', (legalDocument) => {
		const history = new Set(legalDocument.versions.map(({ effective }) => effective));
		expect(textDates(legalDocument).filter((date) => !history.has(date))).toEqual([]);
	});

	// So a coming version always has a page, and a text is never dropped
	// from the middle of the history.
	it.each([TERMS, PRIVACY])('the $name: every version from the first text on has one', (legalDocument) => {
		const texted = versionsWithText(legalDocument);
		expect(texted.length).toBeGreaterThan(0);
		const fromFirstText = legalDocument.versions.indexOf(texted[0]);
		expect(texted).toEqual(legalDocument.versions.slice(fromFirstText));
	});

	it.each([TERMS, PRIVACY])('the $name: the version in force today has a text', (legalDocument) => {
		const inForce = versionInForce(legalDocument, new Date());
		expect(textOf(legalDocument, inForce)).toBeDefined();
	});

	it('finds the version a page names, and no version without a text', () => {
		expect(versionWithText(TERMS, '2026-10-03')).toBe(TERMS.versions.at(-1));
		expect(versionWithText(TERMS, '2026-09-29')).toBeUndefined();
		expect(versionWithText(TERMS, 'not-a-date')).toBeUndefined();
	});

	it('refuses a version with no text, and a document it does not hold', () => {
		const unknown: LegalDocument = { name: 'Cookie Policy', path: '/cookies', versions: [] };
		expect(() => textOf(TERMS, TERMS.versions[0])).toThrow('the Terms of Service has no text for 2026-09-29');
		expect(() => textOf(unknown, { effective: '2026-10-01', material: false, change: 'First version.' })).toThrow(
			'the Cookie Policy has no text for 2026-10-01'
		);
		expect(versionsWithText(unknown)).toEqual([]);
		expect(textDates(unknown)).toEqual([]);
	});
});
