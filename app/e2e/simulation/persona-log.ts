// Assembles one persona's log into docs/simulation/README.md's fixed
// structure -- header, stages in map order each closing with its
// mandatory stage line, an off-map section for x/u entries, and a
// counts section -- and writes it under a run directory. Built for #779
// under map #759; entry.ts holds the rules this module only arranges.
import { mkdirSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { type Entry, type Outcome, renderEntry, stageLine } from './entry';

export interface PersonaLogHeader {
	personaSlug: string;
	// Links are caller-supplied rather than computed, because they're
	// relative to wherever runsRoot ends up mounted (docs/simulation/runs/
	// for a real run, a scratch directory for a rehearsal) and this module
	// has no way to know which.
	personaLink: string;
	journeyLink: string;
	runId: string;
	commitSHA: string;
	// What this persona was doing in this run -- README.md's header field,
	// free text.
	summary: string;
}

// A harness failure has no Entry -- the screenshot or network log this
// module needed was itself the thing that went missing, so there is
// nothing to render but the fact and the note (README.md's 'u'-numbered
// case).
export interface HarnessFailure {
	id: string;
	note: string;
}

// PersonaLog accumulates one persona's walk and renders it once, in the
// order stages were added -- "walk the stages in map order, always"
// (README.md, Comparing one run against the next). Nothing here reorders
// what the caller gave it.
export class PersonaLog {
	private readonly stages: { title: string; entries: Entry[] }[] = [];
	private readonly offMap: Entry[] = [];
	private readonly harnessFailures: HarnessFailure[] = [];

	constructor(private readonly header: PersonaLogHeader) {}

	private counts(): Record<Outcome, number> {
		const all = [...this.stages.flatMap((stage) => stage.entries), ...this.offMap];
		const base: Record<Outcome, number> = { completed: 0, 'completed with friction': 0, refused: 0, stuck: 0 };
		for (const entry of all) base[entry.outcome]++;
		return base;
	}

	// Opens a new stage bucket. Every record() until the next addStage()
	// lands in this one.
	addStage(title: string): void {
		this.stages.push({ title, entries: [] });
	}

	// Records an in-map entry against the most recently opened stage.
	record(entry: Entry): void {
		const current = this.stages.at(-1);
		if (!current) {
			throw new Error(`persona-log: record(${entry.id}) called before any addStage()`);
		}
		current.entries.push(entry);
	}

	// An 'x'-numbered off-map act: the persona improvised, or the run threw
	// something the map never anticipated.
	recordOffMap(entry: Entry): void {
		this.offMap.push(entry);
	}

	recordHarnessFailure(failure: HarnessFailure): void {
		this.harnessFailures.push(failure);
	}

	render(): string {
		const sections: string[] = [
			`# ${this.header.personaSlug} — run ${this.header.runId}`,
			'',
			`Persona: [${this.header.personaSlug}](${this.header.personaLink}) · Journey: [${this.header.personaSlug}](${this.header.journeyLink}) · Commit: \`${this.header.commitSHA}\``,
			'',
			this.header.summary
		];

		for (const stage of this.stages) {
			sections.push('', `## ${stage.title}`, '', ...stage.entries.map((entry) => renderEntry(entry) + '\n'), stageLine(stage.title, stage.entries));
		}

		if (this.offMap.length > 0 || this.harnessFailures.length > 0) {
			sections.push(
				'',
				'## Off-map acts',
				'',
				...this.offMap.map((entry) => renderEntry(entry) + '\n'),
				// A 'u' entry is a fact about the harness, not the product, so
				// it carries no Observed block -- just its id and what went
				// wrong.
				...this.harnessFailures.map((failure) => `**${failure.id}** — not observed: ${failure.note}`)
			);
		}

		const counts = this.counts();
		sections.push(
			'',
			'## Counts',
			'',
			...Object.entries(counts).map(([outcome, count]) => `- ${outcome}: ${count}`),
			`- not observed (u): ${this.harnessFailures.length}`
		);

		return sections.join('\n') + '\n';
	}

	// Writes this log to <runsRoot>/<runId>/<personaSlug>.md and returns
	// the path written. runsRoot is the caller's to choose: docs/simulation/runs
	// for a real run, a scratch directory for a rehearsal that must never
	// be mistaken for one.
	writeTo(runsRoot: string): string {
		const runDirectory = path.join(runsRoot, this.header.runId);
		mkdirSync(runDirectory, { recursive: true });
		const filePath = path.join(runDirectory, `${this.header.personaSlug}.md`);
		writeFileSync(filePath, this.render());
		return filePath;
	}
}

// extras.md's shape (#766): Observed blocks only, no Narrated register at
// any outcome, one shared file per run. Entries are numbered per Extra
// per run by the caller (e.g. "jo-1", "jo-2"), because an Extra has no
// journey map to take step ids from.
export function writeExtrasLog(runsRoot: string, runId: string, entriesBySlug: Map<string, Entry[]>): string {
	const runDirectory = path.join(runsRoot, runId);
	mkdirSync(runDirectory, { recursive: true });
	const filePath = path.join(runDirectory, 'extras.md');
	const sections: string[] = [`# Extras — run ${runId}`];
	for (const [slug, entries] of entriesBySlug) {
		sections.push('', `## ${slug}`, '');
		for (const entry of entries) {
			if (entry.narrated) {
				throw new Error(`persona-log: ${slug}'s ${entry.id} carries a Narrated line, which extras.md forbids`);
			}
			sections.push(renderEntry(entry) + '\n');
		}
	}
	writeFileSync(filePath, sections.join('\n') + '\n');
	return filePath;
}
