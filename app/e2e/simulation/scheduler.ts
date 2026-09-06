// One clock over the whole cast (#821): acts from different personas
// interleave, and never run concurrently, except a named simultaneity
// probe (docs/simulation/calendar.md). Which act comes from which
// persona, and in what order, is the caller's own script -- this
// module's only job is to hold that one-at-a-time rule, and to relax it
// deliberately for a step the caller has marked as a probe.
export type ScheduledAct = () => Promise<unknown>;

export type ScheduleStep =
	| { kind: 'act'; run: ScheduledAct }
	// id names the probe from calendar.md's five-row table (e.g. 'P1' --
	// "Two labors, one night"), kept for the run README's record. This
	// module does not validate it against that table -- building the
	// probes themselves is #821's own out-of-scope, not this function's.
	| { kind: 'probe'; id: string; run: ScheduledAct[] };

// Runs steps in order. An 'act' step is awaited before the next step
// starts, so two ordinary acts are never in flight together. A 'probe'
// step's acts are launched together via Promise.all -- genuine
// concurrency, deliberately, because that is what a probe exists to
// prove happened.
export async function runSchedule(steps: ScheduleStep[]): Promise<void> {
	for (const step of steps) {
		if (step.kind === 'probe') {
			await Promise.all(step.run.map((act) => act()));
		} else {
			await step.run();
		}
	}
}
