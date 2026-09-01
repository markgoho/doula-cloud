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
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { atomPages, moleculePages, organismPages, templatePages } from './components.js';
import { ensureFontLoaded, overflowReport, sweep } from './continuum.js';
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
const KNOWN_BROKEN: Readonly<Record<string, string>> = {
	'description-list': '#530',
	link: '#548'
};

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
				/*
				 * The base size re-resolved against the frame (#544), which is
				 * the drag surface's `.frame > *` rule expressed in the DOM
				 * this check builds by hand. `font-size` inherits as a
				 * computed length and a `cqi` resolves against the nearest
				 * ANCESTOR container, so without this the demo renders in
				 * letters sized for the window while the sweep reports 320px
				 * -- the instrument lying in the same direction every time.
				 */
				for (const child of frame.children) {
					(child as HTMLElement).style.fontSize = 'var(--text-body-size)';
				}
				/*
				 * `Hanken Grotesk` loads under `font-display: swap` (`fonts.css`), so
				 * a sweep taken right after render measures the metric-compatible
				 * fallback face, not the one a browser paints -- and a real break
				 * can fit inside the fallback and go unseen (#550). `ready` alone
				 * turned out not to be enough: it resolves once every REQUESTED
				 * load has settled, not once a load has actually been requested,
				 * so it can resolve before `swap` has asked for the face at all.
				 * `ensureFontLoaded` requests it explicitly and refuses to let a
				 * measurement proceed if the real face never reported loaded,
				 * which is what actually measures the same face the drag surface
				 * paints (#564's floor check hit this same bug independently;
				 * the fix is shared here rather than duplicated).
				 */
				await ensureFontLoaded();
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
