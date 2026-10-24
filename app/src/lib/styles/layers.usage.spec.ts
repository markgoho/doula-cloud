import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1863: the cascade layer order is written in two places, and this spec
 * holds them equal.
 *
 * - `src/app.html` carries it in an inline `<style>` before
 *   `%sveltekit.head%`, so it is the first stylesheet every page meets,
 *   whatever order the built chunks load in. It has to live there: the
 *   production build's CSS minifier (Lightning CSS) removes the names of
 *   `layers.css`'s bare `@layer ...;` statement once `app.css`'s own
 *   blocks already meet them in that order, and the template is outside
 *   Vite's CSS pipeline, so nothing rewrites it.
 * - `layers.css` carries it for the places that never see `app.html`: the
 *   browser spec project's setup file, and `app.css` itself.
 *
 * `e2e/cascade-layer-order.e2e.ts` reads what the built page serves.
 */
function layerOrderIn(relativePath: string): string[] {
	const source = readFileSync(fileURLToPath(new URL(relativePath, import.meta.url)), 'utf8');
	const statement = /@layer\s+([\w\s,-]+);/.exec(source);
	if (statement === null) {
		return [];
	}
	return statement[1].split(',').map((name) => name.trim());
}

describe('the cascade layer order', () => {
	it('is the same list in app.html and layers.css', () => {
		const fromLayersCss = layerOrderIn('./layers.css');
		expect(fromLayersCss).toEqual(['reset', 'tokens', 'base', 'utilities', 'components']);
		expect(layerOrderIn('../../app.html')).toEqual(fromLayersCss);
	});
});
