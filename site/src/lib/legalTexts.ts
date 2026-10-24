import type { Component } from 'svelte';
import { PRIVACY, TERMS, type LegalDocument, type LegalVersion } from './legal.js';
import PrivacyPolicy20261022 from './legalTexts/PrivacyPolicy20261022.svelte';
import TermsOfService20261003 from './legalTexts/TermsOfService20261003.svelte';

/**
 * The text of each version of the Terms of Service and the Privacy
 * Policy, by the date it takes effect (#1816). Two texts can be live at
 * once: during the 30 days before a material version starts, /terms
 * shows the version in force and links the coming one, and each has a
 * page of its own at versionPath.
 *
 * A text is kept once it is written, so the version a Practice agreed to
 * stays readable at its own address. The versions before the first text
 * here were written before #1816, and git holds them. `legalTexts.spec.ts`
 * holds that every version from the first text on has one, so the coming
 * version always has a page for the email to link.
 */
export type LegalText = Component<{ effective: string }>;

// By each document's address, so a copy of a document finds its texts.
const TEXTS = new Map<string, Record<string, LegalText>>([
	[TERMS.path, { '2026-10-03': TermsOfService20261003 }],
	[PRIVACY.path, { '2026-10-22': PrivacyPolicy20261022 }]
]);

// The dates in `legalDocument`'s history that have a text, oldest first.
export function textedVersions(legalDocument: LegalDocument): LegalVersion[] {
	const texts = TEXTS.get(legalDocument.path) ?? {};
	return legalDocument.versions.filter(({ effective }) => Object.hasOwn(texts, effective));
}

// The version a /terms/<date> or /privacy/<date> page names, when it has
// a text; undefined is that page's 404.
export function textedVersion(legalDocument: LegalDocument, date: string): LegalVersion | undefined {
	return textedVersions(legalDocument).find(({ effective }) => effective === date);
}

export function textOf(legalDocument: LegalDocument, version: LegalVersion): LegalText {
	const text = TEXTS.get(legalDocument.path)?.[version.effective];
	if (!text) throw new Error(`the ${legalDocument.name} has no text for ${version.effective}`);
	return text;
}

// Every date that has a text, whether or not the history names it: the
// spec holds the two together.
export function textDates(legalDocument: LegalDocument): string[] {
	return Object.keys(TEXTS.get(legalDocument.path) ?? {});
}
