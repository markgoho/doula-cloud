import { execFileSync } from 'node:child_process';
import { existsSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const EMULATOR_PIDFILE = join(tmpdir(), 'doula-cloud-e2e-firebase-emulator.pid');

export default function globalTeardown() {
	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'down', '-v'], {
		stdio: 'inherit',
		timeout: 60_000
	});
	stopEmulator();
}

function stopEmulator() {
	if (!existsSync(EMULATOR_PIDFILE)) return;
	const pid = Number(readFileSync(EMULATOR_PIDFILE, 'utf8'));
	try {
		process.kill(pid);
	} catch {
		// Already gone -- nothing to clean up.
	}
	rmSync(EMULATOR_PIDFILE, { force: true });
}
