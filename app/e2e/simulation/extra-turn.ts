// Drives one Extra's act (#821): a script, never a conversation, supplies
// the ActOutcome, with `outcome` left unset -- capture.ts defaults it to
// 'completed' and the timing bands still apply on top, exactly as for a
// Persona. She owes no Narrated register at any outcome (README.md,
// "Silence" -- extras.md carries Observed blocks only), which is
// capture.ts's own narration:'forbidden' option, not something this
// module has to police itself. ExtrasBook, not writeExtrasLog, owns the
// '<slug>-<n>' numbering (#821's own words).
import type { Page } from '@playwright/test';
import type { ActOutcome, Capture, ObserveOptions } from './capture';
import { observedAct } from './capture';
import type { ExtrasBook } from './extras';

export interface ExtraActSpec {
	// The step's plain description -- never a journey-map id, since an
	// Extra has no journey map to take one from.
	act: string;
}

export async function extraAct(
	page: Page,
	slug: string,
	spec: ExtraActSpec,
	extras: ExtrasBook,
	script: () => Promise<ActOutcome>,
	options: Omit<ObserveOptions, 'narrateWait' | 'slug' | 'narration'>
): Promise<Capture> {
	const id = extras.next(slug);
	const capture = await observedAct(page, { id, act: spec.act }, script, { ...options, slug, narration: 'forbidden' });
	if (capture.ok) {
		extras.record(slug, capture.entry);
	}
	return capture;
}
