import { execFileSync } from 'node:child_process';

// Brings up the self-contained podman-compose stack (currently just
// Postgres) that e2e tests run against, before Playwright starts the app.
// --wait-timeout bounds the wait: without it, `up --wait` blocks forever if
// a healthcheck never reports healthy, and execFileSync's own timeout is a
// backstop in case the compose provider itself ignores that flag.
export default function globalSetup() {
	execFileSync(
		'podman',
		['compose', '-f', 'compose.e2e.yaml', 'up', '-d', '--wait', '--wait-timeout', '60'],
		{ stdio: 'inherit', timeout: 90_000 }
	);
}
