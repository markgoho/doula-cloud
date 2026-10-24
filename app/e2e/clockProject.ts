// The specs that move the stack's clock, and the Playwright project they
// run in (#1833). The BFF's clock follows sim.offset_row on the e2e stack
// (api/internal/stackswitch), so a jump moves every date the BFF reads
// for every spec at once, and a jump of 12 hours or more ends every Staff
// session (authn.SessionLifetime). A spec that jumps cannot share the
// stack with one that does not.
//
// playwright.config.ts runs these files in their own project, with one
// worker, after every other spec has finished (`dependencies`), and keeps
// them out of the parallel project. jump() (simulation/clock.ts) and
// restartAPI() (stack.ts) call requireClockProject, so a new spec that
// jumps without being listed here fails at the jump, not at another
// spec's sign-in.
import { test } from '@playwright/test';

export const CLOCK_PROJECT = 'clock';
export const PARALLEL_PROJECT = 'parallel';

// File names under app/e2e. A file is listed whole: a describe block in
// it that does not jump runs alone too, which costs a few seconds.
export const CLOCK_SPECS = ['simulation-harness.e2e.ts', 'simulation-orchestrator.e2e.ts', 'terms-agreement.e2e.ts'];

// Throws unless the running test belongs to the clock project.
export function requireClockProject(act: string): void {
	const project = test.info().project.name;
	if (project !== CLOCK_PROJECT) {
		throw new Error(
			`${act} moves what every spec on the stack reads, so it runs only in the "${CLOCK_PROJECT}" project, not "${project}": add the spec file to CLOCK_SPECS in e2e/clockProject.ts`
		);
	}
}
