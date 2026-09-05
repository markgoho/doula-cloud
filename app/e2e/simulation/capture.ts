// Capture is not reporting (#779's own words): every act a persona
// performs must carry a Playwright timing and at least one anchor,
// recorded as it happens, or it never becomes an admissible entry -- no
// later pass recovers what this module did not capture at the moment of
// the act.
import type { Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { type Anchor, type Entry, type Outcome, applyPerformanceBand, isAdmissible, requiresNarration } from './entry';

export interface ActSpec {
	// The journey step id this act is walked against, or an 'x'/'u'
	// number -- see entry.ts's Entry.id.
	id: string;
	act: string;
}

export interface ActOutcome {
	result: string;
	// Defaults to 'completed'. A persona-agent driving perform() already
	// knows whether the product refused her or she got stuck, so it
	// supplies the outcome and, whenever it isn't 'completed', the
	// Narrated line README.md requires for it.
	outcome?: Outcome;
	narrated?: string;
	// Anchors beyond the screenshot this module always takes -- a
	// file:line, an HTTP exchange, a schema reference.
	evidence?: Anchor[];
}

export type Capture =
	| { ok: true; entry: Entry; overTenSeconds: boolean }
	// The act happened but this module could not produce an admissible
	// entry for it -- the harness's own failure, README.md's 'u'-numbered
	// case. note goes straight into a uEntry.
	| { ok: false; id: string; note: string };

export interface ObserveOptions {
	// Where this persona's screenshots for this run live.
	shotsDir: string;
	// The persona's slug (docs/personas/<slug>.md), used in the
	// screenshot filename so two personas' shots never collide.
	slug: string;
	// Called only when the timing band pushes a 'completed' act to
	// 'completed with friction' and perform() had no narration to give,
	// because it did not yet know how slow the act would be. This is the
	// caller's one chance to have the persona name the wait in her own
	// voice, exactly as README.md's performance section requires.
	narrateWait?: (timingMs: number) => Promise<string> | string;
}

// observedAct runs perform(), times it, screenshots the page, and turns
// the result into an Entry -- applying the two performance bands and the
// narration rule along the way. It never writes to a log; persona-log.ts
// does that with whatever this returns.
export async function observedAct(page: Page, spec: ActSpec, perform: () => Promise<ActOutcome>, options: ObserveOptions): Promise<Capture> {
	const started = performance.now();
	let outcome: ActOutcome;
	try {
		outcome = await perform();
	} catch (error) {
		// The product itself stopped her -- an unhandled exception from the
		// act is exactly what 'stuck' means, not a harness failure.
		outcome = { result: error instanceof Error ? error.message : String(error), outcome: 'stuck' };
	}
	const timingMs = Math.round(performance.now() - started);

	mkdirSync(options.shotsDir, { recursive: true });
	const shotPath = path.join(options.shotsDir, `${options.slug}-${spec.id}.png`);
	try {
		await page.screenshot({ path: shotPath });
	} catch {
		return { ok: false, id: spec.id, note: `${spec.act}: the screenshot could not be captured` };
	}

	let entry: Entry = {
		id: spec.id,
		act: spec.act,
		result: outcome.result,
		outcome: outcome.outcome ?? 'completed',
		timingMs,
		evidence: [{ kind: 'screenshot', path: path.relative(process.cwd(), shotPath) }, ...(outcome.evidence ?? [])],
		narrated: outcome.narrated
	};

	if (!isAdmissible(entry)) {
		return { ok: false, id: spec.id, note: `${spec.act}: no anchor beyond the screenshot could be attached` };
	}

	const bandedOutcome = entry.outcome;
	const banded = applyPerformanceBand(entry);
	entry = banded.entry;

	if (requiresNarration(entry.outcome) && !entry.narrated) {
		// Either perform() already knew the outcome and owed narration, or
		// the band alone bumped a 'completed' act into friction -- the one
		// case this module can still recover, by asking the caller to name
		// the wait.
		if (bandedOutcome === 'completed' && options.narrateWait) {
			entry = { ...entry, narrated: await options.narrateWait(entry.timingMs) };
		} else {
			return { ok: false, id: spec.id, note: `${spec.act}: outcome ${entry.outcome} has no Narrated line` };
		}
	}
	if (entry.outcome === 'completed' && entry.narrated) {
		return { ok: false, id: spec.id, note: `${spec.act}: 'completed' must not carry a Narrated line` };
	}

	return { ok: true, entry, overTenSeconds: banded.overTenSeconds };
}

// An act that happened but could not be observed -- the screenshot was
// dropped, the network log was lost. A fact about the harness, never
// about the product (README.md), and never numbered on the same series
// as an improvised 'x' act for exactly that reason.
export function uEntry(uNumber: number, note: string): { id: string; note: string } {
	return { id: `u${uNumber}`, note };
}
