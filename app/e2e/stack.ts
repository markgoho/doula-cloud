import { execFileSync, spawn } from 'node:child_process';
import { createConnection } from 'node:net';
import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

const DB_HOST = '127.0.0.1';
const DB_PORT = 15432;
const READY_TIMEOUT_MS = 60_000;

// The Firebase Auth emulator (see firebase.json) is plain Node with no
// Postgres dependency, so it runs as a host process rather than another
// compose service. Shared by the Playwright e2e run
// (global-setup.ts/global-teardown.ts) and `bun run dev:full`
// (scripts/dev-full.ts) for local interactive use.
const EMULATOR_PIDFILE = join(tmpdir(), 'doula-cloud-e2e-firebase-emulator.pid');

// Brings up the self-contained podman-compose stack -- Postgres, the
// goose migration step, the app_e2e login role, and the Go BFF itself
// (see compose.e2e.yaml) -- plus the Firebase Auth emulator.
//
// Deliberately doesn't use `up --wait`: under this runner's rootless
// Podman + docker-compose-as-provider combination, the container starts
// fine but its healthcheck never reports "healthy", so --wait blocks
// forever (confirmed on trunk -- see git log for this file). Polling the
// host-exposed ports directly sidesteps whatever is wrong with Podman's
// health-status reporting.
export async function startStack() {
	await startEmulator();

	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'up', '-d', '--build'], {
		stdio: 'inherit',
		timeout: READY_TIMEOUT_MS,
		env: {
			...process.env,
			E2E_API_PORT: String(E2E_API_PORT),
			E2E_EMULATOR_PORT: String(E2E_EMULATOR_PORT)
		}
	});
	await waitForPort(DB_HOST, DB_PORT, READY_TIMEOUT_MS);
	await waitForPort(E2E_API_HOST, E2E_API_PORT, READY_TIMEOUT_MS);
}

export function stopStack() {
	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'down', '-v'], {
		stdio: 'inherit',
		timeout: 60_000
	});
	killPidfile();
}

async function startEmulator() {
	// A crashed previous run can leave the emulator holding its port.
	// Clear it out before starting a fresh one instead of failing to bind.
	killPidfile();

	const child = spawn(
		'bunx',
		['firebase-tools', 'emulators:start', '--only', 'auth', '--project', 'doula-cloud'],
		{ stdio: 'ignore', detached: true }
	);
	child.unref();
	if (child.pid) {
		writeFileSync(EMULATOR_PIDFILE, String(child.pid));
	}

	await waitForPort(E2E_EMULATOR_HOST, E2E_EMULATOR_PORT, READY_TIMEOUT_MS);
}

// Kills whatever process the pidfile points at (if any) and removes it.
// Used both to clear a stale pidfile before starting a fresh emulator and
// to stop the emulator this module started.
function killPidfile() {
	if (!existsSync(EMULATOR_PIDFILE)) return;
	const pid = Number(readFileSync(EMULATOR_PIDFILE, 'utf8'));
	try {
		process.kill(pid);
	} catch {
		// Already gone -- nothing to clean up.
	}
	rmSync(EMULATOR_PIDFILE, { force: true });
}

function waitForPort(host: string, port: number, timeoutMs: number): Promise<void> {
	const deadline = Date.now() + timeoutMs;
	return new Promise((resolve, reject) => {
		const attempt = () => {
			const socket = createConnection({ host, port }, () => {
				socket.end();
				resolve();
			});
			socket.on('error', () => {
				socket.destroy();
				if (Date.now() > deadline) {
					reject(new Error(`${host}:${port} not reachable after ${timeoutMs}ms`));
					return;
				}
				setTimeout(attempt, 1000);
			});
		};
		attempt();
	});
}
