const CONTROLS =
	'a, button, input, select, textarea, summary, label, option, details, dialog, [role=button], [role=link], [role=menuitem], [role=checkbox], [contenteditable], td.actions, .record-actions';
const MOVE_LIMIT = 5;

/*
 * Makes a row or a record card a click target as a whole (#2888): a plain
 * primary click anywhere on it calls `click()` on its one `[data-row-link]`,
 * so the keyboard and a screen reader still meet one link. `DataTable` and
 * the Contracts void-request cards attach it.
 */
export function rowTarget(row: HTMLElement) {
	let press: { x: number; y: number; wasSelecting: boolean } | undefined;

	const isSelectingInRow = () => {
		const selection = getSelection();
		return Boolean(
			selection && !selection.isCollapsed && selection.rangeCount > 0 && selection.getRangeAt(0).intersectsNode(row)
		);
	};

	const isOnControl = (event: Event) => {
		const control = (event.target as Element).closest(CONTROLS);
		return Boolean(control && row.contains(control));
	};

	const link = () => row.querySelector<HTMLAnchorElement>('[data-row-link]')!;

	const onpointerdown = (event: PointerEvent) => {
		press = { x: event.clientX, y: event.clientY, wasSelecting: isSelectingInRow() };
	};

	const onclick = (event: MouseEvent) => {
		const started = press;
		press = undefined;
		if (event.defaultPrevented || isOnControl(event) || event.button !== 0) return;
		if (started && (started.wasSelecting || Math.hypot(event.clientX - started.x, event.clientY - started.y) > MOVE_LIMIT)) {
			return;
		}
		const target = link();
		if (event.ctrlKey || event.metaKey || event.shiftKey) {
			window.open(target.href, '_blank', 'noopener');
			return;
		}
		if (event.altKey || event.detail !== 1) return;
		target.click();
	};

	const onauxclick = (event: MouseEvent) => {
		if (event.button !== 1 || event.defaultPrevented || isOnControl(event)) return;
		window.open(link().href, '_blank', 'noopener');
	};

	row.addEventListener('pointerdown', onpointerdown);
	row.addEventListener('click', onclick);
	row.addEventListener('auxclick', onauxclick);
	return () => {
		row.removeEventListener('pointerdown', onpointerdown);
		row.removeEventListener('click', onclick);
		row.removeEventListener('auxclick', onauxclick);
	};
}
