import { expect, vi } from 'vitest';

/**
Asserts a field-side error message by the id `LabeledField` derives from
the field's own id (`${id}-error`).

A field-targeted refusal renders twice by GOV.UK's own design (#467):
once as a link in the error summary, once beside the control itself --
`LabeledField`'s own `<p role="alert">`. Both carry the identical words,
so `getByText` cannot tell the two apart; this reads the one beside the
control instead, by the id `LabeledField` derives internally.

Lives beside `LabeledField` rather than in a generic spec-utility module
(#1223): `${id}-error` is that component's own derived convention, and a
generic module would let a spec reach for the string without the
component ever being party to it. It is a sibling file rather than an
export from `LabeledField.svelte`'s own `<script module>` block so that
importing `vitest` here never touches the production bundle -- nothing
outside a spec imports this file.
*/
export async function fieldError(id: string, message: string): Promise<void> {
	await vi.waitFor(() => {
		expect(document.querySelector(`#${id}-error`)?.textContent).toBe(message);
	});
}
