// Exercises the orchestrator built for #821: one BrowserContext and one
// mailbox per cast member, a Persona's outcome/narration coming from her
// own conversation, an Extra's from a script, the extras.md numbering,
// re-authentication gated on a jump crossing the reauth threshold, and
// the one-clock scheduler's probe escape hatch. A FakePersonaConversation
// backs every test here -- #821 forbids a spec from calling a live model.
import { rmSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { expect, test } from '@playwright/test';
import { readFileSync, readdirSync } from 'node:fs';
import { Cast } from './simulation/cast';
import type { ActOutcome } from './simulation/capture';
import { jump } from './simulation/clock';
import { extraAct } from './simulation/extra-turn';
import { ExtrasBook } from './simulation/extras';
import { personaAct } from './simulation/persona-turn';
import type { PersonaConversation, PersonaJudgment, PersonaTurnRecord, PersonaTurnResult } from './simulation/persona-conversation';
import { runSchedule } from './simulation/scheduler';
import { writeExtrasLog } from './simulation/persona-log';
import { WORKER_SECRET } from './stack';

const RUNS_ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'test-results', 'simulation-orchestrator-rehearsal');
const RUN_ID = 'rehearsal-1';
const SHOTS_DIR = path.join(RUNS_ROOT, RUN_ID, 'shots');

test.afterAll(() => {
	rmSync(RUNS_ROOT, { recursive: true, force: true });
});

// A conversation double: judge() and narrateWait() answer from a queue the
// test controls, and record() just keeps everything it was ever told so a
// test can assert on it -- standing in for the real
// AnthropicPersonaConversation without any network or SDK involved.
class FakePersonaConversation implements PersonaConversation {
	private readonly queue: PersonaJudgment[];
	readonly judged: PersonaTurnRecord[] = [];
	readonly recorded: PersonaTurnResult[] = [];

	constructor(
		readonly slug: string,
		judgments: PersonaJudgment[] = [{ outcome: 'completed' }]
	) {
		this.queue = [...judgments];
	}

	async judge(record: PersonaTurnRecord): Promise<PersonaJudgment> {
		this.judged.push(record);
		return this.queue.shift() ?? { outcome: 'completed' };
	}

	async narrateWait(timingMs: number): Promise<string> {
		return `That took ${timingMs} ms and I noticed.`;
	}

	record(result: PersonaTurnResult): void {
		this.recorded.push(result);
	}
}

test.describe('Cast: one persistent context and mailbox per cast member', () => {
	test('admits distinct contexts, refuses a repeat slug, and closes everything', async ({ browser }) => {
		const cast = new Cast(browser, async () => {});
		const renata = await cast.admit('practice-owner', 'renata@sim.doula.cloud');
		const jo = await cast.admit('jo', 'jo@sim.doula.cloud');

		expect(renata.context).not.toBe(jo.context);
		expect(renata.page).not.toBe(jo.page);
		expect(cast.get('practice-owner')).toBe(renata);
		expect(() => cast.get('nobody')).toThrow('never admitted');
		await expect(cast.admit('jo', 'jo@sim.doula.cloud')).rejects.toThrow('already admitted');

		await cast.closeAll();
		expect(cast.all()).toHaveLength(0);
	});

	test('two personas writing to the same shots directory never collide', async ({ browser }) => {
		const cast = new Cast(browser, async () => {});
		const renata = await cast.admit('practice-owner', 'renata@sim.doula.cloud');
		const jo = await cast.admit('jo', 'jo@sim.doula.cloud');
		await renata.page.setContent('<h1>Renata</h1>');
		await jo.page.setContent('<h1>Jo</h1>');

		const renataConversation = new FakePersonaConversation('practice-owner');
		const joConversation = new FakePersonaConversation('jo');

		const [renataCapture, joCapture] = await Promise.all([
			personaAct(renata.page, { id: '1.1', act: 'Opened her dashboard' }, renataConversation, async () => ({ result: 'Rendered.' }), { shotsDir: SHOTS_DIR }),
			personaAct(jo.page, { id: '1.1', act: 'Opened her dashboard' }, joConversation, async () => ({ result: 'Rendered.' }), { shotsDir: SHOTS_DIR })
		]);

		expect(renataCapture.ok).toBe(true);
		expect(joCapture.ok).toBe(true);
		if (renataCapture.ok && joCapture.ok) {
			const renataShot = renataCapture.entry.evidence.find((a) => a.kind === 'screenshot');
			const joShot = joCapture.entry.evidence.find((a) => a.kind === 'screenshot');
			expect(renataShot?.path).toContain('practice-owner-1.1');
			expect(joShot?.path).toContain('jo-1.1');
			expect(renataShot?.path).not.toBe(joShot?.path);
		}

		await cast.closeAll();
	});

	test('re-authenticates only an open session that a jump marked due, before its next act', async ({ browser }) => {
		const signIns: string[] = [];
		const cast = new Cast(browser, async (member) => {
			signIns.push(member.slug);
		});
		const renata = await cast.admit('practice-owner', 'renata@sim.doula.cloud');
		await cast.admit('jo', 'jo@sim.doula.cloud'); // never signed in this run

		await cast.recordSignedIn('practice-owner', '2027-01-04T09:00:00.000Z');
		expect(renata.signedIn).toBe(true);
		expect(renata.lastAuthenticatedAt).toBe('2027-01-04T09:00:00.000Z');

		// A jump under the 12h reauth threshold: nothing is marked due, and
		// signInIfDue is a no-op.
		await cast.signInIfDue('practice-owner', '2027-01-04T10:00:00.000Z');
		expect(signIns).toEqual([]);

		// clock.ts's jump() reported crossedReauthThreshold -- the caller's
		// job, per its own header comment, is exactly this call.
		cast.markReauthDue();
		expect(cast.get('jo').reauthDue).toBe(false); // never signed in, nothing to re-authenticate

		await cast.signInIfDue('practice-owner', '2027-01-05T09:00:00.000Z');
		expect(signIns).toEqual(['practice-owner']);
		expect(cast.get('practice-owner').reauthDue).toBe(false);
		expect(cast.get('practice-owner').lastAuthenticatedAt).toBe('2027-01-05T09:00:00.000Z');

		// A second act with nothing new due signs nobody in again.
		await cast.signInIfDue('practice-owner', '2027-01-05T09:05:00.000Z');
		expect(signIns).toEqual(['practice-owner']);

		await cast.closeAll();
	});

	test('a real jump crossing the reauth threshold makes act() sign in again before the next act runs', async ({ browser }) => {
		const order: string[] = [];
		const cast = new Cast(browser, async (member) => {
			order.push(`signIn:${member.slug}`);
		});
		await cast.admit('practice-owner', 'renata@sim.doula.cloud');
		await cast.recordSignedIn('practice-owner', '2027-01-04T09:00:00.000Z');

		// The real clock.ts jump(), against the stack this suite already
		// shares -- not a hand-typed ISO string standing in for one.
		const result = await jump({ value: 1, unit: 'days' }, { label: 'orchestrator rehearsal', workerSecret: WORKER_SECRET });
		expect(result.crossedReauthThreshold).toBe(true);

		// The AC this proves: afterJump() is what turns a jump's own report
		// into "every open session re-authenticates", and act() is what
		// makes that true before the next act runs, with no caller having
		// to remember to call signInIfDue() itself.
		cast.afterJump(result);
		expect(cast.get('practice-owner').reauthDue).toBe(true);

		await cast.act('practice-owner', async () => {
			order.push('act:practice-owner');
		});

		expect(order).toEqual(['signIn:practice-owner', 'act:practice-owner']);
		expect(cast.get('practice-owner').reauthDue).toBe(false);

		await cast.closeAll();
	});
});

test.describe('A Persona turn: her own conversation judges outcome and narration', () => {
	test('a completed act needs no judgment beyond hers, and is recorded into her history', async ({ page }) => {
		const conversation = new FakePersonaConversation('practice-owner', [{ outcome: 'completed' }]);
		const capture = await personaAct(page, { id: '2.1', act: 'Opened the invite form' }, conversation, async () => ({ result: 'The form rendered.' }), {
			shotsDir: SHOTS_DIR
		});

		expect(capture.ok).toBe(true);
		expect(conversation.judged).toEqual([{ id: '2.1', act: 'Opened the invite form', result: 'The form rendered.' }]);
		if (capture.ok) {
			expect(capture.entry.outcome).toBe('completed');
			expect(conversation.recorded).toEqual([
				{ id: '2.1', act: 'Opened the invite form', result: 'The form rendered.', outcome: 'completed', narrated: undefined }
			]);
		}
	});

	test('a refused act carries the narration her conversation gave it', async ({ page }) => {
		const conversation = new FakePersonaConversation('practice-owner', [
			{ outcome: 'refused', narrated: "It stopped me and didn't say why." }
		]);
		const capture = await personaAct(
			page,
			{ id: '2.2-a', act: 'Submitted the same invitation twice' },
			conversation,
			async () => ({ result: 'POST /api/practices/{id}/invitations -> 409 conflict', evidence: [{ kind: 'http', exchange: 'POST -> 409' }] }),
			{ shotsDir: SHOTS_DIR }
		);

		expect(capture.ok).toBe(true);
		if (capture.ok) {
			expect(capture.entry.outcome).toBe('refused');
			expect(capture.entry.narrated).toBe("It stopped me and didn't say why.");
		}
	});

	test('a script exception is stuck, and still narrated in her voice, not swallowed as a bare harness failure', async ({ page }) => {
		const conversation = new FakePersonaConversation('practice-owner', [{ outcome: 'stuck', narrated: 'The button did nothing.' }]);
		const capture = await personaAct(
			page,
			{ id: 'x1', act: 'Tried to click a control that was never there' },
			conversation,
			async () => {
				throw new Error('locator not found');
			},
			{ shotsDir: SHOTS_DIR }
		);

		expect(capture.ok).toBe(true);
		expect(conversation.judged[0].result).toBe('locator not found');
		if (capture.ok) {
			expect(capture.entry.outcome).toBe('stuck');
			expect(capture.entry.narrated).toBe('The button did nothing.');
		}
	});

	test("a band-induced friction bump is narrated by her conversation's narrateWait, not judge", async ({ page }) => {
		const conversation = new FakePersonaConversation('practice-owner', [{ outcome: 'completed' }]);
		const capture = await personaAct(
			page,
			{ id: '2.3', act: 'Waited on a slow save' },
			conversation,
			async () => {
				await page.waitForTimeout(1100);
				return { result: 'It eventually saved.' };
			},
			{ shotsDir: SHOTS_DIR }
		);

		expect(capture.ok).toBe(true);
		if (capture.ok) {
			expect(capture.entry.outcome).toBe('completed with friction');
			expect(capture.entry.narrated).toContain('That took');
		}
	});
});

test.describe('An Extra turn: a script, no conversation, no Narrated register ever', () => {
	test('numbers per Extra per run, and never carries narration at any outcome', async ({ page }) => {
		const extras = new ExtrasBook();
		const outcomes: ActOutcome[] = [
			{ result: 'Logged a Visit.' },
			{ result: 'The Client record refused a save.', outcome: 'refused' },
			{ result: 'Nothing on screen responded.', outcome: 'stuck' }
		];

		for (const outcome of outcomes) {
			const capture = await extraAct(page, 'jo', { act: 'Ordinary Tuesday work' }, extras, async () => outcome, { shotsDir: SHOTS_DIR });
			expect(capture.ok).toBe(true);
			if (capture.ok) {
				expect(capture.entry.narrated).toBeUndefined();
			}
		}

		const entries = extras.entries().get('jo') ?? [];
		expect(entries.map((entry) => entry.id)).toEqual(['jo-1', 'jo-2', 'jo-3']);
		expect(entries.map((entry) => entry.outcome)).toEqual(['completed', 'refused', 'stuck']);
	});

	test('numbering is independent per Extra, and feeds writeExtrasLog directly', async ({ page }) => {
		const extras = new ExtrasBook();
		await extraAct(page, 'jo', { act: 'First act' }, extras, async () => ({ result: 'Done.' }), { shotsDir: SHOTS_DIR });
		await extraAct(page, 'dee', { act: 'First act' }, extras, async () => ({ result: 'Done.' }), { shotsDir: SHOTS_DIR });
		await extraAct(page, 'jo', { act: 'Second act' }, extras, async () => ({ result: 'Done.' }), { shotsDir: SHOTS_DIR });

		expect((extras.entries().get('jo') ?? []).map((entry) => entry.id)).toEqual(['jo-1', 'jo-2']);
		expect((extras.entries().get('dee') ?? []).map((entry) => entry.id)).toEqual(['dee-1']);

		const written = writeExtrasLog(RUNS_ROOT, 'extras-numbering', extras.entries());
		expect(readFileSync(written, 'utf8')).toContain('## jo');
		expect(readFileSync(written, 'utf8')).toContain('## dee');
	});
});

// Shared by both scheduler tests below -- pulled to module scope so
// neither test defines a nested closure that only ever reads its own
// parameters.
interface ConcurrencyProbe {
	inFlight: number;
	maxInFlight: number;
	order: string[];
}

function trackedAct(probe: ConcurrencyProbe, label: string, delayMs: number): () => Promise<void> {
	return async () => {
		probe.inFlight++;
		probe.maxInFlight = Math.max(probe.maxInFlight, probe.inFlight);
		await new Promise((resolve) => setTimeout(resolve, delayMs));
		probe.order.push(label);
		probe.inFlight--;
	};
}

test.describe('The scheduler: one clock, interleaved, except a named probe', () => {
	test('ordinary steps never overlap', async () => {
		const probe: ConcurrencyProbe = { inFlight: 0, maxInFlight: 0, order: [] };

		await runSchedule([
			{ kind: 'act', run: trackedAct(probe, 'renata-1', 5) },
			{ kind: 'act', run: trackedAct(probe, 'jo-1', 5) },
			{ kind: 'act', run: trackedAct(probe, 'renata-2', 5) }
		]);

		expect(probe.maxInFlight).toBe(1);
		expect(probe.order).toEqual(['renata-1', 'jo-1', 'renata-2']);
	});

	test('a probe step runs its acts genuinely concurrently', async () => {
		const probe: ConcurrencyProbe = { inFlight: 0, maxInFlight: 0, order: [] };

		await runSchedule([{ kind: 'probe', id: 'P1', run: [trackedAct(probe, 'priya', 20), trackedAct(probe, 'lena', 20)] }]);

		expect(probe.maxInFlight).toBeGreaterThan(1);
	});
});

test('no orchestrator source file invokes gh or reads a tracker token', () => {
	const simulationDirectory = path.join(path.dirname(fileURLToPath(import.meta.url)), 'simulation');
	const forbidden = [/\bgh\b/, /GH_TOKEN/, /GITHUB_TOKEN/, /octokit/i, /api\.github\.com/, /child_process/, /execSync|spawnSync/];
	const files = readdirSync(simulationDirectory).filter((file) => file.endsWith('.ts'));
	expect(files.length).toBeGreaterThan(0);
	for (const file of files) {
		const contents = readFileSync(path.join(simulationDirectory, file), 'utf8');
		for (const pattern of forbidden) {
			expect(contents, `${file} matched ${pattern}`).not.toMatch(pattern);
		}
	}
});
