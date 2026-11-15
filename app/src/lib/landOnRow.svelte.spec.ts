import { afterEach, describe, expect, it } from 'vitest';
import { landOnRow, landOnSection } from './landOnRow.js';

function setup() {
	document.body.innerHTML = '<input id="elsewhere"><div id="row" tabindex="-1">Row</div>';
	const row = document.querySelector<HTMLElement>('#row')!;
	const elsewhere = document.querySelector<HTMLInputElement>('#elsewhere')!;
	return { row, elsewhere };
}

afterEach(() => {
	document.body.replaceChildren();
});

function setupSection() {
	document.body.innerHTML =
		'<input id="elsewhere"><section id="contract"><h2 id="contract-heading">Contract</h2></section>';
	return {
		heading: document.querySelector<HTMLElement>('#contract-heading')!,
		elsewhere: document.querySelector<HTMLInputElement>('#elsewhere')!
	};
}

describe('landOnSection', () => {
	it('focuses the section heading when focus is still on the page', () => {
		const { heading } = setupSection();

		landOnSection('contract');

		expect(document.activeElement).toBe(heading);
	});

	it('leaves focus where the reader moved it', () => {
		const { elsewhere } = setupSection();
		elsewhere.focus();

		landOnSection('contract');

		expect(document.activeElement).toBe(elsewhere);
	});

	it('does nothing when the page has no such section', () => {
		setup();

		landOnSection('contract');

		expect(document.activeElement).toBe(document.body);
	});
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
