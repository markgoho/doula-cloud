// `bun run seed:scenario <name> [flags]` -- builds one walk's named
// starting state (scenarios.ts) against a stack `bun run dev:full` already
// started, at this worktree's port offset, then exits (#2686). It prints
// the sign-in URL, the URL of the screen the walk starts on, and the path
// of a file holding the accounts, the password and the Owner's session.
// No secret is printed. With no name, or one it does not know, it lists
// the scenarios and exits 1.
import { request } from '@playwright/test';
import { PORT_OFFSET_ROOT } from '../e2e/ports';
import { MAILBOX_URL } from '../e2e/stack';
import { SCENARIOS, type Scenario } from './scenarios';

const [name, ...flags] = process.argv.slice(2);
const scenario: Scenario | undefined = name === undefined ? undefined : SCENARIOS[name];
const knownFlags = Object.keys(scenario?.flags ?? {});
const unknownFlags = flags.filter((flag) => !knownFlags.includes(flag));

if (scenario === undefined) {
	const lines = Object.entries(SCENARIOS).flatMap(([key, each]) => [
		`  ${key}: ${each.builds}`,
		...Object.entries(each.flags ?? {}).map(([flag, what]) => `      ${flag}: ${what}`)
	]);
	console.error(
		[
			name === undefined ? 'seed:scenario: name a scenario.' : `seed:scenario: no scenario is named "${name}".`,
			'Usage: bun run --cwd app seed:scenario <name> [flags]',
			'Scenarios:',
			...lines
		].join('\n')
	);
	process.exitCode = 1;
} else if (unknownFlags.length > 0) {
	console.error(`seed:scenario: ${name} takes no ${unknownFlags.join(', ')}. Its flags: ${knownFlags.join(', ') || 'none'}`);
	process.exitCode = 1;
} else {
	const context = await request.newContext();
	try {
		const result = await scenario.run(context, new Set(flags));
		const file = `${PORT_OFFSET_ROOT ?? `${Bun.env.TMPDIR ?? '/tmp'}/doula-cloud`}/.seed-scenario/${name}.json`;
		await Bun.write(file, `${JSON.stringify({ scenario: name, flags, ...result, mailboxUrl: `${MAILBOX_URL}/` }, undefined, 2)}\n`);
		await Bun.$`chmod 600 ${file}`;
		console.log(
			[`${name}: ready`, `  Sign in:     ${result.signInUrl}`, `  Start at:    ${result.startUrl}`, `  Credentials: ${file}`].join('\n')
		);
	} finally {
		await context.dispose();
	}
}
