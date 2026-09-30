import { afterEach, describe, expect, it } from 'vitest';
import { UTM_PARAMETERS, fillUtmFields, utmScript } from './waitlist.js';

function hiddenInputs(): HTMLInputElement[] {
	return UTM_PARAMETERS.map((name) => {
		const input = document.createElement('input');
		input.type = 'hidden';
		input.name = `metadata__${name}`;
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

	it('does nothing on a page with no form', () => {
		expect(() => fillUtmFields('?utm_source=a')).not.toThrow();
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
