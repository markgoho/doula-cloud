/**
 * Brings the row a link named into view and focuses it (#2240, #2747).
 * `rowDomIds` is the row's id in each view its table draws (a table row and
 * a record card); the one actually displayed is the one with a box.
 * `scroll-margin-block-start` on DataTable's rows keeps the row clear of the
 * sticky top bar and header.
 */
export function landOnRow(rowDomIds: readonly string[]): void {
	const shownRow = rowDomIds
		.map((id) => document.querySelector<HTMLElement>(`#${CSS.escape(id)}`))
		.find((row) => row !== null && row.getClientRects().length > 0);
	shownRow?.scrollIntoView({ block: 'start' });
	shownRow?.focus({ preventScroll: true });
}
