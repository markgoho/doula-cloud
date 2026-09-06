// extras.md's numbering is the orchestrator's to own, not
// writeExtrasLog's (persona-log.ts only renders and enforces the
// no-narration rule -- #821). ExtrasBook hands out '<slug>-<n>' ids per
// Extra per run and collects her entries for the one shared
// writeExtrasLog call the run makes at the end.
import type { Entry } from './entry';

export class ExtrasBook {
	private readonly counters = new Map<string, number>();
	private readonly entriesBySlug = new Map<string, Entry[]>();

	// The next id for this Extra -- 'jo-1', 'jo-2', ... -- never the x/u
	// series, which belongs to a Persona's journey map and an Extra has
	// none (README.md, "The unit: one act against one step").
	next(slug: string): string {
		const n = (this.counters.get(slug) ?? 0) + 1;
		this.counters.set(slug, n);
		return `${slug}-${n}`;
	}

	record(slug: string, entry: Entry): void {
		const entries = this.entriesBySlug.get(slug) ?? [];
		entries.push(entry);
		this.entriesBySlug.set(slug, entries);
	}

	// The exact shape writeExtrasLog takes, in first-recorded order (a Map
	// already preserves insertion order).
	entries(): Map<string, Entry[]> {
		return this.entriesBySlug;
	}
}
