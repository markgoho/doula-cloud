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

/*
 * What the sweep does to a disclosure before it measures one (#710).
 *
 * This is the instrument under test rather than a subject swept by it, so
 * the frame is built by hand instead of through `mountInFrame`: what it
 * needs is a `<details>` whose content is wider than any space the sweep
 * will offer it, and no component this repo ships is allowed to be that.
 * It needs no webfont wait either -- the overflow here is a declared
 * inline size, not a measured glyph.
 *
 * The first assertion is the blind spot itself, kept rather than deleted:
 * a plain `scrollWidth` read is exactly what the sweep was before #710,
 * and this is the line that says why it could not stay that. It is also
 * the guard on the claim the rest of this rests on -- that a closed
 * disclosure's content is not laid out at all.
 */
// Wider than the ~414px window this file runs in, so what breaks is the
// disclosure's own content rather than anything the frame could absorb.
const OVERFLOWING = 900;

function frameHolding(markup: string) {
	const run = document.createElement('div');
	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	frame.innerHTML = markup;
	run.append(frame);
	document.body.append(run);
	return { run, frame, remove: () => run.remove() };
}

// The Client portal's Activity ledger in miniature (#486): a summary a
// Client clicks, and behind it content that has to fit at 320px.
function ledger(isOpen = false): string {
	return (
		`<details${isOpen ? ' open' : ''}><summary>Show what has happened</summary>` +
		`<div style="inline-size: ${OVERFLOWING}px">Everything that has happened</div></details>`
	);
}

describe('the sweep, over a closed disclosure (#710)', () => {
	it('measures nothing at all while the disclosure stays closed', () => {
		const { frame, remove } = frameHolding(ledger());
		try {
			frame.style.inlineSize = '320px';
			void frame.offsetWidth;

			expect(frame.scrollWidth).toBe(320);
		} finally {
			remove();
		}
	});

	it('finds the overflow a closed disclosure was hiding', () => {
		const { run, frame, remove } = frameHolding(ledger());
		try {
			const found = sweep(frame, run.clientWidth);

			expect(found?.width).toBe(320);
			expect(found?.needed).toBeGreaterThanOrEqual(OVERFLOWING);
		} finally {
			remove();
		}
	});

	/*
	 * `querySelector` in these last two, deliberately. What they assert is
	 * a fact about the instrument rather than about the screen -- that the
	 * sweep put back the DOM property it changed -- and `open` is that
	 * property. A `<summary>`'s `aria-expanded` describes the same state,
	 * but reading it here would assert the browser's mapping of the
	 * property instead of the property the sweep actually wrote, which is
	 * the thing under test (`.claude/rules/svelte-tests.md` case 3: a fact
	 * about the document with no element for an accessible query to ask
	 * about).
	 */
	it('leaves the disclosure closed again afterwards', () => {
		const { run, frame, remove } = frameHolding(ledger());
		try {
			sweep(frame, run.clientWidth);

			expect(frame.querySelector('details')?.open).toBe(false);
		} finally {
			remove();
		}
	});

	/*
	 * A measurement must not be an action. The Staff roster loads its
	 * work-state history from an `ontoggle` handler, so a sweep that left
	 * a handler seeing `open` would make measuring a screen issue that
	 * screen's requests. It does not, and this pins why rather than
	 * leaving it to a doc comment: a `toggle` event is queued rather than
	 * dispatched synchronously and repeated changes coalesce, so a
	 * disclosure opened and closed again inside one task reports only the
	 * state it ended in. Measured against the real route as well as here
	 * -- a full sweep of the Staff roster makes zero work-state-history
	 * requests -- and this is the assertion that keeps it true if the
	 * open/undo pair ever stops being synchronous.
	 */
	it('never lets a toggle handler see the disclosure open', async () => {
		const { run, frame, remove } = frameHolding(ledger());
		try {
			const openStates: boolean[] = [];
			const disclosure = frame.querySelector('details')!;
			disclosure.addEventListener('toggle', () => {
				openStates.push(disclosure.open);
			});

			const found = sweep(frame, run.clientWidth);
			await new Promise((resolve) => setTimeout(resolve, 50));

			// That the sweep found the break is what says it really did open
			// the disclosure -- without it this passes on a sweep that never
			// touched one, which is the same thing said the other way round.
			expect(found).toBeDefined();
			expect(openStates).not.toContain(true);
		} finally {
			remove();
		}
	});

	it('leaves a disclosure the subject ships open alone', () => {
		const { run, frame, remove } = frameHolding(ledger(true));
		try {
			sweep(frame, run.clientWidth);

			expect(frame.querySelector('details')?.open).toBe(true);
		} finally {
			remove();
		}
	});
});
