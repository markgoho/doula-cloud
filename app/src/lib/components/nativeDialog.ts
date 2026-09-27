/*
 * The open-state effect `molecules/Dialog.svelte` (#473) and
 * `organisms/Drawer.svelte` (#1521) share: syncing a bindable `open`
 * boolean onto a native `<dialog>`'s own imperative API. `openDialog` is
 * the one thing the two callers disagree about -- Dialog always calls
 * `showModal()`; Drawer picks `showModal()` or `show()` depending on how
 * much room it has -- so it is the parameter, and everything else here is
 * identical between them.
 */
export function syncDialogOpen(
	dialog: HTMLDialogElement | undefined,
	isOpen: boolean,
	openDialog: (dialog: HTMLDialogElement) => void
): void {
	/* v8 ignore next -- dialog is bound via bind:this before this function
	   is ever called from an effect, so this guard exists only to satisfy
	   HTMLDialogElement | undefined, not a reachable branch. */
	if (!dialog) return;
	/*
	 * The parent's checkVisibility(), not the dialog's own: a closed
	 * <dialog> is display:none by the UA stylesheet regardless of its
	 * ancestors (dialog:not([open])), so checking the dialog itself
	 * always reads false right here, before showModal()/show() has run.
	 *
	 * This guards the open call because DataTable renders its rowActions
	 * content once per tree, one hidden via display:none (#508, ADR-0024),
	 * so a caller whose open state is a shared boolean bound outside the
	 * row -- confirmEndSessionsFor === member.staffId, the same shape
	 * every ConfirmDialog call site here uses -- flips both copies' `open`
	 * true together. A display:none ancestor does not stop showModal()
	 * from succeeding (verified directly: dialog.open and :modal both come
	 * back true), so without this the hidden copy's own ::backdrop -- a
	 * top-layer sibling, unaffected by its own ancestor's display:none --
	 * can end up stacked over the real one and swallow every click meant
	 * for it.
	 */
	if (isOpen && !dialog.open) {
		if (dialog.parentElement?.checkVisibility()) openDialog(dialog);
	} else if (!isOpen && dialog.open) {
		dialog.close();
	}
}
