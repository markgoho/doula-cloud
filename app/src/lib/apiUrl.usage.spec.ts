import { globSync, readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/*
 * #1254: 17 files under app/e2e each declared their own
 * `const API_URL = \`http://${E2E_API_HOST}:${E2E_API_PORT}\`;`, all
 * deriving the same value the same way. ports.ts now exports `API_URL`
 * as the one owner, beside `E2E_API_HOST`/`E2E_API_PORT`, the pieces it's
 * built from -- so a change to the scheme, host or port has one site to
 * find rather than 17.
 *
 * This is the seam that keeps a new spec from reintroducing a local
 * copy silently: the same grep-the-tree shape as spelling.usage.spec.ts
 * and tokens.usage.spec.ts, applied to app/e2e rather than app/src.
 */

const appRoot = fileURLToPath(new URL('../../', import.meta.url));
const repoRoot = path.join(appRoot, '..');

const OWNER = 'app/e2e/ports.ts';

// Paths come back prefixed "app/e2e/", the same shape
// spelling.usage.spec.ts's own e2eFiles glob uses, since this runs from
// repoRoot rather than appRoot -- e2e/ sits outside src/ entirely.
const e2eFiles = globSync('app/e2e/**/*.{ts,js}', { cwd: repoRoot }).filter((file) => file !== OWNER);

// Catches a reintroduced `const API_URL = ...` under that exact name.
const DECLARATION = /\bconst\s+API_URL\s*=/;

// Catches the underlying duplication even under a different name --
// mailbox.ts's old `BOUNCE_TARGET` and simulation/clock.ts's inline drain
// URL both built this same string without ever calling it `API_URL`, so
// the name-only check above would have missed both. The issue's own
// framing ("every one of them derives the same value the same way") is
// about the derivation, not the identifier.
const DERIVATION = /\$\{E2E_API_HOST\}:\$\{E2E_API_PORT\}/;

describe('app/e2e derives API_URL in one place', () => {
	it('reads the whole app/e2e tree', () => {
		// A glob that silently matched nothing would make the assertions
		// below pass while checking no source at all.
		expect(e2eFiles.length).toBeGreaterThan(10);
	});

	it('exports API_URL from ports.ts', () => {
		const source = readFileSync(path.join(repoRoot, OWNER), 'utf8');
		expect(source).toMatch(/export const API_URL\s*=/);
	});

	it('redeclares API_URL nowhere else under app/e2e', () => {
		const offenders = e2eFiles.filter((file) =>
			DECLARATION.test(readFileSync(path.join(repoRoot, file), 'utf8'))
		);

		expect(offenders).toEqual([]);
	});

	it('rebuilds the host:port pair nowhere else under app/e2e, under any name', () => {
		const offenders = e2eFiles.filter((file) =>
			DERIVATION.test(readFileSync(path.join(repoRoot, file), 'utf8'))
		);

		expect(offenders).toEqual([]);
	});
});
