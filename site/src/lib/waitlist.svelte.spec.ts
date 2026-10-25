import { afterEach, describe, expect, it } from 'vitest';
import { UTM_PARAMETERS, fillUtmFields, utmScript } from './waitlist.js';

function hiddenInputs(): HTMLInputElement[] {
	return UTM_PARAMETERS.map((name) => {
		const input = document.createElement('input');
		input.type = 'hidden';
		input.name = `metadata__${name}`;
		// As the card renders them: off until the script fills one (#1771).
		input.disabled = true;
		document.body.append(input);
		return input;
	});
}

afterEach(() => {
	document.body.replaceChildren();
});

describe('fillUtmFields', () => {
	it('copies each campaign parameter into its hidden input', () => {
		const inputs = hiddenInputs();
		fillUtmFields('?utm_source=a&utm_medium=b&utm_campaign=c&utm_content=d&other=e');
		expect(inputs.map((input) => input.value)).toEqual(['a', 'b', 'c', 'd']);
	});

	it('leaves an input empty when its parameter is absent', () => {
		const inputs = hiddenInputs();
		fillUtmFields('?utm_source=fb-rochester-birth-workers');
		expect(inputs.map((input) => input.value)).toEqual(['fb-rochester-birth-workers', '', '', '']);
	});

	it('turns on each input it fills, so that the form posts it (#1771)', () => {
		const inputs = hiddenInputs();
		fillUtmFields('?utm_source=dona-summit-2026&utm_content=booth');
		expect(inputs.map((input) => input.disabled)).toEqual([false, true, true, false]);
	});

	it('leaves every input off when the address carries no campaign value (#1771)', () => {
		const inputs = hiddenInputs();
		fillUtmFields('?other=e&utm_medium=');
		expect(inputs.map((input) => input.disabled)).toEqual([true, true, true, true]);
	});

	it('does nothing on a page with no form', () => {
		expect(() => fillUtmFields('?utm_source=a')).not.toThrow();
	});
});

function waitlistForm(firstName: string): HTMLFormElement {
	const form = document.createElement('form');
	const name = document.createElement('input');
	name.name = 'metadata__first_name';
	name.value = firstName;
	const email = document.createElement('input');
	email.name = 'email';
	email.value = 'ada@example.com';
	form.append(name, email);
	document.body.append(form);
	return form;
}

describe('fillUtmFields, on the first name (#1898)', () => {
	it('keeps a filled first name in the submission', () => {
		const form = waitlistForm('Ada');
		fillUtmFields('');
		expect([...new FormData(form)]).toEqual([
			['metadata__first_name', 'Ada'],
			['email', 'ada@example.com']
		]);
	});

	it('leaves a blank first name out of the submission, and always keeps the address', () => {
		const form = waitlistForm(' ');
		fillUtmFields('');
		expect([...new FormData(form)]).toEqual([['email', 'ada@example.com']]);
	});

	it('does nothing to a first-name input outside a form', () => {
		const name = document.createElement('input');
		name.name = 'metadata__first_name';
		document.body.append(name);
		expect(() => fillUtmFields('')).not.toThrow();
	});
});

describe('utmScript', () => {
	it('is the function itself, called on the address bar, so it runs with nothing else loaded', () => {
		const inputs = hiddenInputs();
		// The script runs against location.search; evaluate its function
		// part against a search string of the spec's own.
		const call = utmScript.replace('(location.search);', '');
		new Function(`${call}('?utm_campaign=booth')`)();
		expect(inputs[2].value).toBe('booth');
		expect(utmScript.endsWith('(location.search);')).toBe(true);
	});
});
