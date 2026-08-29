// `bun run sync-fonts` -- pulls Hanken Grotesk from Google Fonts and writes
// self-hosted woff2 files plus the generated `@font-face` block into
// src/lib/styles/fonts/, the same delivery decision #96 made for Phosphor
// icons: committed assets, never a runtime CDN link and never an npm package.
//
// The brief (docs/design/brief.md) names Hanken Grotesk as the single family
// and asks for self-hosting on #96's privacy and performance grounds. It also
// makes "nothing shifts after it paints" a requirement, which is why this
// script does more than download a file: it reads the real font's metrics out
// of the variable TTF and emits a metric-override fallback, so the swap from
// the fallback face to the real one moves no text.
//
// One variable file covers every weight the scale uses (400/500/600), so there
// are no per-weight downloads. Italic is deliberately not fetched -- no step in
// the brief's type scale uses it (#417).

const targetDirectory = new URL('../src/lib/styles/fonts/', import.meta.url);
const subsets = ['latin', 'latin-ext'];

// Google's CSS API serves woff2 + variable ranges only to a browser-shaped
// request; an unrecognised agent gets legacy static TTF instead.
const modernBrowser =
	'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36';

const cssUrl =
	'https://fonts.googleapis.com/css2?family=Hanken+Grotesk:wght@400..600&display=swap';

// Arial is the fallback the stack lands on when nothing else is installed.
// Its metrics are fixed and well known, so they are stated here rather than
// probed -- the local file is not ours to read.
//
// size-adjust is derived from x-height, NOT from OS/2 xAvgCharWidth. Arial
// computes that field under the original spec (a weighted average over
// lowercase only) while modern fonts use the revised one (every non-zero
// glyph), so the two are not comparable and the ratio comes out ~23% too
// large. x-height is defined the same way everywhere, and it is what governs
// how big a face looks at a given size.
const arial = { unitsPerEm: 2048, sxHeight: 1062 };

async function fetchOrThrow(url: string, headers: HeadersInit = {}): Promise<Response> {
	const response = await fetch(url, { headers });
	if (!response.ok) {
		throw new Error(`Failed to fetch ${url}: ${response.status} ${response.statusText}`);
	}
	return response;
}

/**
Reads unitsPerEm, hhea vertical metrics and OS/2 average width from a TTF.
*/
function readMetrics(ttf: DataView) {
	const tables = new Map<string, number>();
	const tableCount = ttf.getUint16(4);
	for (let index = 0; index < tableCount; index += 1) {
		const record = 12 + index * 16;
		const tag = String.fromCodePoint(...[0, 1, 2, 3].map((n) => ttf.getUint8(record + n)));
		tables.set(tag, ttf.getUint32(record + 8));
	}

	const head = tables.get('head');
	const hhea = tables.get('hhea');
	const os2 = tables.get('OS/2');
	if (head === undefined || hhea === undefined || os2 === undefined) {
		throw new Error('TTF is missing head, hhea or OS/2 -- cannot derive fallback metrics.');
	}

	return {
		unitsPerEm: ttf.getUint16(head + 18),
		os2Version: ttf.getUint16(os2),
		sxHeight: ttf.getInt16(os2 + 86),
		ascender: ttf.getInt16(hhea + 4),
		descender: ttf.getInt16(hhea + 6),
		lineGap: ttf.getInt16(hhea + 8),
		xAvgCharWidth: ttf.getInt16(os2 + 2)
	};
}

async function fetchText(url: string, headers: HeadersInit = {}): Promise<string> {
	const response = await fetchOrThrow(url, headers);
	return response.text();
}

async function fetchBytes(url: string): Promise<ArrayBuffer> {
	const response = await fetchOrThrow(url);
	return response.arrayBuffer();
}

const percent = (ratio: number) => `${(ratio * 100).toFixed(2)}%`;

const css = await fetchText(cssUrl, { 'User-Agent': modernBrowser });

// Each @font-face in the response carries one subset's unicode-range. Keep
// only the subsets we serve, and rewrite each src to the committed file.
const faces: string[] = [];
for (const [, subset, body] of css.matchAll(/\/\*\s*([\w-]+)\s*\*\/\s*@font-face\s*\{([^}]+)\}/g)) {
	const remoteUrl = /src:\s*url\(([^)]+)\)/.exec(body)?.[1];
	const unicodeRange = /unicode-range:\s*([^;]+);/.exec(body)?.[1];
	if (!subset || !remoteUrl || !unicodeRange || !subsets.includes(subset)) continue;

	const fileName = `hanken-grotesk-${subset}.woff2`;
	const bytes = new Uint8Array(await fetchBytes(remoteUrl));
	// No mkdir: Bun.write creates the parent directories it needs.
	await Bun.write(new URL(fileName, targetDirectory), bytes);

	faces.push(`@font-face {
	font-family: 'Hanken Grotesk';
	font-style: normal;
	font-weight: 400 600;
	font-display: swap;
	src: url('./fonts/${fileName}') format('woff2');
	unicode-range: ${unicodeRange.trim()};
}`);
}

if (faces.length === 0) {
	throw new Error(`No ${subsets.join('/')} subset found in the Google Fonts response.`);
}

const variableTtfUrl =
	'https://raw.githubusercontent.com/google/fonts/main/ofl/hankengrotesk/HankenGrotesk%5Bwght%5D.ttf';

const metrics = readMetrics(
	new DataView(await fetchBytes(variableTtfUrl))
);

// size-adjust first: every vertical override is expressed relative to the
// already-scaled em, so it has to divide through by the same factor.
if (metrics.os2Version < 2) {
	throw new Error('OS/2 table predates sxHeight -- cannot derive a comparable size-adjust.');
}
const sizeAdjust =
	metrics.sxHeight / metrics.unitsPerEm / (arial.sxHeight / arial.unitsPerEm);

const fallback = `/*
 * The metric-compatible fallback. Arial is re-declared under a private name
 * with overrides that make it occupy exactly the space Hanken Grotesk will,
 * so when the real face arrives the text does not reflow -- the brief's
 * "nothing shifts after it paints", enforced at the font layer.
 */
@font-face {
	font-family: 'Hanken Grotesk Fallback';
	src: local('Arial'), local('Helvetica'), local('Liberation Sans');
	size-adjust: ${percent(sizeAdjust)};
	ascent-override: ${percent(metrics.ascender / metrics.unitsPerEm / sizeAdjust)};
	descent-override: ${percent(Math.abs(metrics.descender) / metrics.unitsPerEm / sizeAdjust)};
	line-gap-override: ${percent(metrics.lineGap / metrics.unitsPerEm / sizeAdjust)};
}`;

const generated = `/*
 * Generated by \`bun run sync-fonts\`. Do not edit by hand.
 *
 * Source: Google Fonts (Hanken Grotesk, SIL Open Font License), self-hosted
 * per the brief and #96's delivery decision. Regenerate rather than patch.
 */

${faces.join('\n\n')}

${fallback}
`;

await Bun.write(new URL('../fonts.css', targetDirectory), generated);

console.log(
	`Wrote ${faces.length} face(s) and a fallback at size-adjust ${percent(sizeAdjust)}.`
);
