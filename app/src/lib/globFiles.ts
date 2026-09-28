import { globSync, statSync } from 'node:fs';
import path from 'node:path';

/*
 * The file list every `*.usage.spec.ts` sweeps (#1552). `globSync` matches
 * directories as well as files, and a failed browser test leaves one behind
 * that ends in `.ts` -- `__screenshots__/<spec file name>/` -- which a
 * `*.ts` pattern then hands to `readFileSync`, and the spec dies with
 * `EISDIR` before any `it` runs. A path is kept by what it is, a regular
 * file, never by its name: the next tool to write a `*.ts/` directory will
 * not call it `__screenshots__`.
 *
 * Same inputs and the same cwd-relative paths as `globSync`, so each spec's
 * own `.filter(...)` chain reads the result unchanged.
 */
export function globFiles(pattern: string | string[], { cwd }: { cwd: string }): string[] {
	return globSync(pattern, { cwd }).filter((file) => statSync(path.join(cwd, file)).isFile());
}
