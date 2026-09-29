import { expect } from 'vitest';
import { page } from 'vitest/browser';

/**
Asserts the screen has exactly one `<h1>`, named `name`, and that it sits
inside a Template's page frame -- the `center-l` every Template in this
directory wraps its content in, which is what supplies the page gutters.

#1576 found five routes whose `<h1>` rendered straight into a layout with
no gutter; a heading outside `center-l` is that defect. The frame is a
layout primitive with no role, so `closest('center-l')` is the only way to
ask -- svelte-tests.md's third named exception, a fact about the document's
structure rather than about any one accessible element.

Lives beside the Templates because `center-l` as the frame is their own
convention; a sibling file rather than a `<script module>` export so that
importing `vitest` never touches the production bundle.
*/
export async function expectOneFramedHeading(name: string): Promise<void> {
	const heading = page.getByRole('heading', { level: 1, name });
	await expect.element(heading).toBeVisible();
	expect(page.getByRole('heading', { level: 1 }).elements()).toHaveLength(1);
	expect(heading.element().closest('center-l')).not.toBeNull();
}
