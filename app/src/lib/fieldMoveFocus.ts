/**
 * What a screen reader is told after *Move up* or *Move down* in a field
 * editor, #1955: the field's name and its new place in the list, "Due
 * date moved to 2 of 3". The editor puts it in a polite `role="status"`
 * region that is in the page from the first render, so the first move is
 * read too. `fields` is the list being reordered, after the move, and
 * `name` is the editor's own name for a row (its `rowName`, which falls
 * back to "Untitled field"). The id is always in `fields`: it is the id of
 * the row whose button was pressed.
 *
 * A press at an end of the list moves nothing and calls nothing (the
 * button is `ariaDisabled`), so the region keeps the last move's text and
 * announces nothing new: that move already said "3 of 3", and the button
 * now reads as unavailable.
 */
export function moveAnnouncement<F extends { id: string }>(
	fields: readonly F[],
	id: string,
	name: (field: F) => string
): string {
	const index = fields.findIndex((field) => field.id === id);
	return `${name(fields[index])} moved to ${index + 1} of ${fields.length}`;
}

/**
 * Where focus goes after *Move up* or *Move down* in a field editor
 * (`DynamicFieldEditor`, `ClientFieldTemplateEditor`), #1920: back on the
 * button that was pressed, in the row that moved.
 *
 * A keyed `{#each}` moves the row's DOM nodes, and a focused element that
 * is moved loses focus to `<body>`, so the editor puts it back once the
 * list has reordered. The same button, and not the row's other move
 * button, so that a second press moves the row one more place and never
 * undoes the move. At an end of the list that button is `ariaDisabled`,
 * not `disabled`, so it keeps focus there too, a second press does
 * nothing, and a screen reader reads it as unavailable.
 *
 * Each move button already carries a unique `aria-describedby`
 * (`<id>-move-up-name`, #515), so that is how the row's button is found.
 */
export function focusAfterMove(list: HTMLElement, id: string, direction: 'up' | 'down'): void {
	list
		.querySelector<HTMLButtonElement>(`button[aria-describedby="${CSS.escape(`${id}-move-${direction}-name`)}"]`)
		?.focus();
}
