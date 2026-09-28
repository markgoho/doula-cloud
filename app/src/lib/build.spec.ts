import { describe, expect, it } from 'vitest';
import { appBuild } from './build.js';

describe('appBuild', () => {
	/*
	 * `__APP_BUILD__` is baked in by `vite.config.ts`'s `define` before
	 * Vitest ever runs, from the same `GITHUB_SHA ?? 'dev'` this test
	 * recomputes -- not a hardcoded `'dev'`, which passes locally (no
	 * `GITHUB_SHA`) but fails in CI, where the variable is genuinely set.
	 */
	it("carries GITHUB_SHA where the build set one, or 'dev' otherwise", () => {
		expect(appBuild()).toBe(process.env.GITHUB_SHA ?? 'dev');
	});
});
