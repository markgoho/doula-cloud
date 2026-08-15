import { execFileSync } from 'node:child_process';

// Brings up the self-contained podman-compose stack (currently just
// Postgres) that e2e tests run against, before Playwright starts the app.
export default function globalSetup() {
	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'up', '-d', '--wait'], {
		stdio: 'inherit'
	});
}
