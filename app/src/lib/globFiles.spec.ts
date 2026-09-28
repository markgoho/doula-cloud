import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { describe, expect, it, onTestFinished } from 'vitest';
import { globFiles } from './globFiles';

// A tree under the OS temp directory, never app/src: two real files beside
// the directory a failed browser test leaves next to its spec.
function setup(): string {
	const root = mkdtempSync(path.join(tmpdir(), 'globFiles-'));
	onTestFinished(() => rmSync(root, { recursive: true, force: true }));
	mkdirSync(path.join(root, 'src/routes/__screenshots__/x.svelte.spec.ts'), { recursive: true });
	writeFileSync(path.join(root, 'src/routes/page.ts'), '');
	writeFileSync(path.join(root, 'src/routes/page.svelte'), '');
	return root;
}

function sorted(files: string[]): string[] {
	return files.toSorted((a, b) => a.localeCompare(b));
}

describe('globFiles', () => {
	it('returns the files a pattern matches, relative to cwd', () => {
		const cwd = setup();
		expect(sorted(globFiles('src/**/*.{svelte,ts}', { cwd }))).toEqual([
			'src/routes/page.svelte',
			'src/routes/page.ts'
		]);
	});

	it('drops a directory whose name matches the pattern, as a failed browser test leaves behind (#1552)', () => {
		const cwd = setup();
		expect(globFiles('src/**/*.ts', { cwd })).toEqual(['src/routes/page.ts']);
	});

	it('takes several patterns at once, as globSync does', () => {
		const cwd = setup();
		expect(sorted(globFiles(['src/**/*.ts', 'src/**/*.svelte'], { cwd }))).toEqual([
			'src/routes/page.svelte',
			'src/routes/page.ts'
		]);
	});
});
