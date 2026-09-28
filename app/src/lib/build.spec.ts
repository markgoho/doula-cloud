import { describe, expect, it } from 'vitest';
import { appBuild } from './build.js';

describe('appBuild', () => {
	it("is 'dev' outside a CI build, where __APP_BUILD__ carries no commit", () => {
		expect(appBuild()).toBe('dev');
	});
});
