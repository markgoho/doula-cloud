// `bun run dev:full` -- brings up the local stack (Firebase Auth
// emulator + the podman-compose Postgres/API stack, see e2e/stack.ts)
// and then runs the SvelteKit dev server against it. Tears the stack
// down again on Ctrl-C or if the dev server exits on its own.
//
// It starts from a clean stack each time (#2010): a dev:full that was
// killed without its teardown leaves the database volume behind, while
// the Auth emulator keeps nothing between starts, so the old rows would
// outlive every account that signs in to them. stopStack first removes
// what such a run left, and it costs nothing when there is nothing left.
//
// Between the stack and the dev server it seeds the local demo Practice
// (#1994, demo-practice.ts), so a person can sign in from /login at once;
// SEED_DEMO=0 skips it. Only dev:full seeds it: the e2e suite starts its
// stack from global-setup.ts, which never reaches this file.
//
// While the dev server runs, the outboxes are drained on a timer (#2010),
// the local stand-in for Cloud Scheduler's process-outbox-drain job: the
// same route, POST /api/internal/outboxes/drain, which runs every
// registered outbox. So mail a person causes in the browser -- a magic
// link, an Invitation, a reset -- reaches the sandbox mailbox.
import { request } from '@playwright/test';
import { MAILBOX_URL, startStack, stopStack, WORKER_SECRET } from '../e2e/stack';
import { API_URL, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, DEV_SERVER_ORIGIN } from '../e2e/ports';
import { demoSummary, mintPortalLinks, seedDemoPractice } from './demo-practice';

// Deployed, the drain runs every five minutes, as a backstop under the
// Cloud Tasks nudge that sends most mail at once. Locally there is no
// nudge, so the drain is the only thing that sends anything, and a
// person waits at /portal/login for the link: a few seconds, not five
// minutes.
const DRAIN_INTERVAL_MS = 3000;

stopStack({ quiet: true });
await startStack(DEV_SERVER_ORIGIN);

if (process.env.SEED_DEMO !== '0') {
	const context = await request.newContext();
	try {
		const practice = await seedDemoPractice(context);
		console.log(demoSummary(practice, await mintPortalLinks(context)));
	} catch (error) {
		stopStack();
		throw error;
	} finally {
		await context.dispose();
	}
}

// One drain at a time: a tick that comes while the last drain still runs
// is skipped, not stacked behind it. A failed drain is logged with the
// BFF's own answer (a 500 names the outbox that failed) and the timer
// keeps going, as the Scheduler job keeps its schedule.
let isDraining = false;
async function drainOutboxes() {
	if (isDraining) return;
	// eslint-disable-next-line unicorn/no-top-level-assignment-in-function -- the one-at-a-time guard every tick reads
	isDraining = true;
	try {
		const response = await fetch(`${API_URL}/api/internal/outboxes/drain`, {
			method: 'POST',
			headers: { 'X-Internal-Secret': WORKER_SECRET }
		});
		if (!response.ok) {
			console.error(`dev:full: the outbox drain answered ${response.status}: ${await response.text()}`);
		}
	} catch (error) {
		console.error('dev:full: the outbox drain failed:', error);
	} finally {
		// eslint-disable-next-line unicorn/no-top-level-assignment-in-function -- see the guard above
		isDraining = false;
	}
}
const drainTimer = setInterval(drainOutboxes, DRAIN_INTERVAL_MS);
console.log(`Local mail is drained every ${DRAIN_INTERVAL_MS / 1000}s. Read it at ${MAILBOX_URL}/\n`);

const vite = Bun.spawn(['bun', 'run', 'dev'], {
	stdio: ['inherit', 'inherit', 'inherit'],
	env: {
		...process.env,
		VITE_FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`
	},
	onExit(_subprocess, exitCode) {
		teardown();
		// eslint-disable-next-line unicorn/no-process-exit -- forwards the dev server's own exit code to the parent shell
		process.exit(exitCode ?? 0);
	}
});

let isTornDown = false;
function teardown() {
	if (isTornDown) return;
	// eslint-disable-next-line unicorn/no-top-level-assignment-in-function -- one-shot guard against a double teardown from both the exit handler and a signal handler
	isTornDown = true;
	clearInterval(drainTimer);
	stopStack();
}

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
	process.on(signal, () => {
		vite.kill(signal);
	});
}
