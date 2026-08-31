/*
 * The continuum check (CONTEXT.md, ADR-0025): for every component the
 * style guide lists, asserts that its rendered content never needs more
 * inline room than the frame it is given, at any available space from
 * 320px (ADR-0024's conformance commitment) up. It runs against the same
 * artifact a person drags at /style-guide/drag-surface -- the frame
 * markup and the demo registry are copied from that page's own
 * `+page.svelte` rather than reimplemented, because CONTEXT.md defines
 * the drag surface and this check as one artifact seen two ways.
 *
 * The glob has to be its own eager copy rather than an import of
 * `+page.svelte` itself: #527 found that an eager glob of every
 * component dropped the repo's coverage from 100% to 94% when it lived
 * in a module a plain `.spec.ts` file imported, because that file also
 * runs under the node-environment "server" Vitest project. This file's
 * `.svelte.spec.ts` name keeps it out of that project entirely (see
 * `vite.config.ts`), so the glob only ever executes here, in the
 * browser, the same place `+page.svelte` executes its own copy.
 *
 * It runs in the ordinary client suite -- part of `bun run test` and
 * `scripts/hooks/pre-commit` -- rather than the e2e suite. No style-guide
 * page fetches anything, so nothing here needs a build, Postgres or the
 * BFF; the browser Vitest project is already the full cost this check
 * requires.
 */
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages } from './components.js';
import { toDemos, type PageModule } from './drag-surface/dragSurface.js';

const pageModules = import.meta.glob<PageModule>('./*/+page.svelte', { eager: true });

const demos = toDemos(pageModules, [
	...atomPages,
	...moleculePages,
	...organismPages,
	...templatePages
]);

// ADR-0024: 320 is a conformance commitment, not a content floor -- the
// only width this check ever names, and the low end of every sweep.
const CONFORMANCE_COMMITMENT = 320;
// A sweep's step size is a resolution, not a design (ADR-0025) -- 4px is
// fine enough to catch a content floor and coarse enough to keep the
// suite fast across every component in one run.
const RESOLUTION = 4;
// Sub-pixel rounding from the browser's own layout, not a real overflow.
const TOLERANCE = 1;

/*
 * Known-broken today, tracked on their own tickets rather than suppressed
 * here (ADR-0025's own instruction). `it.fails` turns red, not green, the
 * day either issue closes without this file changing -- that is what
 * forces the entry out once the retrofit actually lands.
 */
const KNOWN_BROKEN: Readonly<Record<string, string>> = {
	'description-list': '#530',
	'data-table': '#508'
};

interface Break {
	width: number;
	needed: number;
}

/*
 * Sweeps a frame already holding a rendered demo, from 320px up to
 * whatever this environment naturally offers -- the same
 * `Math.max(availableSpace, CONFORMANCE_COMMITMENT)` the drag surface's
 * own page uses for its handle's far end, so the check invents no upper
 * width of its own either.
 */
function sweep(frame: HTMLElement, availableSpace: number): Break | undefined {
	const widestSpace = Math.max(availableSpace, CONFORMANCE_COMMITMENT);
	for (let width = CONFORMANCE_COMMITMENT; width <= widestSpace; width += RESOLUTION) {
		frame.style.inlineSize = `${width}px`;
		void frame.offsetWidth;
		if (frame.scrollWidth - width > TOLERANCE) {
			return { width, needed: frame.scrollWidth };
		}
	}
	return undefined;
}

if (!customElements.get('stack-l')) registerLayoutPrimitives();

describe('the continuum check', () => {
	for (const demo of demos) {
		async function assertion() {
			// An unconstrained run, exactly like the drag surface's own `.run`,
			// reports how much space this environment offers on its own.
			const run = document.createElement('div');
			const frame = document.createElement('div');
			frame.style.containerType = 'inline-size';
			run.append(frame);
			document.body.append(run);
			try {
				await render(demo.component, {}, { baseElement: frame });
				const found = sweep(frame, run.clientWidth);
				// `found` prints its own width/needed pair in the failure diff --
				// the "space given" and "space needed" the acceptance criteria ask
				// for -- and the test name already carries the component.
				expect(found).toBeUndefined();
			} finally {
				run.remove();
			}
		}

		const brokenOn = KNOWN_BROKEN[demo.slug];
		if (brokenOn) {
			it.fails(`${demo.name} (${brokenOn}) still needs more room than it is given`, assertion);
		} else {
			it(`${demo.name} never needs more room than it is given`, assertion);
		}
	}
});
