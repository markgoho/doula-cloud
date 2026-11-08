import { readdirSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

// Every screen's <h1> is one size. The Overview's title once took the
// display step while every other Template drew the page Heading, so it
// alone was larger. A Template draws its title as that Heading; the one
// raw <h1> is QuestionPage's, which has to sit in a <legend> or wrap a
// <label> (#464), and is set at the same heading-lg step.
const templatesRoot = fileURLToPath(new URL('.', import.meta.url));
const RAW_H1_ALLOWED = new Set(['QuestionPage.svelte']);

const templates = readdirSync(templatesRoot)
	.filter((name) => name.endsWith('.svelte'))
	.map((name) => ({
		name,
		source: readFileSync(path.join(templatesRoot, name), 'utf8').replaceAll(/<!--[\s\S]*?-->|\/\*[\s\S]*?\*\//g, '')
	}));

describe('a page title', () => {
	it('is drawn as the page Heading by every Template but QuestionPage', () => {
		const raw = templates.filter(({ name, source }) => !RAW_H1_ALLOWED.has(name) && /<h1[\s>]/.test(source));

		expect(raw.map(({ name }) => name)).toEqual([]);
	});

	it('is never set at another type step than heading-lg', () => {
		const offStep = templates.filter(({ source }) => /h1\b[^{]*\{[^}]*--text-(?!heading-lg)[a-z-]+-size/.test(source));

		expect(offStep.map(({ name }) => name)).toEqual([]);
	});

	it('takes no level-1 Heading in a variant other than page', () => {
		const offVariant = templates.filter(({ source }) => /<Heading level=\{1\}(?![^>]*variant="page")/.test(source));

		expect(offVariant.map(({ name }) => name)).toEqual([]);
	});
});
