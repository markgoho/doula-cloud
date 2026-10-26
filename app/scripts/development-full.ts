// `bun run dev:full` -- brings up the local stack (Firebase Auth
// emulator + the podman-compose Postgres/API stack, see e2e/stack.ts)
// and then runs the SvelteKit dev server against it. Tears the stack
// down again on Ctrl-C or if the dev server exits on its own.
//
// Between the two it seeds the local demo Practice (#1994,
// demo-practice.ts), so a person can sign in from /login at once; the
// stack starts empty each time, so this runs on every start. SEED_DEMO=0
// skips it. Only dev:full seeds it: the e2e suite starts its stack from
// global-setup.ts, which never reaches this file.
import { request } from '@playwright/test';
import { startStack, stopStack } from '../e2e/stack';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, DEV_SERVER_ORIGIN } from '../e2e/ports';
import { demoSummary, mintPortalLink, seedDemoPractice } from './demo-practice';

await startStack(DEV_SERVER_ORIGIN);

if (process.env.SEED_DEMO !== '0') {
	const context = await request.newContext();
	try {
		const practice = await seedDemoPractice(context);
		console.log(demoSummary(practice, await mintPortalLink(context)));
	} catch (error) {
		stopStack();
		throw error;
	} finally {
		await context.dispose();
	}
}

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
	stopStack();
}

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
	process.on(signal, () => {
		vite.kill(signal);
	});
}
