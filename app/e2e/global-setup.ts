import { execFileSync } from 'node:child_process';
import { createConnection } from 'node:net';

const DB_HOST = '127.0.0.1';
const DB_PORT = 15432;
const READY_TIMEOUT_MS = 60_000;

// Brings up the self-contained podman-compose stack (currently just
// Postgres) that e2e tests run against, before Playwright starts the app.
//
// Deliberately doesn't use `up --wait`: under this runner's rootless
// Podman + docker-compose-as-provider combination, the container starts
// fine but its healthcheck never reports "healthy", so --wait blocks
// forever (confirmed on trunk — see git log for this file). Polling the
// host-exposed port directly sidesteps whatever is wrong with Podman's
// health-status reporting.
export default async function globalSetup() {
	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'up', '-d'], {
		stdio: 'inherit',
		timeout: READY_TIMEOUT_MS
	});
	await waitForPort(DB_HOST, DB_PORT, READY_TIMEOUT_MS);
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
					reject(new Error(`db not reachable on ${host}:${port} after ${timeoutMs}ms`));
					return;
				}
				setTimeout(attempt, 1000);
			});
		};
		attempt();
	});
}
