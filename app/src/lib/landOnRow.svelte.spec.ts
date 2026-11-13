import { afterEach, describe, expect, it } from 'vitest';
import { landOnRow } from './landOnRow.js';

function setup() {
	document.body.innerHTML = '<input id="elsewhere"><div id="row" tabindex="-1">Row</div>';
	const row = document.querySelector<HTMLElement>('#row')!;
	const elsewhere = document.querySelector<HTMLInputElement>('#elsewhere')!;
	return { row, elsewhere };
}

afterEach(() => {
	document.body.replaceChildren();
});

describe('landOnRow', () => {
	it('scrolls to the row and focuses it when focus is still on the page', () => {
		const { row } = setup();

		landOnRow(['row']);

		expect(document.activeElement).toBe(row);
	});

	it('does nothing when no row is shown', () => {
		setup();

		landOnRow(['missing']);

		expect(document.activeElement).toBe(document.body);
	});

	it('leaves focus where the reader moved it', () => {
		const { elsewhere } = setup();
		elsewhere.focus();

		landOnRow(['row']);

		expect(document.activeElement).toBe(elsewhere);
	});

	it('lands again when focus is already on the row', () => {
		const { row } = setup();
		row.focus();

		landOnRow(['row']);

		expect(document.activeElement).toBe(row);
	});
});
