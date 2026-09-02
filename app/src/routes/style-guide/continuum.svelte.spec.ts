/*
 * The continuum check (CONTEXT.md, ADR-0025): for every component the
 * style guide lists, asserts that its rendered content never needs more
 * inline room than the frame it is given, at any available space from
 * 320px (ADR-0024's conformance commitment) up. It runs against the same
 * artifact a person drags at /style-guide/drag-surface -- the frame
 * markup and the demo registry are copied from that page's own
 * `+page.svelte` rather than reimplemented, because CONTEXT.md defines
 * the drag surface and this check as one artifact seen two ways. The
 * sweep itself lives in `continuum.ts`, shared with the layout exercise
 * (#534) so that exercise is marked by this instrument rather than by a
 * private copy of it.
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
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages } from './components.js';
import { mountInFrame, overflowReport, sweep } from './continuum.js';
import { toDemos, type PageModule } from './drag-surface/dragSurface.js';

const pageModules = import.meta.glob<PageModule>('./*/+page.svelte', { eager: true });

const demos = toDemos(pageModules, [
	...atomPages,
	...moleculePages,
	...organismPages,
	...templatePages
]);

/*
 * Known-broken today, tracked on their own tickets rather than suppressed
 * here (ADR-0025's own instruction). `it.fails` turns red, not green, the
 * day either issue closes without this file changing -- that is what
 * forces the entry out once the retrofit actually lands.
 */
const KNOWN_BROKEN: Readonly<Record<string, string>> = {};

if (!customElements.get('stack-l')) registerLayoutPrimitives();

describe('the continuum check', () => {
	for (const demo of demos) {
		async function assertion() {
			/*
			 * `mountInFrame` (`continuum.ts`) is the shared mount procedure:
			 * an unconstrained run, exactly like the drag surface's own
			 * `.run`, reporting how much space this environment offers on its
			 * own; the pairing re-declared inside the frame (#544); and the
			 * wait for the real webfont without which this measures the
			 * metric-compatible fallback face instead of the one a browser
			 * paints (#550). It was inline here, copied in
			 * `floor.svelte.spec.ts`, and #570's route sweep would have been
			 * the third copy -- so it moved next to `sweep`.
			 */
			const { run, frame, remove } = await mountInFrame(demo.component);
			try {
				const found = sweep(frame, run.clientWidth);
				/*
				 * The failure sentence names the component, the space it was
				 * given and the space it needed -- and then points at the
				 * exercise (#534). This is the carrier: a session that only
				 * chose a component and wrote no CSS still has to read this
				 * line, because it is the reason its commit will not go in.
				 */
				expect(found, found && overflowReport(demo.name, found)).toBeUndefined();
			} finally {
				remove();
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
