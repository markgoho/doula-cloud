// The friction-log entry, and the rules docs/simulation/README.md (#760)
// fixes for one: the four outcomes, the two performance bands, and the
// admissibility bar that deletes rather than softens. Part of building
// the harness (#779, under map #759).
//
// This module holds the instrument's data shape and the rules that apply
// to it uniformly; persona-log.ts holds the fixed structure a whole log
// renders into.

export type Outcome = 'completed' | 'completed with friction' | 'refused' | 'stuck';

// The four anchor kinds README.md names other than a timing, which is
// always present as its own field (see Entry.timingMs) and so never
// counted here as the only anchor for a claim about behaviour.
export type Anchor =
	| { kind: 'screenshot'; path: string }
	| { kind: 'file'; ref: string }
	| { kind: 'http'; exchange: string }
	| { kind: 'schema'; ref: string };

export interface Entry {
	// The journey step id ('3.2'), an appended check ('3.2-a'), an
	// off-map act ('x1'), or a harness failure ('u1') -- README.md's three
	// marked ids. A run never renumbers a map, so this is supplied by the
	// caller, not generated here.
	id: string;
	act: string;
	result: string;
	outcome: Outcome;
	timingMs: number;
	evidence: Anchor[];
	// First person, present tense, evidence-free. Required whenever
	// outcome is not 'completed' (README.md), forbidden when it is.
	narrated?: string;
}

// Nielsen's response-time limits, fixed by README.md and never moved --
// moving either would silently reclassify every entry in every past run.
const FRICTION_BAND_MS = 1000;
const FINDING_BAND_MS = 10_000;

export function requiresNarration(outcome: Outcome): boolean {
	return outcome !== 'completed';
}

// Applies the two timing bands to an already-built entry, exactly as
// README.md specifies: the band decides, not the agent. Never downgrades
// an outcome the act itself produced (a refusal under 1 s is still a
// refusal), only ever bumps a 'completed' entry up to 'completed with
// friction' when the measured timing crosses a band.
//
// Returns whether the entry crossed the 10 s band, which is the caller's
// signal to file a finding with no discretion -- filing itself is
// findings.md's pipeline (#766), run only after a run closes, so this
// function never files anything itself.
export function applyPerformanceBand(entry: Entry): { entry: Entry; overTenSeconds: boolean } {
	if (entry.timingMs < FRICTION_BAND_MS) {
		return { entry, overTenSeconds: false };
	}
	const outcome: Outcome = entry.outcome === 'completed' ? 'completed with friction' : entry.outcome;
	return { entry: { ...entry, outcome }, overTenSeconds: entry.timingMs >= FINDING_BAND_MS };
}

// README.md's whole admissibility mechanism: an entry that cannot meet
// the anchor bar is deleted, never weakened. isAdmissible is the check a
// caller runs before ever writing an entry into a log; an inadmissible
// one is not corrected here, because there is no correction -- see
// capture.ts's uEntry for the honest replacement, a 'u'-numbered fact
// about the harness.
export function isAdmissible(entry: Entry): boolean {
	return entry.evidence.length > 0 && Number.isFinite(entry.timingMs);
}

// The mandatory per-stage line (README.md, "Silence, and the line that
// breaks it"): every stage closes with one, carrying the act count so an
// unremarkable stage and a skipped one never look identical.
export function stageLine(stageTitle: string, entries: Entry[]): string {
	const friction = entries.filter((entry) => entry.outcome !== 'completed').length;
	const tail = friction === 0 ? ' Nothing to report.' : '';
	return `**${stageTitle}.** ${entries.length} acts, ${friction} with friction.${tail}`;
}

function anchorText(anchor: Anchor): string {
	switch (anchor.kind) {
		case 'screenshot': {
			return anchor.path;
		}
		case 'file': {
			return anchor.ref;
		}
		case 'http': {
			return anchor.exchange;
		}
		case 'schema': {
			return anchor.ref;
		}
	}
}

// Renders one entry in the worked-example's block form (README.md): a
// header line, the Observed fields, and -- only where outcome requires
// it -- a Narrated blockquote directly beneath. Tables are the other
// legal form the spec allows; blocks are used unconditionally here
// because they never become unreadable, which a table can (README.md,
// "Fixed structure of a persona log").
export function renderEntry(entry: Entry): string {
	const lines = [
		`**${entry.id}** — \`${entry.outcome}\` · ${entry.timingMs.toLocaleString('en-US')} ms`,
		`**Act**: ${entry.act}`,
		`**Result**: ${entry.result}`,
		`**Evidence**: ${entry.evidence.map((anchor) => anchorText(anchor)).join('; ')}`
	];
	if (entry.narrated) {
		lines.push('', `> ${entry.narrated}`);
	}
	return lines.join('\n');
}
