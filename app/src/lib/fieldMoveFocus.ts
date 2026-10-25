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
