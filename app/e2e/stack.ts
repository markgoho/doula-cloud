import { execFileSync, spawn } from 'node:child_process';
import { createConnection } from 'node:net';
import { existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { E2E_API_HOST, E2E_API_PORT, E2E_EMULATOR_HOST, E2E_EMULATOR_PORT } from './ports';

const DB_HOST = '127.0.0.1';
const DB_PORT = 15432;
const READY_TIMEOUT_MS = 60_000;
// `up --build` builds two separate Go services (migrate, api), each
// running its own `go mod download` from a cold module cache with no
// sharing between them, plus pulling base images -- observed to take up
// to ~57s on a cold CI runner even when nothing is wrong, so it needs a
// longer budget than the port-readiness waits below.
const BUILD_TIMEOUT_MS = 300_000;

// `docker compose` and `podman compose` share the same v2 CLI syntax, so
// this doubles as both the binary to exec and the flag that picks
// compose.e2e.yaml's host-gateway hostname (see HOST_GATEWAY below).
// Local dev defaults to Podman (see docs/testing.md); CI sets
// CONTAINER_ENGINE=docker, since GH-hosted runners ship Docker natively
// with no rootless-socket setup and no docker-compose-as-external
// -provider translation layer -- the source of a real, hard-to-debug
// Podman-only bug (container-to-host networking via
// host.containers.internal silently breaking under CI's Podman setup).
const CONTAINER_ENGINE = process.env.CONTAINER_ENGINE ?? 'podman';
// Docker Engine (unlike Docker Desktop) doesn't predefine
// host.docker.internal -- compose.e2e.yaml adds it via `extra_hosts:
// host-gateway` for when this is 'docker'. Podman predefines
// host.containers.internal itself, no extra_hosts entry needed.
const HOST_GATEWAY = CONTAINER_ENGINE === 'docker' ? 'host.docker.internal' : 'host.containers.internal';

// The Firebase Auth emulator (see firebase.json) is plain Node with no
// Postgres dependency, so it runs as a host process rather than another
// compose service. Shared by the Playwright e2e run
// (global-setup.ts/global-teardown.ts) and `bun run dev:full`
// (scripts/dev-full.ts) for local interactive use.
const EMULATOR_PIDFILE = join(tmpdir(), 'doula-cloud-e2e-firebase-emulator.pid');

// Brings up the self-contained compose stack -- Postgres, the goose
// migration step, the app_e2e login role, and the Go BFF itself (see
// compose.e2e.yaml) -- plus the Firebase Auth emulator.
//
// Deliberately doesn't use `up --wait`: under CI's rootless Podman (when
// CONTAINER_ENGINE=podman) with docker-compose-as-provider, the container
// starts fine but its healthcheck never reports "healthy", so --wait
// blocks forever (confirmed on trunk -- see git log for this file).
// Polling the host-exposed ports directly sidesteps that.
export async function startStack() {
	await startEmulator();

	execFileSync(CONTAINER_ENGINE, ['compose', '-f', 'compose.e2e.yaml', 'up', '-d', '--build'], {
		stdio: 'inherit',
		timeout: BUILD_TIMEOUT_MS,
		env: {
			...process.env,
			E2E_API_PORT: String(E2E_API_PORT),
			E2E_EMULATOR_PORT: String(E2E_EMULATOR_PORT),
			E2E_HOST_GATEWAY: HOST_GATEWAY
		}
	});
	await waitForPort(DB_HOST, DB_PORT, READY_TIMEOUT_MS);
	await waitForPort(E2E_API_HOST, E2E_API_PORT, READY_TIMEOUT_MS);
}

// Links identityUID to clientID via client_portal_users, directly against
// the compose stack's `db` service. Nothing in the BFF creates this row
// (see #53: v1 has no Client-portal-provisioning endpoint, staff-side or
// otherwise -- Staff creates a Client + Engagement per #52, but linking a
// Client to portal login credentials isn't spec'd anywhere yet), so the
// e2e test that proves login->landing works has to seed it itself. Uses
// `compose exec` + psql the same way compose.e2e.yaml's own seed-role
// service provisions the app_e2e login role, rather than adding a new
// Postgres client dependency to app/ just for this one insert.
//
// Values are inlined into the -c string (escaped, not passed via psql's
// own -v/:'var' substitution): podman-compose's `exec` passthrough
// doesn't forward -v flags to the exec'd process intact (confirmed
// empirically -- the SQL psql received still had the literal `:'uid'`
// text in it, unsubstituted), so :'var' interpolation never fires.
// identityUID and clientID both come from this test's own prior
// API/emulator calls, not external input.
function sqlLiteral(value: string): string {
	return `'${value.replace(/'/g, "''")}'`;
}

export function seedClientPortalUser(identityUID: string, clientID: string) {
	execFileSync(
		CONTAINER_ENGINE,
		[
			'compose',
			'-f',
			'compose.e2e.yaml',
			'exec',
			'-T',
			'db',
			'psql',
			'-U',
			'app',
			'-d',
			'app',
			'-v',
			'ON_ERROR_STOP=1',
			'-c',
			`INSERT INTO client_portal_users (identity_uid, client_id) VALUES (${sqlLiteral(identityUID)}, ${sqlLiteral(clientID)})`
		],
		{ stdio: 'inherit', timeout: 30_000 }
	);
}

export function stopStack() {
	execFileSync(CONTAINER_ENGINE, ['compose', '-f', 'compose.e2e.yaml', 'down', '-v'], {
		stdio: 'inherit',
		timeout: 60_000
	});
	killPidfile();
}

async function startEmulator() {
	// A crashed previous run can leave the emulator holding its port.
	// Clear it out before starting a fresh one instead of failing to bind.
	killPidfile();

	// firebase.json sets emulators.auth.host to 0.0.0.0 -- the Firebase
	// CLI defaults to binding auth's port on 127.0.0.1 only, which the
	// Playwright test process (running on the host) can reach fine, but
	// the `api` container cannot: on plain Linux (unlike this repo's
	// Podman-on-macOS-VM local dev setup, where host.containers.internal
	// happens to route to loopback-bound services too), a container's
	// route to the host's gateway IP never reaches a service bound only
	// to the host's own loopback interface. Confirmed as the actual cause
	// of a "401 invalid token" e2e failure that persisted across both
	// Podman and Docker in CI -- switching container engines was a red
	// herring for this specific bug, even though it fixed a separate,
	// real Podman-in-CI issue (see git log for this file/compose.e2e.yaml).

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
