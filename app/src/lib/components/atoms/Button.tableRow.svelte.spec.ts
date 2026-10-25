import { page } from 'vitest/browser';
import { afterEach, describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import Button from './Button.svelte';
// The row's height and the type come from the tokens, so the whole
// stylesheet is loaded -- the reason this is a file of its own rather
// than a block in Button.svelte.spec.ts, whose cases run unstyled.
import '#lib/styles/app.css';

// `--row-target-min`: the 40px row less its 1px rule, while no input can
// tap (tokens.css).
const ROW_TARGET = 39;

/*
 * Past the ramp's 1920px end, so every fluid step sits at its ceiling:
 * the widest the type and the spacing get, and where #1945 found the
 * button taller than its row.
 */
const CEILING = [2100, 900] as const;

// `render` cleans up the Button it mounted, not the table it was mounted
// into, so each row this file builds is taken down here.
afterEach(() => {
	for (const table of document.body.querySelectorAll(':scope > table')) table.remove();
});

async function setupInRow(size: 'sm' | 'md') {
	await page.viewport(...CEILING);
	const table = document.createElement('table');
	const cell = table.insertRow().insertCell();
	document.body.append(table);
	await render(Button, { target: cell, props: { label: 'Remove', size, onClick: () => {} } });
	return page.getByRole('button', { name: 'Remove' }).element();
}

describe('a Button in a table row (#1843, #1945)', () => {
	it.each(['sm', 'md'] as const)(
		'keeps a %s button on one line at the row target at the ramp ceiling',
		async (size) => {
			const button = await setupInRow(size);

			expect(button.getBoundingClientRect().height).toBe(ROW_TARGET);
		}
	);
});
