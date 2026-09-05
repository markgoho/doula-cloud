import { readFileSync, rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { jump } from './simulation/clock';
import { observedAct } from './simulation/capture';
import { PersonaLog } from './simulation/persona-log';
import { MAILBOX_URL, WORKER_SECRET, readSimulatedNow, startStack, stopStack } from './stack';
import { PREVIEW_SERVER_ORIGIN } from './ports';

// A scratch directory, not docs/simulation/runs/ -- this proves the
// harness's mechanics (#779, under map #759), not a real six-month walk
// against the World, so its output must never sit beside a genuine run
// and be mistaken for one. Gitignored (app/.gitignore's test-results),
// cleaned up at the end of the spec either way.
const RUNS_ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'test-results', 'simulation-harness-rehearsal');
const RUN_ID = 'rehearsal-1';

test.describe.serial('The harness spine: jump, capture, log, resume', () => {
	test.afterAll(() => {
		rmSync(RUNS_ROOT, { recursive: true, force: true });
	});

	test('a jump advances simulated time and drains the outboxes', async () => {
		const before = new Date(readSimulatedNow());
		const result = await jump({ value: 1, unit: 'days' }, { label: 'rehearsal day 1', workerSecret: WORKER_SECRET });
		const after = new Date(readSimulatedNow());

		expect(after.getTime(), 'sim.now() did not move').toBeGreaterThan(before.getTime());
		// A whole day crosses the 12-simulated-hour line every session's
		// Firebase ID token expires on (calendar.md's jump schedule).
		expect(result.crossedReauthThreshold).toBe(true);
	});

	test('one observed act renders and writes in the fixed format, and a slow one earns friction and narration', async ({ page }) => {
		const log = new PersonaLog({
			personaSlug: 'test-persona',
			personaLink: '../../../../docs/personas/test-persona.md',
			journeyLink: '../../../../docs/journeys/test-persona.md',
			runId: RUN_ID,
			commitSHA: 'rehearsal',
			summary: 'A harness rehearsal, not a persona walk against the World -- proves capture end to end.'
		});

		// No journey map owns this rehearsal, so both acts are off-map
		// ('x'-numbered) rather than borrowing a real step id (README.md,
		// "The unit: one act against one step").
		const opened = await observedAct(
			page,
			{ id: 'x1', act: `Opened the sandbox mailbox index page at ${MAILBOX_URL}/` },
			async () => {
				await page.goto(`${MAILBOX_URL}/`);
				await expect(page.getByRole('heading', { name: 'Sandbox mailboxes' })).toBeVisible();
				return { result: 'The mailbox index rendered with its run-clock label.' };
			},
			{ shotsDir: path.join(RUNS_ROOT, RUN_ID, 'shots'), slug: 'test-persona' }
		);
		expect(opened.ok).toBe(true);
		if (opened.ok) {
			expect(opened.entry.outcome).toBe('completed');
			log.recordOffMap(opened.entry);
		}

		// A deliberately slow act: the band bumps 'completed' to 'completed
		// with friction' on its own, and narrateWait is this module's one
		// chance to give the persona a line about the wait she never asked
		// for (README.md's performance section).
		const slow = await observedAct(
			page,
			{ id: 'x2', act: `Reopened the sandbox mailbox index page at ${MAILBOX_URL}/, slowly` },
			async () => {
				await page.waitForTimeout(1200);
				await page.goto(`${MAILBOX_URL}/`);
				return { result: 'The mailbox index rendered again, after the wait.' };
			},
			{
				shotsDir: path.join(RUNS_ROOT, RUN_ID, 'shots'),
				slug: 'test-persona',
				narrateWait: (timingMs) => `That took a while -- ${Math.round(timingMs / 100) / 10} s just to load an inbox with nothing in it.`
			}
		);
		expect(slow.ok).toBe(true);
		if (slow.ok) {
			expect(slow.entry.outcome).toBe('completed with friction');
			expect(slow.entry.narrated).toContain('That took a while');
			expect(slow.overTenSeconds).toBe(false);
			log.recordOffMap(slow.entry);
		}

		const written = log.writeTo(RUNS_ROOT);
		const rendered = log.render();

		expect(rendered).toContain('# test-persona — run rehearsal-1');
		expect(rendered).toContain('## Off-map acts');
		expect(rendered).toContain('**x1** — `completed` ·');
		expect(rendered).toContain('**x2** — `completed with friction` ·');
		expect(rendered).toContain('> That took a while');
		expect(rendered).toContain('- completed with friction: 1');

		expect(readFileSync(written, 'utf8')).toBe(rendered);
	});

	test('an act whose screenshot cannot be captured produces no admissible entry', async ({ context }) => {
		const closedPage = await context.newPage();
		await closedPage.goto(`${MAILBOX_URL}/`);
		await closedPage.close();

		const capture = await observedAct(
			closedPage,
			{ id: 'x3', act: 'Tried to act on a page that had already closed' },
			async () => ({ result: 'never reached -- the page is closed' }),
			{ shotsDir: path.join(RUNS_ROOT, RUN_ID, 'shots'), slug: 'test-persona' }
		);

		expect(capture.ok).toBe(false);
		if (!capture.ok) {
			expect(capture.id).toBe('x3');
			// A fact about the harness, not the product -- README.md's
			// 'u'-numbered case, never softened into a kept-but-flagged entry.
			expect(capture.note).toContain('screenshot');
		}
	});

	test('a run can be stopped with its volume kept and resumed with simulated time intact', async () => {
		const beforeStop = new Date(readSimulatedNow());

		stopStack({ keepVolume: true });
		await startStack(PREVIEW_SERVER_ORIGIN);

		const afterResume = new Date(readSimulatedNow());
		// The world -- sim.offset_row included -- survives a kept-volume
		// stop/start; only the host processes, which hold no state of their
		// own, were ever killed (stack.ts's stopStack).
		expect(afterResume.getTime()).toBeGreaterThanOrEqual(beforeStop.getTime());
	});
});
