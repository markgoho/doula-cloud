/*
 * Marks the layout exercise (#534) with the continuum check's own sweep,
 * imported from `../continuum.js` rather than reimplemented -- so START
 * fails and FINISHED passes by the same instrument that will mark the
 * session's real work, and there is no way for the exercise to be graded
 * more kindly than the gate.
 *
 * START is deliberately broken, and this is the only place that is
 * allowed to be true. It is safe because neither the continuum check nor
 * the drag surface can see it: both build their list with
 * `toDemos(..., [atomPages, moleculePages, organismPages, templatePages])`,
 * which produces a demo only for a `/style-guide/<slug>` route whose slug
 * matches a file under `src/lib/components`. Nothing named
 * `layout-exercise` lives there, so both skip this route by construction
 * rather than by an allowlist either of them maintains.
 */
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { registerLayoutPrimitives } from '#lib/primitives/index.js';
import '#lib/styles/app.css';
import { CONFORMANCE_COMMITMENT, sweep, type Break } from '../continuum.js';
import ExercisePage from './+page.svelte';
import ContactCardFinished from './ContactCardFinished.svelte';
import ContactCardStart from './ContactCardStart.svelte';
import { exerciseFields, type Field } from './fields.js';
import type { Component } from 'svelte';

if (!customElements.get('stack-l')) registerLayoutPrimitives();

type Card = Component<{ fields: Field[] }>;

async function setup(card: Card): Promise<Break | undefined> {
	const run = document.createElement('div');
	const frame = document.createElement('div');
	frame.style.containerType = 'inline-size';
	run.append(frame);
	document.body.append(run);
	try {
		await render(card, { fields: [...exerciseFields] }, { baseElement: frame });
		return sweep(frame, run.clientWidth);
	} finally {
		run.remove();
	}
}

describe('the layout exercise', () => {
	it('START needs more room than it is given, which is the point of it', async () => {
		const found = await setup(ContactCardStart);
		expect(found?.width).toBe(CONFORMANCE_COMMITMENT);
		// 370px inside 320px when this was written -- 50px past its own
		// edge. The assertion stays "more than it was given" rather than
		// pinning 370, because the exact figure moves with the font.
		// The exercise is only an exercise while this stays true: if a
		// change elsewhere makes START fit, the task it states no longer
		// describes anything and the file has to be re-broken or dropped.
		expect(found?.needed).toBeGreaterThan(CONFORMANCE_COMMITMENT);
	});

	it('FINISHED never needs more room than it is given', async () => {
		expect(await setup(ContactCardFinished)).toBeUndefined();
	});

	// The failure message sends a session to this page, so the page has to
	// render -- a carrier pointing at a runtime error teaches nothing. The
	// continuum check's glob imports this module but never mounts it.
	it('states the task on a page that renders', async () => {
		const screen = await render(ExercisePage);
		await expect
			.element(screen.getByRole('heading', { name: /a card that has one configuration/i }))
			.toBeVisible();
	});
});
