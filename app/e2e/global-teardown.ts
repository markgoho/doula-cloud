import { execFileSync } from 'node:child_process';

export default function globalTeardown() {
	execFileSync('podman', ['compose', '-f', 'compose.e2e.yaml', 'down', '-v'], {
		stdio: 'inherit'
	});
}
