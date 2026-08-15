// `bun run dev:full` -- brings up the local stack (Firebase Auth
// emulator + the podman-compose Postgres/API stack, see e2e/stack.ts)
// and then runs the SvelteKit dev server against it. Tears the stack
// down again on Ctrl-C or if the dev server exits on its own.
import { spawn } from 'node:child_process';
import { startStack, stopStack } from '../e2e/stack';
import { E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from '../e2e/ports';

await startStack();

const vite = spawn('bun', ['run', 'dev'], {
	stdio: 'inherit',
	env: {
		...process.env,
		VITE_FIREBASE_AUTH_EMULATOR_HOST: `${E2E_EMULATOR_HOST}:${E2E_EMULATOR_PORT}`
	}
});

let tornDown = false;
function teardown() {
	if (tornDown) return;
	tornDown = true;
	stopStack();
}

vite.on('exit', (code) => {
	teardown();
	process.exit(code ?? 0);
});

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
	process.on(signal, () => {
		vite.kill(signal);
	});
}
