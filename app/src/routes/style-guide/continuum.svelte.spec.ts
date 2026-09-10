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
import {
	frameHolding,
	frameHoldingLoadingLedger,
	ledgerMarkup,
	mountInFrame,
	overflowReport,
	OVERFLOWING,
	revealDisclosures,
	SETTLE_TURNS,
	sweep
} from './continuum.js';
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
 * the frame is built by hand instead of through `mountInFrame`
 * (`continuum.ts`'s `frameHolding` and `ledgerMarkup`, shared with the
 * floor check's own disclosure block since #1124).
 *
 * The first assertion is the blind spot itself, kept rather than deleted:
 * a plain `scrollWidth` read is exactly what the sweep was before #710,
 * and this is the line that says why it could not stay that. It is also
 * the guard on the claim the rest of this rests on -- that a closed
 * disclosure's content is not laid out at all.
 */
describe('the sweep, over a closed disclosure (#710)', () => {
	it('measures nothing at all while the disclosure stays closed', () => {
		const { frame, remove } = frameHolding(ledgerMarkup());
		try {
			frame.style.inlineSize = '320px';
			void frame.offsetWidth;

			expect(frame.scrollWidth).toBe(320);
		} finally {
			remove();
		}
	});

	it('finds the overflow a closed disclosure was hiding', () => {
		const { run, frame, remove } = frameHolding(ledgerMarkup());
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
		const { run, frame, remove } = frameHolding(ledgerMarkup());
		try {
			sweep(frame, run.clientWidth);

			expect(frame.querySelector('details')?.open).toBe(false);
		} finally {
			remove();
		}
	});

	/*
	 * A measurement must not be an action. The Staff roster loads both of
	 * a row's histories -- work state (#459) and Membership (#872), one
	 * `HistoryDisclosure` each -- from an `ontoggle` handler, so a sweep that left
	 * a handler seeing `open` would make measuring a screen issue that
	 * screen's requests. It does not, and this pins why rather than
	 * leaving it to a doc comment: a `toggle` event is queued rather than
	 * dispatched synchronously and repeated changes coalesce, so a
	 * disclosure opened and closed again inside one task reports only the
	 * state it ended in. Measured against the real route as well as here
	 * -- the MEASUREMENT of the Staff roster makes zero history requests of
	 * either kind, which is what #1126 had to leave standing while giving
	 * the check a way to reach that history at all: what asks for it is
	 * `revealDisclosures`, in preparation, and the sweep that follows asks
	 * for nothing. This is the assertion that keeps that true if the
	 * open/undo pair ever stops being synchronous.
	 */
	it('never lets a toggle handler see the disclosure open', async () => {
		const { run, frame, remove } = frameHolding(ledgerMarkup());
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
		const { run, frame, remove } = frameHolding(ledgerMarkup(true));
		try {
			sweep(frame, run.clientWidth);

			expect(frame.querySelector('details')?.open).toBe(true);
		} finally {
			remove();
		}
	});
});

/*
 * What reaches the frame before the sweep measures it, when a disclosure
 * fetches its own content the first time it opens (#1126).
 *
 * #710 left this open on purpose and said why: the sweep opens, measures
 * and closes again inside one task, a `toggle` event is queued rather than
 * dispatched synchronously, and repeated changes coalesce -- so no handler
 * ever sees the disclosure open, and a measurement is therefore not an
 * action. That is the property this ticket had to keep while closing the
 * hole it leaves, which is why the fix is not "await the toggle" inside
 * `sweep`.
 *
 * It is split instead: `revealDisclosures` prepares the subject and
 * `sweep` measures it. Preparation is where a screen's own loads already
 * happen -- `mountInFrame` runs a route's `onMount` cascade, and #885's
 * settle wait exists precisely because those loads are the check's to wait
 * for -- so a disclosure opened there reaches the state a person reaches
 * by clicking one on the drag surface. The measurement is unchanged from
 * #710 and still acts on nothing, which the load counts below pin to a
 * number rather than to this paragraph.
 */
describe('the sweep, over a disclosure that loads on open (#1126)', () => {
	it('measures the loading state when nothing has prepared the disclosure', async () => {
		const { run, frame, remove, loads } = frameHoldingLoadingLedger();
		try {
			const found = sweep(frame, run.clientWidth);
			await new Promise((resolve) => setTimeout(resolve, 50));

			// The `Loading...` the disclosure holds fits at every width, so the
			// sweep reports a screen that fits -- and the content that does not
			// fit was never asked for. The blind spot and the property worth
			// keeping, in one measurement.
			expect(found).toBeUndefined();
			expect(loads()).toBe(0);
		} finally {
			remove();
		}
	});

	it('finds the overflow in content the disclosure loaded on open', async () => {
		const { run, frame, remove } = frameHoldingLoadingLedger();
		try {
			await revealDisclosures(frame, 'The loading ledger');
			const found = sweep(frame, run.clientWidth);

			expect(found?.width).toBe(320);
			expect(found?.needed).toBeGreaterThanOrEqual(OVERFLOWING);
		} finally {
			remove();
		}
	});

	it('asks for the content once, in preparation, and never while measuring', async () => {
		const { run, frame, remove, loads } = frameHoldingLoadingLedger();
		try {
			await revealDisclosures(frame, 'The loading ledger');
			const afterPreparing = loads();
			sweep(frame, run.clientWidth);
			await new Promise((resolve) => setTimeout(resolve, 50));

			expect(afterPreparing).toBe(1);
			expect(loads()).toBe(1);
		} finally {
			remove();
		}
	});

	/*
	 * Where every instrument gets this for free (#1126). The floor check and
	 * the component sweep both reach their subjects through `mountInFrame`
	 * and neither reveals anything of its own, so what says they are covered
	 * is that the shared mount procedure leaves nothing closed behind it.
	 * That the line runs is not the same as that it worked -- coverage would
	 * be satisfied either way -- so this reads the mounted DOM instead.
	 *
	 * `HistoryDisclosure`'s own demo page is the subject because it is the
	 * one in the registry built out of closed disclosures, five of them, and
	 * `querySelectorAll` rather than an accessible query for the reason this
	 * file's other disclosure assertions give (`.claude/rules/
	 * svelte-tests.md` case 3): what is asserted is a fact about the
	 * document, and `open` is the property the instrument writes.
	 */
	it('leaves nothing closed behind it when it mounts a subject', async () => {
		const demo = pageModules['./history-disclosure/+page.svelte'];
		const { frame, remove } = await mountInFrame(demo.default);
		try {
			expect(frame.querySelectorAll('details').length).toBeGreaterThan(0);
			expect(frame.querySelectorAll('details:not([open])')).toHaveLength(0);
		} finally {
			remove();
		}
	});

	/*
	 * A settle signal that gives up quietly is the one failure this
	 * instrument must not have: it would measure whatever happened to be in
	 * the frame at the moment the budget ran out and report that as the
	 * screen. So the wait is bounded and the bound throws, and this is the
	 * subject that trips it -- a disclosure whose content never stops
	 * arriving, which is what a screen that polls looks like from here.
	 */
	it('stops loudly rather than measuring content that never settles', async () => {
		const { frame, remove } = frameHolding(
			'<details><summary>Show what has happened</summary><p>Loading...</p></details>'
		);
		const waiting = frame.querySelector('p')!;
		const restless = setInterval(() => {
			waiting.textContent = `Loading... ${Date.now()}`;
		}, 0);
		try {
			await expect(revealDisclosures(frame, 'The restless ledger')).rejects.toThrow(
				`after ${SETTLE_TURNS} turns`
			);
		} finally {
			clearInterval(restless);
			remove();
		}
	});
});
